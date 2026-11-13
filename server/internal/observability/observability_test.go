package observability

import "testing"

func TestLogRedactsSecretsAndBounds(t *testing.T) {
	l := New(2)
	l.Add("join", "p", "", map[string]any{"token": "hidden", "ok": true})
	l.Add("action", "p", "mine", nil)
	l.Add("third", "p", "", nil)
	got := l.Snapshot(10)
	if len(got) != 2 || got[0].Kind != "action" || got[1].Kind != "third" {
		t.Fatalf("events: %+v", got)
	}
	if _, ok := got[0].Meta["token"]; ok {
		t.Fatal("secret leaked")
	}
}
