package reference

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/skkil/store/modules/conformance/writepath"
)

// Handler runs one named mutation with its JSON arguments inside the write path.
type Handler func(ctx context.Context, m *writepath.Mutation, args json.RawMessage) error

// Registry maps mutation names to handlers.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{handlers: map[string]Handler{}}
}

// Register adds handler under name. Registering a name twice panics.
func (r *Registry) Register(name string, handler Handler) {
	if _, ok := r.handlers[name]; ok {
		panic(fmt.Sprintf("reference: mutation %q registered twice", name))
	}
	r.handlers[name] = handler
}

// Body returns the write-path body for one pushed mutation. Unknown names and
// arguments that are not a JSON object become rejections.
func (r *Registry) Body(name string, args []byte) writepath.Body {
	handler, ok := r.handlers[name]
	if !ok {
		return rejectWith(reasonUnknownPrefix + name)
	}
	if len(args) == 0 {
		args = []byte("{}")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(args, &object); err != nil || object == nil {
		return rejectWith(ReasonBadArguments + ": expected a JSON object")
	}
	return func(ctx context.Context, m *writepath.Mutation) error {
		return handler(ctx, m, args)
	}
}

func rejectWith(reason string) writepath.Body {
	return func(context.Context, *writepath.Mutation) error {
		return writepath.Reject(reason)
	}
}
