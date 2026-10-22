package credentials

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func fakeJWT(t *testing.T, sub, cid string, exp time.Time) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{"sub": sub, "cid": cid, "exp": exp.Unix()})
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"ES256"}`)) + "." + enc(payload) + ".sig"
}

func TestParseReadsABareToken(t *testing.T) {
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	token := fakeJWT(t, "scope-1", "client-1", exp)
	c, err := Parse([]byte(token+"\n"), "client-1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != token || c.Scope != "scope-1" || !c.Expiry.Equal(exp) {
		t.Fatalf("credentials = %+v", c)
	}
	if got := c.PushHeaders.Get("Authorization"); got != "Bearer "+token {
		t.Fatalf("push Authorization = %q", got)
	}
}

func TestParseReadsJSONWithPushHeaders(t *testing.T) {
	token := fakeJWT(t, "scope-1", "client-1", time.Now().Add(time.Hour))
	c, err := Parse([]byte(`{"token":"`+token+`","push_headers":{"Authorization":"Bearer user-session","X-Tenant":"t1"}}`), "client-1")
	if err != nil {
		t.Fatal(err)
	}
	if c.PushHeaders.Get("Authorization") != "Bearer user-session" || c.PushHeaders.Get("X-Tenant") != "t1" {
		t.Fatalf("push headers = %v", c.PushHeaders)
	}
}

func TestParseRefusesBadAnswers(t *testing.T) {
	good := time.Now().Add(time.Hour)
	for name, answer := range map[string]string{
		"empty":        "",
		"not a jwt":    "abc",
		"bad payload":  "a.!!!.c",
		"wrong client": fakeJWT(t, "scope", "other", good),
		"no scope":     fakeJWT(t, "", "client-1", good),
		"bad json":     `{"token":`,
	} {
		if _, err := Parse([]byte(answer), "client-1"); err == nil {
			t.Errorf("%s: accepted %q", name, answer)
		}
	}
}

func TestURLSourcePostsUserAndClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || r.Method != http.MethodPost || r.Header.Get("X-Key") != "k" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, fakeJWT(t, "scope-of-"+req["user"], req["client_id"], time.Now().Add(time.Hour)))
	}))
	defer server.Close()
	source := URLSource{URL: server.URL, Header: http.Header{"X-Key": {"k"}}}
	c, err := source.Fetch(context.Background(), "ada", "client-1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Scope != "scope-of-ada" {
		t.Fatalf("scope = %q", c.Scope)
	}
	if _, err := (URLSource{URL: server.URL}).Fetch(context.Background(), "ada", "client-1"); err == nil {
		t.Fatal("a 400 answer was accepted")
	}
}

func TestCommandSourcePassesTheEnvironment(t *testing.T) {
	token := fakeJWT(t, "scope-1", "client-1", time.Now().Add(time.Hour))
	source := CommandSource{Command: `test "$` + EnvUser + `" = ada && test "$` + EnvClientID + `" = client-1 && echo ` + token}
	c, err := source.Fetch(context.Background(), "ada", "client-1")
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != token {
		t.Fatalf("token = %q", c.Token)
	}
	if _, err := (CommandSource{Command: "echo oops >&2; exit 3"}).Fetch(context.Background(), "ada", "client-1"); err == nil {
		t.Fatal("a failing command was accepted")
	}
}

type countingSource struct {
	calls atomic.Int32
	ttl   time.Duration
	t     *testing.T
}

func (s *countingSource) Fetch(_ context.Context, user, client string) (Credentials, error) {
	s.calls.Add(1)
	return Parse([]byte(fakeJWT(s.t, user, client, time.Now().Add(s.ttl))), client)
}

func TestCacheRefreshesShortlyBeforeExpiry(t *testing.T) {
	long := &countingSource{ttl: time.Hour, t: t}
	cache := NewCache(long)
	for range 3 {
		if _, err := cache.Fetch(context.Background(), "u", "c"); err != nil {
			t.Fatal(err)
		}
	}
	if long.calls.Load() != 1 {
		t.Fatalf("long-lived token fetched %d times", long.calls.Load())
	}
	short := &countingSource{ttl: time.Minute, t: t}
	cache = NewCache(short)
	for range 2 {
		if _, err := cache.Fetch(context.Background(), "u", "c"); err != nil {
			t.Fatal(err)
		}
	}
	if short.calls.Load() != 2 {
		t.Fatalf("token expiring within %s fetched %d times, want 2", RefreshBefore, short.calls.Load())
	}
}
