package interp

import (
	"context"
	"strings"
	"testing"
)

// buildMsg joins lines with CRLF into a raw MIME message.
func buildMsg(lines ...string) []byte {
	return []byte(strings.Join(lines, "\r\n"))
}

// nested multipart: mixed[ text/plain, alternative[ text/plain, text/html ] ]
var mimeSample = buildMsg(
	"Content-Type: multipart/mixed; boundary=outer",
	"",
	"--outer",
	"Content-Type: text/plain",
	"",
	"hello",
	"--outer",
	"Content-Type: multipart/alternative; boundary=inner",
	"",
	"--inner",
	"Content-Type: text/plain",
	"",
	"alt1",
	"--inner",
	"Content-Type: text/html",
	"",
	"<p>alt2</p>",
	"--inner--",
	"--outer--",
	"",
)

func TestParseMIMETree(t *testing.T) {
	tree, err := parseMIMETree(mimeSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tree.contentType != "multipart/mixed" || len(tree.children) != 2 {
		t.Fatalf("root = %q with %d children, want multipart/mixed with 2", tree.contentType, len(tree.children))
	}
	got := []string{}
	for _, p := range tree.descendants() {
		got = append(got, p.contentType)
	}
	want := []string{"text/plain", "multipart/alternative", "text/plain", "text/html"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("descendants = %v, want %v", got, want)
	}
	// type/subtype split
	if tree.children[1].typ != "multipart" || tree.children[1].subtype != "alternative" {
		t.Errorf("alt part type/subtype = %q/%q", tree.children[1].typ, tree.children[1].subtype)
	}
}

type countCmd struct{ n *int }

func (c countCmd) Execute(_ context.Context, _ *RuntimeData) error { *c.n++; return nil }

type breakCmd struct{ name string }

func (c breakCmd) Execute(_ context.Context, _ *RuntimeData) error {
	return &breakSignal{name: c.name}
}

func newData(raw []byte) *RuntimeData {
	return NewRuntimeData(&Script{}, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{RawMessage: raw})
}

func TestForEveryPart_Count(t *testing.T) {
	n := 0
	d := newData(mimeSample)
	cmd := CmdForEveryPart{Block: []Cmd{countCmd{&n}}}
	if err := cmd.Execute(context.Background(), d); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if n != 4 {
		t.Errorf("block ran %d times, want 4 (all parts depth-first)", n)
	}
}

func TestForEveryPart_SinglePart(t *testing.T) {
	// A non-multipart message has exactly one part: itself.
	raw := buildMsg("Content-Type: text/plain", "", "just text", "")
	n := 0
	d := newData(raw)
	if err := (CmdForEveryPart{Block: []Cmd{countCmd{&n}}}).Execute(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("single-part ran %d times, want 1", n)
	}
}

func TestForEveryPart_Break(t *testing.T) {
	n := 0
	d := newData(mimeSample)
	// Count once then break — should stop after the first part.
	cmd := CmdForEveryPart{Block: []Cmd{countCmd{&n}, breakCmd{}}}
	if err := cmd.Execute(context.Background(), d); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if n != 1 {
		t.Errorf("break: block ran %d times, want 1", n)
	}
}

func TestForEveryPart_NoRawMessage(t *testing.T) {
	// Message without raw bytes → foreverypart iterates nothing.
	n := 0
	d := NewRuntimeData(&Script{}, DummyPolicy{}, EnvelopeStatic{}, MessageStatic{})
	if err := (CmdForEveryPart{Block: []Cmd{countCmd{&n}}}).Execute(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("no-raw ran %d times, want 0", n)
	}
}
