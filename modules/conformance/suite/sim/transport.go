// Package sim holds the conformance suite's simulated client: a pending
// queue, a cursor and an in-memory replica, driven over the wire against a
// product's Push and the store service's Pull and Pokes.
package sim

import (
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"github.com/skkil/store/modules/conformance/gen/storev1/storev1connect"
)

// ProtocolVersion is the wire protocol version the simulated clients speak.
const ProtocolVersion = 1

// Transport reaches a product's PushService and the store service.
type Transport struct {
	Push  storev1connect.PushServiceClient
	Store storev1connect.StoreServiceClient
}

// NewTransport returns a Transport over httpClient using the Connect
// protocol with binary protobuf, so change data arrives byte for byte.
func NewTransport(httpClient *http.Client, pushURL, storeURL string) *Transport {
	return &Transport{
		Push:  storev1connect.NewPushServiceClient(httpClient, pushURL),
		Store: storev1connect.NewStoreServiceClient(httpClient, storeURL),
	}
}

// Retryable reports whether err is a transport failure the protocol says to
// retry ("try the whole batch again"), as opposed to a misconfiguration.
func Retryable(err error) bool {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return true
	}
	switch connectErr.Code() {
	case connect.CodeUnavailable, connect.CodeDeadlineExceeded, connect.CodeAborted,
		connect.CodeResourceExhausted, connect.CodeInternal, connect.CodeUnknown, connect.CodeCanceled:
		return true
	default:
		return false
	}
}
