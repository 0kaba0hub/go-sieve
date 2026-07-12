package sieve

import (
	"bufio"
	"context"
	"net/textproto"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/interp"
)

func runData(t *testing.T, script, eml string) *interp.RuntimeData {
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
	return data
}

func TestReplace_WholeMessage(t *testing.T) {
	eml := "From: a@x\r\nTo: b@x\r\nSubject: hi\r\nContent-Type: text/plain\r\n\r\noriginal\r\n"
	d := runData(t, `require ["replace"];
replace "NEW BODY";`, eml)
	out := string(d.FilteredMessage)
	if !strings.Contains(out, "NEW BODY") {
		t.Errorf("filtered message missing replacement:\n%s", out)
	}
	if !strings.Contains(out, "From: a@x") {
		t.Errorf("whole-message replace should keep From header:\n%s", out)
	}
	if !strings.Contains(out, "text/plain") {
		t.Errorf("replace without :mime should be text/plain:\n%s", out)
	}
}

func TestReplace_SubjectFrom(t *testing.T) {
	eml := "From: a@x\r\nSubject: original subj\r\nContent-Type: text/plain\r\n\r\nbody\r\n"
	d := runData(t, `require ["replace"];
replace :subject "new subj" "b";`, eml)
	out := string(d.FilteredMessage)
	if !strings.Contains(out, "Subject: new subj") {
		t.Errorf(":subject not applied:\n%s", out)
	}
	if !strings.Contains(out, "Original-Subject: original subj") {
		t.Errorf("original subject not preserved:\n%s", out)
	}
}

func TestReplace_PartInLoop(t *testing.T) {
	eml := "Content-Type: multipart/alternative; boundary=b\r\n\r\n" +
		"--b\r\nContent-Type: text/plain\r\n\r\nkeep me\r\n" +
		"--b\r\nContent-Type: text/html\r\n\r\n<p>secret</p>\r\n--b--\r\n"
	d := runData(t, `require ["foreverypart","mime","replace"];
foreverypart {
    if header :mime :subtype "Content-Type" "html" {
        replace "REDACTED";
    }
}`, eml)
	out := string(d.FilteredMessage)
	if strings.Contains(out, "secret") {
		t.Errorf("html part not replaced:\n%s", out)
	}
	if !strings.Contains(out, "REDACTED") || !strings.Contains(out, "keep me") {
		t.Errorf("expected REDACTED + kept text part:\n%s", out)
	}
}

func TestEnclose(t *testing.T) {
	eml := "From: a@x\r\nSubject: orig\r\nContent-Type: text/plain\r\n\r\ninner body\r\n"
	d := runData(t, `require ["enclose"];
enclose :subject "Wrapped" "cover note";`, eml)
	out := string(d.FilteredMessage)
	if !strings.Contains(out, "multipart/mixed") {
		t.Errorf("enclose should produce multipart/mixed:\n%s", out)
	}
	if !strings.Contains(out, "message/rfc822") {
		t.Errorf("enclose should wrap original as message/rfc822:\n%s", out)
	}
	if !strings.Contains(out, "cover note") || !strings.Contains(out, "inner body") {
		t.Errorf("enclose should carry cover + original:\n%s", out)
	}
	if !strings.Contains(out, "Subject: Wrapped") {
		t.Errorf(":subject not set on wrapper:\n%s", out)
	}
}
