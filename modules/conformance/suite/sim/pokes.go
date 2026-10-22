package sim

import (
	"context"
	"sync/atomic"

	"connectrpc.com/connect"

	storev1 "github.com/skkil/store/modules/conformance/gen/storev1"
	"github.com/skkil/store/modules/conformance/gen/storev1/storev1connect"
)

type pokeStream struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func openPokes(store storev1connect.StoreServiceClient, client, token string, highest *atomic.Uint64) *pokeStream {
	ctx, cancel := context.WithCancel(context.Background())
	s := &pokeStream{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		req := connect.NewRequest(&storev1.PokesRequest{ProtocolVersion: ProtocolVersion, ClientId: client})
		req.Header().Set("Authorization", "Bearer "+token)
		stream, err := store.Pokes(ctx, req)
		if err != nil {
			return
		}
		defer stream.Close()
		for stream.Receive() {
			v := stream.Msg().GetVersion()
			for {
				current := highest.Load()
				if v <= current || highest.CompareAndSwap(current, v) {
					break
				}
			}
		}
	}()
	return s
}

func (s *pokeStream) close() {
	s.cancel()
	<-s.done
}
