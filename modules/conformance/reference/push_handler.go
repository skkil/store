package reference

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/gen/storev1/storev1connect"
	"github.com/skkil/store/modules/conformance/writepath"
)

// ProtocolVersion is the only wire protocol version the reference speaks.
const ProtocolVersion = 1

var _ storev1connect.PushServiceHandler = (*PushHandler)(nil)

// PushHandler serves skkil.store.v1.PushService. The caller authenticates
// with a sync token this product issued (a real product would use its own
// sign-in); the token's scope is the scope every mutation writes to.
type PushHandler struct {
	issuer   *TokenIssuer
	registry *Registry
	applier  Applier
	logger   *slog.Logger
}

// NewPushHandler returns a PushHandler applying registry's mutations through applier.
func NewPushHandler(issuer *TokenIssuer, registry *Registry, applier Applier, logger *slog.Logger) *PushHandler {
	return &PushHandler{issuer: issuer, registry: registry, applier: applier, logger: logger}
}

// Push applies each mutation in its own transaction, in order. A rejection
// is a result; any other failure aborts the batch with an error so the client
// retries it, and mutations committed before the failure answer DUPLICATE then.
func (h *PushHandler) Push(ctx context.Context, req *connect.Request[storev1.PushRequest]) (*connect.Response[storev1.PushResponse], error) {
	identity, err := h.authenticate(req.Header().Get("Authorization"))
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	msg := req.Msg
	if msg.GetProtocolVersion() != ProtocolVersion {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("unsupported protocol_version; this server speaks 1"))
	}
	client, err := uuid.Parse(msg.GetClientId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("client_id is not a UUID"))
	}
	if client != identity.Client {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("client_id does not match the token"))
	}
	results := make([]*storev1.MutationResult, 0, len(msg.GetMutations()))
	for _, mutation := range msg.GetMutations() {
		body := h.registry.Body(mutation.GetName(), mutation.GetArgs())
		result, err := h.applier.Apply(ctx, identity.Scope, client, mutation.GetMutationId(), body)
		if err != nil {
			return nil, h.failure(mutation, err)
		}
		results = append(results, &storev1.MutationResult{
			MutationId: mutation.GetMutationId(),
			Status:     status(result.Status),
			Reason:     result.Reason,
			Version:    result.Version,
		})
	}
	return connect.NewResponse(&storev1.PushResponse{Results: results}), nil
}

func (h *PushHandler) authenticate(header string) (Identity, error) {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || token == "" {
		return Identity{}, errors.New("missing bearer token")
	}
	return h.issuer.Verify(token)
}

func (h *PushHandler) failure(mutation *storev1.Mutation, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42501":
			return connect.NewError(connect.CodePermissionDenied, errors.New(pgErr.Message))
		case "22023":
			return connect.NewError(connect.CodeInvalidArgument, errors.New(pgErr.Message))
		}
	}
	h.logger.Error("push failed", "mutation_id", mutation.GetMutationId(), "name", mutation.GetName(), "error", err)
	return connect.NewError(connect.CodeUnavailable, errors.New("mutation failed; retry the batch"))
}

func status(s writepath.Status) storev1.MutationResult_Status {
	switch s {
	case writepath.Applied:
		return storev1.MutationResult_STATUS_APPLIED
	case writepath.Duplicate:
		return storev1.MutationResult_STATUS_DUPLICATE
	case writepath.Rejected:
		return storev1.MutationResult_STATUS_REJECTED
	default:
		return storev1.MutationResult_STATUS_UNSPECIFIED
	}
}
