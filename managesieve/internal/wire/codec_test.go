package wire

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/foxcpp/go-sieve/managesieve"
)

func TestStringLiteralRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(bufio.NewWriter(&buf))
	enc.StringServer("line1\nline2")
	if err := enc.CRLF(); err != nil {
		t.Fatalf("CRLF failed: %v", err)
	}

	dec := NewDecoder(bufio.NewReader(&buf))
	v, err := dec.ReadString()
	if err != nil {
		t.Fatalf("ReadString failed: %v", err)
	}
	if got, want := v, "line1\nline2"; got != want {
		t.Fatalf("string mismatch: got %q want %q", got, want)
	}
	if err := dec.ExpectCRLF(); err != nil {
		t.Fatalf("ExpectCRLF failed: %v", err)
	}
}

func TestReadStatusLine(t *testing.T) {
	raw := "NO (NONEXISTENT) \"missing\"\r\n"
	dec := NewDecoder(bufio.NewReader(bytes.NewBufferString(raw)))
	resp, err := dec.ReadStatusLine()
	if err != nil {
		t.Fatalf("ReadStatusLine failed: %v", err)
	}
	if resp.Type != managesieve.StatusNO {
		t.Fatalf("wrong status type: %v", resp.Type)
	}
	if resp.Code != managesieve.ResponseCodeNonexistent {
		t.Fatalf("wrong response code: %v", resp.Code)
	}
	if resp.Text != "missing" {
		t.Fatalf("wrong response text: %q", resp.Text)
	}
}

func TestReadCapabilityOrStatus(t *testing.T) {
	raw := "\"SIEVE\" \"fileinto vacation\"\r\nOK\r\n"
	dec := NewDecoder(bufio.NewReader(bytes.NewBufferString(raw)))

	item, status, err := dec.ReadCapabilityOrStatus()
	if err != nil {
		t.Fatalf("ReadCapabilityOrStatus failed: %v", err)
	}
	if status != nil {
		t.Fatalf("expected capability item, got status: %#v", status)
	}
	if item == nil || item.Name != "SIEVE" {
		t.Fatalf("wrong capability item: %#v", item)
	}
	if item.Value == nil || *item.Value != "fileinto vacation" {
		t.Fatalf("wrong capability value: %#v", item)
	}

	item, status, err = dec.ReadCapabilityOrStatus()
	if err != nil {
		t.Fatalf("ReadCapabilityOrStatus status failed: %v", err)
	}
	if item != nil {
		t.Fatalf("expected status, got item: %#v", item)
	}
	if status == nil || status.Type != managesieve.StatusOK {
		t.Fatalf("wrong status: %#v", status)
	}
}

func TestReadQuotedLimit(t *testing.T) {
	dec := NewDecoder(bufio.NewReader(bytes.NewBufferString("\"abcd\"")))
	dec.MaxQuotedBytes = 3
	if _, err := dec.ReadString(); err == nil {
		t.Fatalf("expected quoted string limit error")
	}
}

func TestReadAtomLimit(t *testing.T) {
	dec := NewDecoder(bufio.NewReader(bytes.NewBufferString("abcdef")))
	dec.MaxAtomBytes = 3
	if _, err := dec.ReadAtom(); err == nil {
		t.Fatalf("expected atom limit error")
	}
}

func TestReadCommandLimits(t *testing.T) {
	dec := NewDecoder(bufio.NewReader(bytes.NewBufferString("NOOP \"a\" \"b\"\r\n")))
	dec.MaxCommandArgs = 1
	if _, _, err := dec.ReadCommand(); err == nil {
		t.Fatalf("expected command arg limit error")
	}

	dec = NewDecoder(bufio.NewReader(bytes.NewBufferString("NOOP \"abcd\"\r\n")))
	dec.MaxNonLiteralCommandLen = 5
	if _, _, err := dec.ReadCommand(); err == nil {
		t.Fatalf("expected command byte limit error")
	}
}
