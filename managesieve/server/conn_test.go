package server

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/foxcpp/go-sieve/managesieve"
)

func startTestServer(t *testing.T) (addr string, closeFn func()) {
	t.Helper()
	memOpts := MemorySessionOptions{}
	srv := New(BackendFunc(func(_ *Conn, _ string) (Storage, error) {
		return NewMemorySession(memOpts), nil
	}), &Options{
		Capabilities: func(*Conn) []managesieve.CapabilityItem { return memOpts.Capabilities() },
		InsecureAuth: true,
	})
	srv.EnableAuth("PLAIN", PlainAuth(func(identity, username, password string) (string, error) {
		return username, nil
	}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	return ln.Addr().String(), func() {
		_ = ln.Close()
		_ = srv.Close()
	}
}

func TestCommandArityValidation(t *testing.T) {
	addr, closeFn := startTestServer(t)
	defer closeFn()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			break
		}
	}

	if _, err := w.WriteString("CAPABILITY \"x\"\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "NO") {
		t.Fatalf("expected NO for invalid CAPABILITY args, got %q", line)
	}
}

func TestHugeLiteralRejected(t *testing.T) {
	addr, closeFn := startTestServer(t)
	defer closeFn()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			break
		}
	}

	if _, err := w.WriteString("AUTHENTICATE \"PLAIN\" \"AGFsaWNlAHNlY3JldA==\"\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "OK") {
			break
		}
	}

	if _, err := w.WriteString("PUTSCRIPT \"x\" {9999999999+}\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "NO") {
		t.Fatalf("expected NO for huge literal, got %q", line)
	}
}

func TestAuthenticateReturnsCapabilityBlockThenCompletion(t *testing.T) {
	addr, closeFn := startTestServer(t)
	defer closeFn()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			break
		}
	}

	if _, err := w.WriteString("AUTHENTICATE \"PLAIN\" \"AGFsaWNlAHNlY3JldA==\"\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Flush(); err != nil {
		t.Fatal(err)
	}

	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "\"IMPLEMENTATION\"") {
		t.Fatalf("expected auth response to start with capability block, got %q", line)
	}

	line, err = r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "\"SIEVE\"") && !strings.HasPrefix(line, "\"VERSION\"") && !strings.HasPrefix(line, "\"SASL\"") && !strings.HasPrefix(line, "\"OWNER\"") {
		t.Fatalf("expected more capabilities after auth, got %q", line)
	}

	for {
		line, err = r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "OK") {
			break
		}
	}
	if !strings.HasPrefix(line, "OK") {
		t.Fatalf("expected final OK after auth capabilities, got %q", line)
	}
}

func TestGreetingFiltersUnusablePlain(t *testing.T) {
	memOpts := MemorySessionOptions{}
	srv := New(BackendFunc(func(_ *Conn, _ string) (Storage, error) {
		return NewMemorySession(memOpts), nil
	}), &Options{
		Capabilities: func(*Conn) []managesieve.CapabilityItem { return memOpts.Capabilities() },
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ln.Close()
		_ = srv.Close()
	}()
	go func() { _ = srv.Serve(ln) }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "\"SASL\"") && strings.Contains(line, "PLAIN") {
			t.Fatalf("unexpected PLAIN advertisement without TLS/override: %q", line)
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			break
		}
	}
}

func TestAuthTimeoutClosesIdleConnection(t *testing.T) {
	memOpts := MemorySessionOptions{}
	srv := New(BackendFunc(func(_ *Conn, _ string) (Storage, error) {
		return NewMemorySession(memOpts), nil
	}), &Options{
		Capabilities: func(*Conn) []managesieve.CapabilityItem { return memOpts.Capabilities() },
		AuthTimeout:  50 * time.Millisecond,
		IdleTimeout:  50 * time.Millisecond,
	})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = ln.Close()
		_ = srv.Close()
	}()
	go func() { _ = srv.Serve(ln) }()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(line, "OK") || strings.HasPrefix(line, "NO") || strings.HasPrefix(line, "BYE") {
			break
		}
	}

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(line, "BYE") {
		t.Fatalf("expected timeout BYE, got %q", line)
	}
}
