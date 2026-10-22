package sim

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	mrand "math/rand/v2"
	"strings"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/suite/check"
	"github.com/skkil/store/modules/conformance/suite/credentials"
	"github.com/skkil/store/modules/conformance/suite/hashchain"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

// Limits of one simulated client.
const (
	MaxPushBatch = 100
	maxPages     = 10_000
)

// RequestTimeout bounds one Push or Pull.
const RequestTimeout = 30 * time.Second

// Options configure a simulated client.
type Options struct {
	Name        string
	User        string
	Profile     *profile.Profile
	Transport   *Transport
	Credentials credentials.Source
	Ledger      *check.Ledger
	// ListenPokes keeps a Pokes stream open while online; otherwise the
	// client ignores pokes and relies on its own pulls.
	ListenPokes bool
	// PageSize is the limit of a full pull; 0 lets the server choose.
	PageSize uint32
	// KeepLog keeps every applied version, for an auditor.
	KeepLog bool
}

// Pending is a mutation the client made that the server has not yet acknowledged.
type Pending struct {
	ID   uint64
	Name string
	Args []byte
	Kind profile.Kind
	HLC  int64
}

// Client is a minimal store client: a pending queue, a cursor and a replica.
// It is not safe for concurrent use; the pokes it receives are.
type Client struct {
	opts         Options
	id           uuid.UUID
	nextMutation uint64
	pending      []Pending
	cursor       *storev1.Cursor
	replica      *Replica
	lastAck      uint64
	minVersion   uint64
	online       bool
	expectReset  bool
	resetEpoch   uint32
	resets       int
	pokes        *pokeStream
	highestPoke  atomic.Uint64
	log          []*storev1.Version
	clock        hybridClock
}

// New returns an online client with a fresh id, an empty replica and a fresh cursor.
func New(opts Options) *Client {
	c := &Client{opts: opts, id: uuid.New(), nextMutation: 1, cursor: &storev1.Cursor{}, replica: NewReplica()}
	opts.Ledger.Name(c.id.String(), opts.Name)
	return c
}

// ID returns the client id.
func (c *Client) ID() string { return c.id.String() }

// Name returns the client's short name.
func (c *Client) Name() string { return c.opts.Name }

// Online reports whether the client is connected.
func (c *Client) Online() bool { return c.online }

// Listening reports whether the client acts on pokes.
func (c *Client) Listening() bool { return c.opts.ListenPokes }

// PendingCount returns how many mutations wait for an acknowledgement.
func (c *Client) PendingCount() int { return len(c.pending) }

// Cursor returns how far the client has applied the log.
func (c *Client) Cursor() *storev1.Cursor { return c.cursor }

// State returns the client's synced rows.
func (c *Client) State() check.State { return c.replica.State() }

// HighestPoke returns the highest version a poke announced.
func (c *Client) HighestPoke() uint64 { return c.highestPoke.Load() }

// Log returns the versions applied since the last Reset when KeepLog is set.
func (c *Client) Log() []*storev1.Version { return c.log }

// Resets returns how many Resets the client handled.
func (c *Client) Resets() int { return c.resets }

