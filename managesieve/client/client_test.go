package client

import (
	"bufio"
	"bytes"
	"testing"
)

func TestWriteSimpleCommandLockedKeepsStringArgumentsQuoted(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{bw: bufio.NewWriter(&buf)}

	if err := c.writeSimpleCommandLocked("GETSCRIPT", []commandArg{strArg("123")}); err != nil {
		t.Fatalf("writeSimpleCommandLocked failed: %v", err)
	}

	if got, want := buf.String(), "GETSCRIPT \"123\"\r\n"; got != want {
		t.Fatalf("unexpected command encoding: got %q want %q", got, want)
	}
}

func TestWriteSimpleCommandLockedWritesNumbersAsAtoms(t *testing.T) {
	var buf bytes.Buffer
	c := &Client{bw: bufio.NewWriter(&buf)}

	if err := c.writeSimpleCommandLocked("HAVESPACE", []commandArg{strArg("main"), numArg(42)}); err != nil {
		t.Fatalf("writeSimpleCommandLocked failed: %v", err)
	}

	if got, want := buf.String(), "HAVESPACE \"main\" 42\r\n"; got != want {
		t.Fatalf("unexpected command encoding: got %q want %q", got, want)
	}
}
