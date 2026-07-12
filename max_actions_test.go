package sieve

import (
	"bufio"
	"context"
	"net/textproto"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/interp"
)

// TestMaxActions verifies the sieve_max_actions cap: a script applying more
// actions than the limit aborts, while one within the limit runs to completion.
func TestMaxActions(t *testing.T) {
	const eml = "From: a@test.com\r\nTo: b@test.com\r\nSubject: hi\r\n\r\nbody\r\n"
	// Three distinct fileinto actions.
	const script = `require ["fileinto"];
fileinto "A";
fileinto "B";
fileinto "C";
`
	run := func(t *testing.T, maxActions int) error {
		t.Helper()
		hdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(eml))).ReadMIMEHeader()
		if err != nil {
			t.Fatal(err)
		}
		opts := DefaultOptions()
		opts.Interp.MaxActions = maxActions
		loaded, err := Load(bufio.NewReader(strings.NewReader(script)), opts)
		if err != nil {
			t.Fatal(err)
		}
		data := interp.NewRuntimeData(loaded, interp.DummyPolicy{},
			interp.EnvelopeStatic{From: "a@test.com", To: "b@test.com"},
			interp.MessageStatic{Size: len(eml), Header: hdr})
		return loaded.Execute(context.Background(), data)
	}

	t.Run("over limit aborts", func(t *testing.T) {
		if err := run(t, 2); err == nil {
			t.Fatal("expected error when actions exceed MaxActions=2, got nil")
		}
	})
	t.Run("within limit ok", func(t *testing.T) {
		if err := run(t, 3); err != nil {
			t.Fatalf("3 actions within MaxActions=3 should pass, got: %v", err)
		}
	})
	t.Run("zero means unlimited", func(t *testing.T) {
		if err := run(t, 0); err != nil {
			t.Fatalf("MaxActions=0 (unlimited) should pass, got: %v", err)
		}
	})
}
