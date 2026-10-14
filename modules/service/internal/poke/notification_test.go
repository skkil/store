package poke

import "testing"

func TestParseNotification(t *testing.T) {
	n, err := ParseNotification("6F9619FF-8B86-D011-B42D-00C04FC964FF:42")
	if err != nil {
		t.Fatal(err)
	}
	if n != (Notification{Scope: "6f9619ff-8b86-d011-b42d-00c04fc964ff", Version: 42}) {
		t.Fatalf("got %+v", n)
	}
	for _, bad := range []string{"", "scope", "not-a-uuid:1", "6f9619ff-8b86-d011-b42d-00c04fc964ff:", "6f9619ff-8b86-d011-b42d-00c04fc964ff:-1"} {
		if _, err := ParseNotification(bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
}
