// Package e2e runs whole service instances over HTTP against real Postgres.
package e2e

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
	"github.com/skkil/store/modules/service/gen/storev1/storev1connect"
	"github.com/skkil/store/modules/service/internal/app"
	"github.com/skkil/store/modules/service/internal/config"
	"github.com/skkil/store/modules/service/internal/storetest"
)

type instance struct {
	url    string
	stop   context.CancelFunc
	served chan error
}

func baseConfig(t *testing.T, db *storetest.Database, issuer *storetest.Issuer) config.Config {
	t.Helper()
	return config.Config{
		DatabaseURL:       db.URL,
		TokenIssuer:       issuer.Name,
		TokenPublicKeyPEM: issuer.PublicKeyPEM(t),
		PollInterval:      time.Hour,
		PokeKeepAlive:     time.Hour,
		MinVersionWait:    200 * time.Millisecond,
		ShutdownTimeout:   5 * time.Second,
	}
}

func start(t *testing.T, cfg config.Config) *instance {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	a, err := app.New(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	inst := &instance{url: "http://" + ln.Addr().String(), stop: cancel, served: make(chan error, 1)}
	go func() { inst.served <- a.Serve(ctx, ln) }()
	t.Cleanup(func() {
		inst.shutdown(t)
		a.Close()
	})
	return inst
}

func (i *instance) shutdown(t *testing.T) error {
	t.Helper()
	i.stop()
	select {
	case err, ok := <-i.served:
		if ok {
			close(i.served)
		}
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("instance did not stop")
		return nil
	}
}

func (i *instance) client(opts ...connect.ClientOption) storev1connect.StoreServiceClient {
	return storev1connect.NewStoreServiceClient(http.DefaultClient, i.url, opts...)
}

func (i *instance) h2cClient(opts ...connect.ClientOption) storev1connect.StoreServiceClient {
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	httpClient := &http.Client{Transport: &http.Transport{Protocols: &protocols}}
	return storev1connect.NewStoreServiceClient(httpClient, i.url, opts...)
}

type device struct {
	scope, client, token string
}

func newDevice(t *testing.T, issuer *storetest.Issuer) device {
	t.Helper()
	scope, client := uuid.NewString(), uuid.NewString()
	return device{scope: scope, client: client, token: issuer.Token(t, scope, client)}
}

func pullRequest(d device, cursor *storev1.Cursor) *connect.Request[storev1.PullRequest] {
	req := connect.NewRequest(&storev1.PullRequest{ProtocolVersion: 1, ClientId: d.client, Cursor: cursor})
	req.Header().Set("Authorization", "Bearer "+d.token)
	return req
}

func pokesRequest(d device) *connect.Request[storev1.PokesRequest] {
	req := connect.NewRequest(&storev1.PokesRequest{ProtocolVersion: 1, ClientId: d.client})
	req.Header().Set("Authorization", "Bearer "+d.token)
	return req
}

func mutate(t *testing.T, w storetest.Writer, d device, id uint64) storetest.Result {
	t.Helper()
	result, err := w.Mutate(context.Background(), d.scope, d.client, id, storetest.InsertNote(uuid.NewString(), d.scope, "n"))
	if err != nil {
		t.Fatal(err)
	}
	return result
}

type pokeReader struct {
	versions chan uint64
	errs     chan error
}

func openPokes(t *testing.T, client storev1connect.StoreServiceClient, d device) *pokeReader {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	stream, err := client.Pokes(ctx, pokesRequest(d))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	r := &pokeReader{versions: make(chan uint64, 64), errs: make(chan error, 1)}
	go func() {
		defer close(r.versions)
		for stream.Receive() {
			r.versions <- stream.Msg().GetVersion()
		}
		r.errs <- stream.Err()
	}()
	t.Cleanup(func() {
		cancel()
		_ = stream.Close()
	})
	return r
}

func (r *pokeReader) next(t *testing.T) uint64 {
	t.Helper()
	select {
	case v, ok := <-r.versions:
		if !ok {
			t.Fatalf("stream ended: %v", <-r.errs)
		}
		return v
	case <-time.After(5 * time.Second):
		t.Fatal("no poke")
		return 0
	}
}

func (r *pokeReader) await(t *testing.T, want uint64) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case v, ok := <-r.versions:
			if !ok {
				t.Fatalf("stream ended: %v", <-r.errs)
			}
			if v == want {
				return
			}
		case <-deadline:
			t.Fatalf("no poke for version %d", want)
		}
	}
}
