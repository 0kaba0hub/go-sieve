package sieve

import (
	"bufio"
	"context"
	"net/textproto"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/interp"
)

func runScript(t *testing.T, script, eml string) []interp.AppliedAction {
	t.Helper()
	hdr, err := textproto.NewReader(bufio.NewReader(strings.NewReader(eml))).ReadMIMEHeader()
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(bufio.NewReader(strings.NewReader(script)), DefaultOptions())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	data := interp.NewRuntimeData(loaded, interp.DummyPolicy{},
		interp.EnvelopeStatic{From: "a@x", To: "b@x"},
		interp.MessageStatic{Size: len(eml), Header: hdr, RawMessage: []byte(eml)})
	if err := loaded.Execute(context.Background(), data); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return data.AppliedActions
}

func fileIntoMailboxes(actions []interp.AppliedAction) []string {
	var out []string
	for _, a := range actions {
		if fi, ok := a.(interp.ActionFileInto); ok {
			out = append(out, fi.Mailbox)
		}
	}
	return out
}

const mimeEML = "Content-Type: multipart/alternative; boundary=b\r\n" +
	"\r\n" +
	"--b\r\n" +
	"Content-Type: text/plain\r\n" +
	"\r\n" +
	"plain text\r\n" +
	"--b\r\n" +
	"Content-Type: text/html; charset=utf-8\r\n" +
	"\r\n" +
	"<p>html</p>\r\n" +
	"--b--\r\n"

func TestMime_SubtypeFileInto(t *testing.T) {
	script := `require ["foreverypart","mime","fileinto"];
foreverypart {
    if header :mime :subtype "Content-Type" "html" {
        fileinto "HTML";
    }
}`
	got := fileIntoMailboxes(runScript(t, script, mimeEML))
	if len(got) != 1 || got[0] != "HTML" {
		t.Errorf("fileinto = %v, want [HTML]", got)
	}
}

func TestMime_TypeAndParam(t *testing.T) {
	// text/html part matched by :type "text"; matches at least once.
	script := `require ["foreverypart","mime","fileinto"];
foreverypart {
    if header :mime :param "charset" :contains "Content-Type" "utf-8" {
        fileinto "UTF8";
    }
}`
	got := fileIntoMailboxes(runScript(t, script, mimeEML))
	if len(got) != 1 || got[0] != "UTF8" {
		t.Errorf("fileinto = %v, want [UTF8] (charset param match)", got)
	}
}

func TestMime_AnyChildOutsideLoop(t *testing.T) {
	// :mime :anychild outside foreverypart scans all parts.
	script := `require ["mime","fileinto"];
if header :mime :anychild :subtype "Content-Type" "html" {
    fileinto "HasHTML";
}`
	got := fileIntoMailboxes(runScript(t, script, mimeEML))
	if len(got) != 1 || got[0] != "HasHTML" {
		t.Errorf("fileinto = %v, want [HasHTML]", got)
	}
}

func TestMime_RequiresMime(t *testing.T) {
	// :mime without require "mime" is a compile error.
	script := `require ["fileinto"];
if header :mime "Content-Type" "text/plain" { fileinto "x"; }`
	if _, err := Load(bufio.NewReader(strings.NewReader(script)), DefaultOptions()); err == nil {
		t.Fatal("expected compile error for :mime without require \"mime\"")
	}
}