// Mutate makes one mutation of kind (any kind when "") from the profile,
// chosen and filled from seed, and queues it. It describes what it made.
func (c *Client) Mutate(seed uint64, kind profile.Kind) string {
	rng := mrand.New(mrand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	usable := c.opts.Profile.Usable(c.replica.Count, kind)
	if len(usable) == 0 {
		return "no usable mutation"
	}
	m := pick(rng, usable)
	id := c.nextMutation
	args, err := m.Render(&renderEnv{rng: rng, client: c.id, mutationID: id, replica: c.replica})
	if err != nil {
		return "render failed: " + err.Error()
	}
	c.nextMutation++
	c.pending = append(c.pending, Pending{ID: id, Name: m.Name, Args: args, Kind: m.Kind, HLC: c.clock.next()})
	created := make([]check.CreatedRow, len(m.Creates))
	for n, table := range m.Creates {
		created[n] = check.CreatedRow{Table: table, ID: (&renderEnv{client: c.id, mutationID: id}).NewID(uint32(n))}
	}
	c.opts.Ledger.Expect(check.MutationKey{Client: c.ID(), MutationID: id}, check.Expectation{Name: m.Name, Kind: m.Kind, Created: created})
	return fmt.Sprintf("m%d %s (%s) %s", id, m.Name, m.Kind, args)
}

func pick(rng *mrand.Rand, usable []*profile.Mutation) *profile.Mutation {
	total := 0
	for _, m := range usable {
		total += m.Weight
	}
	n := rng.IntN(total)
	for _, m := range usable {
		if n < m.Weight {
			return m
		}
		n -= m.Weight
	}
	return usable[len(usable)-1]
}

// Push sends the pending queue. With lose set, the response is dropped as
// if it were lost on the way back, so the client will send the same
// mutations again. Retryable transport failures are part of the summary,
// not an error.
func (c *Client) Push(ctx context.Context, lose bool) (string, error) {
	if !c.online {
		return "offline", nil
	}
	if len(c.pending) == 0 {
		return "nothing pending", nil
	}
	batch := c.pending[:min(len(c.pending), MaxPushBatch)]
	creds, err := c.credentials(ctx)
	if err != nil {
		return "", err
	}
	req := connect.NewRequest(&storev1.PushRequest{
		ProtocolVersion: ProtocolVersion,
		ClientId:        c.ID(),
		SchemaVersion:   c.opts.Profile.SchemaVersion,
		Mutations:       make([]*storev1.Mutation, len(batch)),
	})
	sent := make([]uint64, len(batch))
	for i, p := range batch {
		req.Msg.Mutations[i] = &storev1.Mutation{MutationId: p.ID, Name: p.Name, Args: p.Args, Hlc: p.HLC}
		sent[i] = p.ID
	}
	for name, values := range creds.PushHeaders {
		req.Header()[name] = values
	}
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	resp, err := c.opts.Transport.Push.Push(ctx, req)
	if err != nil {
		if Retryable(err) {
			return "push failed, will retry: " + err.Error(), nil
		}
		return "", fmt.Errorf("%s push: %w", c.opts.Name, err)
	}
	results := resp.Msg.GetResults()
	c.opts.Ledger.Pushed(c.ID(), sent, results)
	summary := describeResults(results)
	if lose {
		return "response lost: " + summary, nil
	}
	for _, r := range results {
		c.minVersion = max(c.minVersion, r.GetVersion())
	}
	return summary, nil
}

func describeResults(results []*storev1.MutationResult) string {
	parts := make([]string, len(results))
	for i, r := range results {
		status := strings.TrimPrefix(r.GetStatus().String(), "STATUS_")
		parts[i] = fmt.Sprintf("m%d %s v%d", r.GetMutationId(), status, r.GetVersion())
		if r.GetReason() != "" {
			parts[i] += fmt.Sprintf(" %q", r.GetReason())
		}
	}
	return strings.Join(parts, ", ")
}

// Pull fetches one page of at most limit versions (0: the server's default).
func (c *Client) Pull(ctx context.Context, limit uint32) (string, error) {
	if !c.online {
		return "offline", nil
	}
	summary, _, err := c.pullPage(ctx, limit)
	return summary, err
}

// PullAll pulls pages until the client has the head.
func (c *Client) PullAll(ctx context.Context) (string, error) {
	if !c.online {
		return "offline", nil
	}
	var parts []string
	firstReset := c.resets
	for range maxPages {
		before, resets := c.cursor, c.resets
		summary, more, err := c.pullPage(ctx, c.opts.PageSize)
		if err != nil {
			return "", err
		}
		parts = append(parts, summary)
		if c.resets > resets && c.resets == firstReset+1 {
			continue
		}
		if !more {
			break
		}
		if c.cursor == before {
			c.opts.Ledger.Violate(check.Dense, c.opts.Name, "a page after v%d returned no versions but says more exist", before.GetVersion())
			break
		}
	}
	return strings.Join(parts, "; "), nil
}

func (c *Client) pullPage(ctx context.Context, limit uint32) (string, bool, error) {
	creds, err := c.credentials(ctx)
	if err != nil {
		return "", false, err
	}
	req := connect.NewRequest(&storev1.PullRequest{
		ProtocolVersion: ProtocolVersion,
		ClientId:        c.ID(),
		Cursor:          c.cursor,
		MinVersion:      c.minVersion,
		Limit:           limit,
	})
	req.Header().Set("Authorization", "Bearer "+creds.Token)
	ctx, cancel := context.WithTimeout(ctx, RequestTimeout)
	defer cancel()
	resp, err := c.opts.Transport.Store.Pull(ctx, req)
	if err != nil {
		if Retryable(err) {
			c.minVersion = 0
			return "pull failed, will retry: " + err.Error(), false, nil
		}
		return "", false, fmt.Errorf("%s pull: %w", c.opts.Name, err)
	}
	c.minVersion = 0
	msg := resp.Msg
	c.acknowledge(msg.GetLastMutationId())
	if reset := msg.GetReset_(); reset != nil {
		return c.handleReset(reset), false, nil
	}
	return c.applyEntries(msg, limit)
}

func (c *Client) acknowledge(last uint64) {
	if last < c.lastAck {
		c.opts.Ledger.Violate(check.Dense, c.opts.Name, "last_mutation_id went back from m%d to m%d", c.lastAck, last)
	}
	if last >= c.nextMutation {
		c.opts.Ledger.Violate(check.AtMostOnce, c.opts.Name, "last_mutation_id m%d acknowledges a mutation never made (next is m%d)", last, c.nextMutation)
	}
	c.lastAck = max(c.lastAck, last)
	kept := c.pending[:0]
	for _, p := range c.pending {
		if p.ID > c.lastAck {
			kept = append(kept, p)
		}
	}
	c.pending = kept
}

func (c *Client) handleReset(reset *storev1.Reset) string {
	if !c.expectReset {
		c.opts.Ledger.Violate(check.ResetOnMismatch, c.opts.Name, "unexpected Reset (%s) at cursor epoch %d v%d", reset.GetReason(), c.cursor.GetEpoch(), c.cursor.GetVersion())
	}
	if c.resetEpoch != 0 && reset.GetEpoch() < c.resetEpoch {
		c.opts.Ledger.Violate(check.ResetOnMismatch, c.opts.Name, "Reset to epoch %d after the epoch was bumped to at least %d", reset.GetEpoch(), c.resetEpoch)
	}
	c.expectReset, c.resetEpoch = false, 0
	c.resets++
	c.replica.Clear()
	c.log = nil
	c.cursor = &storev1.Cursor{Epoch: reset.GetEpoch()}
	return fmt.Sprintf("Reset to epoch %d (%s)", reset.GetEpoch(), reset.GetReason())
}

func (c *Client) applyEntries(msg *storev1.PullResponse, limit uint32) (string, bool, error) {
	name := c.opts.Name
	if c.expectReset {
		c.opts.Ledger.Violate(check.ResetOnMismatch, name, "cursor epoch %d v%d no longer matches the log, but Pull returned entries instead of Reset", c.cursor.GetEpoch(), c.cursor.GetVersion())
		c.expectReset, c.resetEpoch = false, 0
		c.replica.Clear()
		c.log = nil
		c.cursor = &storev1.Cursor{}
		return "entries where Reset was due; starting over", true, nil
	}
	entries := msg.GetEntries()
	if c.cursor.GetEpoch() == 0 {
		c.cursor.Epoch = msg.GetEpoch()
	} else if msg.GetEpoch() != c.cursor.GetEpoch() {
		c.opts.Ledger.Violate(check.ResetOnMismatch, name, "entries from epoch %d for a cursor at epoch %d", msg.GetEpoch(), c.cursor.GetEpoch())
	}
	versions := entries.GetVersions()
	if limit > 0 && uint32(len(versions)) > limit {
		c.opts.Ledger.Violate(check.WellFormed, name, "asked for at most %d versions, got %d", limit, len(versions))
	}
	from := c.cursor.GetVersion()
	for _, v := range versions {
		c.apply(v)
	}
	if !entries.GetMore() && c.cursor.GetVersion() != msg.GetHeadVersion() {
		c.opts.Ledger.Violate(check.Dense, name, "no more versions after v%d, but head_version is v%d", c.cursor.GetVersion(), msg.GetHeadVersion())
	}
	if len(versions) == 0 {
		return fmt.Sprintf("up to date at v%d", c.cursor.GetVersion()), entries.GetMore(), nil
	}
	return fmt.Sprintf("v%d..v%d of head v%d", from+1, c.cursor.GetVersion(), msg.GetHeadVersion()), entries.GetMore(), nil
}

func (c *Client) apply(v *storev1.Version) {
	name := c.opts.Name
	if want := c.cursor.GetVersion() + 1; v.GetVersion() != want {
		c.opts.Ledger.Violate(check.Dense, name, "after v%d the next version pulled is v%d", c.cursor.GetVersion(), v.GetVersion())
	}
	if computed := hashchain.Next(c.cursor.GetHash(), v); !bytes.Equal(computed, v.GetHash()) {
		c.opts.Ledger.Violate(check.HashChain, name, "v%d hash %x, recomputed %x", v.GetVersion(), v.GetHash(), computed)
	}
	c.opts.Ledger.Delivered(c.ID(), c.cursor.GetEpoch(), v)
	if err := c.replica.Apply(v); err != nil {
		c.opts.Ledger.Violate(check.WellFormed, name, "%v", err)
	}
	c.cursor = &storev1.Cursor{Epoch: c.cursor.GetEpoch(), Version: v.GetVersion(), Hash: v.GetHash()}
	if c.opts.KeepLog {
		c.log = append(c.log, v)
	}
}

// Disconnect takes the client offline and drops its Pokes stream.
func (c *Client) Disconnect() string {
	c.online = false
	c.closePokes()
	return "offline"
}

// Connect brings the client online: it reopens its Pokes stream and pulls,
// as a client does on reconnect.
func (c *Client) Connect(ctx context.Context) (string, error) {
	if c.online {
		return "already online", nil
	}
	c.online = true
	if err := c.openPokes(ctx); err != nil {
		return "", err
	}
	summary, err := c.PullAll(ctx)
	return "online; " + summary, err
}

// Start brings a new client online with its Pokes stream, without pulling.
func (c *Client) Start(ctx context.Context) error {
	c.online = true
	return c.openPokes(ctx)
}

// StaleCursor is a way a cursor stops matching the log.
type StaleCursor int

// The ways the suite corrupts a cursor.
const (
	// WrongHash keeps the version but not its hash, as after a rewound log.
	WrongHash StaleCursor = iota
	// AheadOfHead points past the head, as after a restore lost commits.
	AheadOfHead
)

// Corrupt makes the cursor stale; the next pull must answer Reset.
func (c *Client) Corrupt(kind StaleCursor) string {
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	epoch := max(c.cursor.GetEpoch(), 1)
	switch kind {
	case AheadOfHead:
		c.cursor = &storev1.Cursor{Epoch: epoch, Version: c.cursor.GetVersion() + 1000, Hash: hash}
		c.expectReset = true
		return "cursor moved past the head"
	default:
		c.cursor = &storev1.Cursor{Epoch: epoch, Version: c.cursor.GetVersion(), Hash: hash}
		c.expectReset = true
		return fmt.Sprintf("cursor hash at v%d replaced", c.cursor.GetVersion())
	}
}

// EpochBumped tells the client the scope's epoch moved past epoch; a client
// whose cursor is not fresh must get Reset on its next pull.
func (c *Client) EpochBumped(epoch uint32) {
	if c.cursor.GetEpoch() != 0 && c.cursor.GetEpoch() < epoch {
		c.expectReset = true
		c.resetEpoch = epoch
	}
}

// React pulls when a poke announced a version beyond the cursor.
func (c *Client) React(ctx context.Context) (string, error) {
	if !c.online || !c.opts.ListenPokes {
		return "ignores pokes", nil
	}
	if c.HighestPoke() <= c.cursor.GetVersion() {
		return fmt.Sprintf("no poke beyond v%d", c.cursor.GetVersion()), nil
	}
	summary, err := c.PullAll(ctx)
	return fmt.Sprintf("poked to v%d; %s", c.HighestPoke(), summary), err
}

// Close drops the Pokes stream.
func (c *Client) Close() {
	c.closePokes()
}

func (c *Client) openPokes(ctx context.Context) error {
	if !c.opts.ListenPokes || c.pokes != nil {
		return nil
	}
	creds, err := c.credentials(ctx)
	if err != nil {
		return err
	}
	c.pokes = openPokes(c.opts.Transport.Store, c.ID(), creds.Token, &c.highestPoke)
	return nil
}

func (c *Client) closePokes() {
	if c.pokes != nil {
		c.pokes.close()
		c.pokes = nil
	}
}

func (c *Client) credentials(ctx context.Context) (credentials.Credentials, error) {
	creds, err := c.opts.Credentials.Fetch(ctx, c.opts.User, c.ID())
	if err != nil {
		return credentials.Credentials{}, fmt.Errorf("%s: %w", c.opts.Name, err)
	}
	return creds, nil
}
