package reference_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/gen/storev1/storev1connect"
	"github.com/skkil/store/modules/conformance/internal/stacktest"
	"github.com/skkil/store/modules/conformance/reference"
	"github.com/skkil/store/modules/conformance/writepath"
)

type harness struct {
	t      *testing.T
	db     *stacktest.Database
	issuer *reference.TokenIssuer
	push   storev1connect.PushServiceClient
	url    string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	db := stacktest.NewDatabase(t)
	key, err := reference.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	issuer := reference.NewTokenIssuer("reference", key)
	handler, err := reference.NewHandler(context.Background(), reference.Config{
		Pool: db.Pool, Issuer: issuer, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &harness{t: t, db: db, issuer: issuer, push: storev1connect.NewPushServiceClient(http.DefaultClient, server.URL), url: server.URL}
}

type device struct {
	scope, client uuid.UUID
	token         string
}

func (h *harness) device() device {
	h.t.Helper()
	d := device{scope: uuid.New(), client: uuid.New()}
	token, err := h.issuer.Issue(d.scope, d.client)
	if err != nil {
		h.t.Fatal(err)
	}
	d.token = token
	return d
}

func (h *harness) send(d device, mutations ...*storev1.Mutation) ([]*storev1.MutationResult, error) {
	h.t.Helper()
	req := connect.NewRequest(&storev1.PushRequest{ProtocolVersion: 1, ClientId: d.client.String(), Mutations: mutations})
	req.Header().Set("Authorization", "Bearer "+d.token)
	resp, err := h.push.Push(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return resp.Msg.GetResults(), nil
}

func (h *harness) mustSend(d device, mutations ...*storev1.Mutation) []*storev1.MutationResult {
	h.t.Helper()
	results, err := h.send(d, mutations...)
	if err != nil {
		h.t.Fatal(err)
	}
	return results
}

func mutation(id uint64, name, args string) *storev1.Mutation {
	return &storev1.Mutation{MutationId: id, Name: name, Args: []byte(args)}
}

type change struct {
	table, rowID, op string
}

func (h *harness) changes(scope uuid.UUID, version uint64) []change {
	h.t.Helper()
	rows, err := h.db.Pool.Query(context.Background(),
		"SELECT table_name, row_id, op FROM store_log WHERE scope_id = $1 AND version = $2 AND kind = 'patch' ORDER BY seq",
		scope, int64(version))
	if err != nil {
		h.t.Fatal(err)
	}
	defer rows.Close()
	var out []change
	for rows.Next() {
		var c change
		if err := rows.Scan(&c.table, &c.rowID, &c.op); err != nil {
			h.t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

func TestCreateUsesDerivedIDsAndOneVersion(t *testing.T) {
	h := newHarness(t)
	d := h.device()
	results := h.mustSend(d, mutation(1, "todo.create", `{"title":"groceries","items":["milk","eggs"]}`))
	if results[0].GetStatus() != storev1.MutationResult_STATUS_APPLIED || results[0].GetVersion() != 1 {
		t.Fatalf("result = %v, want applied v1", results[0])
	}
	want := []change{
		{"todo", writepath.DeriveID(d.client, 1, 0).String(), "upsert"},
		{"todo_item", writepath.DeriveID(d.client, 1, 1).String(), "upsert"},
		{"todo_item", writepath.DeriveID(d.client, 1, 2).String(), "upsert"},
	}
	got := h.changes(d.scope, 1)
	if len(got) != len(want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("change %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestDeleteCascadesToItemsInTheSameVersion(t *testing.T) {
	h := newHarness(t)
	d := h.device()
	h.mustSend(d, mutation(1, "todo.create", `{"title":"trip","items":["passport"]}`))
	todo := writepath.DeriveID(d.client, 1, 0).String()
	results := h.mustSend(d, mutation(2, "todo.delete", `{"id":"`+todo+`"}`))
	if results[0].GetStatus() != storev1.MutationResult_STATUS_APPLIED {
		t.Fatalf("delete = %v", results[0])
	}
	got := h.changes(d.scope, results[0].GetVersion())
	deletes := map[string]bool{}
	for _, c := range got {
		if c.op != "delete" {
			t.Fatalf("unexpected change %v", c)
		}
		deletes[c.table] = true
	}
	if !deletes["todo"] || !deletes["todo_item"] {
		t.Fatalf("changes = %v, want the todo and its item deleted", got)
	}
}

func TestRejectionsAreResultsAndLeaveNoRows(t *testing.T) {
	h := newHarness(t)
	d := h.device()
	results := h.mustSend(d,
		mutation(1, "todo.create", `{"title":""}`),
		mutation(2, "todo.create", `{"title":"half","items":["ok"," "]}`),
		mutation(3, "todo.rename", `{"id":"`+uuid.NewString()+`","title":"x"}`),
		mutation(4, "nope", `{}`),
		mutation(5, "todo.create", `[1]`),
		mutation(6, "todo.create", `{"title":7}`),
	)
	reasons := []string{
		reference.ReasonEmptyTitle,
		reference.ReasonEmptyItem,
		reference.ReasonTodoNotFound,
		"unknown mutation: nope",
		reference.ReasonBadArguments + ": expected a JSON object",
	}
	for i, r := range results {
		if r.GetStatus() != storev1.MutationResult_STATUS_REJECTED || r.GetVersion() != uint64(i+1) {
			t.Fatalf("result %d = %v, want rejected v%d", i, r, i+1)
		}
		if i < len(reasons) && r.GetReason() != reasons[i] {
			t.Errorf("result %d reason = %q, want %q", i, r.GetReason(), reasons[i])
		}
	}
	var todos int
	if err := h.db.Pool.QueryRow(context.Background(), "SELECT count(*) FROM todo WHERE scope_id = $1", d.scope).Scan(&todos); err != nil {
		t.Fatal(err)
	}
	if todos != 0 {
		t.Fatalf("%d todos left behind by rejections", todos)
	}
}

func TestRepeatedPushAnswersDuplicateWithTheRecordedVersion(t *testing.T) {
	h := newHarness(t)
	d := h.device()
	h.mustSend(d, mutation(1, "todo.create", `{"title":"once"}`))
	results := h.mustSend(d, mutation(1, "todo.create", `{"title":"once"}`), mutation(2, "todo.create", `{"title":"twice"}`))
	if results[0].GetStatus() != storev1.MutationResult_STATUS_DUPLICATE || results[0].GetVersion() != 1 {
		t.Fatalf("re-push = %v, want duplicate v1", results[0])
	}
	if results[1].GetStatus() != storev1.MutationResult_STATUS_APPLIED || results[1].GetVersion() != 2 {
		t.Fatalf("next = %v, want applied v2", results[1])
	}
}

func TestPushRefusesBadRequests(t *testing.T) {
	h := newHarness(t)
	d := h.device()
	other := h.device()
	tests := []struct {
		name string
		req  *storev1.PushRequest
		auth string
		want connect.Code
	}{
		{"no token", &storev1.PushRequest{ProtocolVersion: 1, ClientId: d.client.String()}, "", connect.CodeUnauthenticated},
		{"foreign token", &storev1.PushRequest{ProtocolVersion: 1, ClientId: d.client.String()}, "Bearer " + foreignToken(t, d), connect.CodeUnauthenticated},
		{"old protocol", &storev1.PushRequest{ProtocolVersion: 2, ClientId: d.client.String()}, "Bearer " + d.token, connect.CodeFailedPrecondition},
		{"bad client id", &storev1.PushRequest{ProtocolVersion: 1, ClientId: "x"}, "Bearer " + d.token, connect.CodeInvalidArgument},
		{"someone else's client", &storev1.PushRequest{ProtocolVersion: 1, ClientId: other.client.String()}, "Bearer " + d.token, connect.CodePermissionDenied},
		{"mutation id 0", &storev1.PushRequest{ProtocolVersion: 1, ClientId: d.client.String(), Mutations: []*storev1.Mutation{mutation(0, "todo.create", `{"title":"a"}`)}}, "Bearer " + d.token, connect.CodeInvalidArgument},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := connect.NewRequest(tc.req)
			if tc.auth != "" {
				req.Header().Set("Authorization", tc.auth)
			}
			_, err := h.push.Push(context.Background(), req)
			if got := connect.CodeOf(err); got != tc.want {
				t.Fatalf("code = %v (%v), want %v", got, err, tc.want)
			}
		})
	}
}

func TestClientsStayInTheirScope(t *testing.T) {
	h := newHarness(t)
	d := h.device()
	h.mustSend(d, mutation(1, "todo.create", `{"title":"mine"}`))
	moved := device{scope: uuid.New(), client: d.client}
	token, err := h.issuer.Issue(moved.scope, moved.client)
	if err != nil {
		t.Fatal(err)
	}
	moved.token = token
	_, err = h.send(moved, mutation(2, "todo.create", `{"title":"theirs"}`))
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodePermissionDenied {
		t.Fatalf("err = %v, want permission_denied", err)
	}
	intruder := h.device()
	todo := writepath.DeriveID(d.client, 1, 0).String()
	results := h.mustSend(intruder, mutation(1, "todo.rename", `{"id":"`+todo+`","title":"stolen"}`))
	if results[0].GetStatus() != storev1.MutationResult_STATUS_REJECTED {
		t.Fatalf("renaming another scope's todo = %v, want rejected", results[0])
	}
}

func foreignToken(t *testing.T, d device) string {
	t.Helper()
	key, err := reference.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	token, err := reference.NewTokenIssuer("reference", key).Issue(d.scope, d.client)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
