package e2e

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	storev1 "github.com/skkil/store/modules/service/gen/storev1"
	"github.com/skkil/store/modules/service/internal/storetest"
)

func TestPullOverEveryProtocol(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	w := storetest.Writer{Pool: db.Pool}
	mutate(t, w, d, 1)
	mutate(t, w, d, 2)

	clients := map[string]func() *connect.Response[storev1.PullResponse]{
		"connect json over http/1.1": func() *connect.Response[storev1.PullResponse] {
			resp, err := inst.client(connect.WithProtoJSON()).Pull(context.Background(), pullRequest(d, nil))
			if err != nil {
				t.Fatal(err)
			}
			return resp
		},
		"connect proto over h2c": func() *connect.Response[storev1.PullResponse] {
			resp, err := inst.h2cClient().Pull(context.Background(), pullRequest(d, nil))
			if err != nil {
				t.Fatal(err)
			}
			return resp
		},
		"grpc over h2c": func() *connect.Response[storev1.PullResponse] {
			resp, err := inst.h2cClient(connect.WithGRPC()).Pull(context.Background(), pullRequest(d, nil))
			if err != nil {
				t.Fatal(err)
			}
			return resp
		},
		"grpc-web over http/1.1": func() *connect.Response[storev1.PullResponse] {
			resp, err := inst.client(connect.WithGRPCWeb()).Pull(context.Background(), pullRequest(d, nil))
			if err != nil {
				t.Fatal(err)
			}
			return resp
		},
	}
	for name, call := range clients {
		t.Run(name, func(t *testing.T) {
			msg := call().Msg
			if msg.GetHeadVersion() != 2 || msg.GetLastMutationId() != 2 || len(msg.GetEntries().GetVersions()) != 2 {
				t.Fatalf("got %v", msg)
			}
		})
	}
}

func TestPullRefusesBadTokens(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	stranger := storetest.NewIssuer(t, "time")
	claims := func(edit func(jwt.MapClaims)) jwt.MapClaims {
		c := jwt.MapClaims{
			"iss": "time", "aud": "skkil-store", "sub": d.scope, "cid": d.client,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Minute).Unix(),
		}
		edit(c)
		return c
	}
	tokens := map[string]string{
		"wrong issuer":   issuer.Sign(t, claims(func(c jwt.MapClaims) { c["iss"] = "clip" })),
		"wrong audience": issuer.Sign(t, claims(func(c jwt.MapClaims) { c["aud"] = "time" })),
		"expired": issuer.Sign(t, claims(func(c jwt.MapClaims) {
			c["iat"] = time.Now().Add(-time.Hour).Unix()
			c["exp"] = time.Now().Add(-50 * time.Minute).Unix()
		})),
		"bad signature": stranger.Token(t, d.scope, d.client),
		"missing":       "",
	}
	for name, token := range tokens {
		t.Run(name, func(t *testing.T) {
			req := connect.NewRequest(&storev1.PullRequest{ProtocolVersion: 1, ClientId: d.client})
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			_, err := inst.client().Pull(context.Background(), req)
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatalf("got %v", err)
			}
			stream, err := inst.client().Pokes(context.Background(), connectPokes(d, token))
			if err == nil {
				for stream.Receive() {
				}
				err = stream.Err()
				_ = stream.Close()
			}
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatalf("pokes: got %v", err)
			}
		})
	}
}

func connectPokes(d device, token string) *connect.Request[storev1.PokesRequest] {
	req := connect.NewRequest(&storev1.PokesRequest{ProtocolVersion: 1, ClientId: d.client})
	if token != "" {
		req.Header().Set("Authorization", "Bearer "+token)
	}
	return req
}

func TestPullRequiresProtocolVersion1(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	for _, version := range []uint32{0, 2} {
		req := pullRequest(d, nil)
		req.Msg.ProtocolVersion = version
		if _, err := inst.client().Pull(context.Background(), req); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("protocol_version %d: got %v", version, err)
		}
	}
}

func TestPullRequiresTheTokensClient(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	req := pullRequest(d, nil)
	req.Msg.ClientId = uuid.NewString()
	if _, err := inst.client().Pull(context.Background(), req); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestOneTokenCannotReadAnotherScope(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	owner, other := newDevice(t, issuer), newDevice(t, issuer)
	mutate(t, storetest.Writer{Pool: db.Pool}, owner, 1)
	resp, err := inst.client().Pull(context.Background(), pullRequest(other, nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetHeadVersion() != 0 || len(resp.Msg.GetEntries().GetVersions()) != 0 {
		t.Fatalf("other scope read %v", resp.Msg)
	}
}

func TestPullAnswersUnavailableBelowMinVersion(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	req := pullRequest(d, nil)
	req.Msg.MinVersion = 3
	if _, err := inst.client().Pull(context.Background(), req); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatalf("got %v", err)
	}
}

func TestPullResetsOverTheWire(t *testing.T) {
	db := storetest.NewDatabase(t)
	issuer := storetest.NewIssuer(t, "time")
	inst := start(t, baseConfig(t, db, issuer))
	d := newDevice(t, issuer)
	mutate(t, storetest.Writer{Pool: db.Pool}, d, 1)
	resp, err := inst.client().Pull(context.Background(), pullRequest(d, &storev1.Cursor{Epoch: 1, Version: 1, Hash: []byte("forged")}))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Msg.GetReset_().GetReason() != "hash mismatch" {
		t.Fatalf("got %v", resp.Msg)
	}
}
