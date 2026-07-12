package sieve

import (
	"bufio"
	"context"
	"net/textproto"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/interp"
)

func TestExtractText_ToVariable(t *testing.T) {
	eml := "Content-Type: multipart/mixed; boundary=b\r\n\r\n" +
		"--b\r\n" +
		"Content-Type: text/plain\r\n\r\n" +
		"Hello World Body\r\n" +
		"--b--\r\n"
	script := `require ["foreverypart","mime","extracttext","variables","fileinto"];
foreverypart {
    if header :mime :type "Content-Type" "text" {
        extracttext :first 5 "snippet";
        fileinto "${snippet}";
    }
}`
	hdr, _ := textproto.NewReader(bufio.NewReader(strings.NewReader(eml))).ReadMIMEHeader()
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
	got := fileIntoMailboxes(data.AppliedActions)
	// :first 5 of "Hello World Body" → "Hello"
	if len(got) != 1 || got[0] != "Hello" {
		t.Errorf("fileinto = %v, want [Hello] (extracttext :first 5)", got)
	}
}

func TestExtractText_RequiresExtracttext(t *testing.T) {
	script := `require ["variables"];
extracttext "v";`
	if _, err := Load(bufio.NewReader(strings.NewReader(script)), DefaultOptions()); err == nil {
		t.Fatal("expected error: extracttext without require")
	}
}
