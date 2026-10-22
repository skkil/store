package reference_test

import (
	"testing"

	"github.com/skkil/store/modules/conformance/reference"
	"github.com/skkil/store/modules/conformance/suite/profile"
)

func TestProfileDescribesEveryMutation(t *testing.T) {
	p, err := profile.Parse(reference.Profile)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, m := range p.Mutations {
		names[m.Name] = true
	}
	for _, name := range []string{"todo.create", "todo.rename", "todo.complete", "todo.delete", "item.create"} {
		if !names[name] {
			t.Errorf("the profile leaves out %s", name)
		}
	}
}
