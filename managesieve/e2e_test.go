package managesieve_test

import (
	"bytes"
	"net"
	"testing"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/go-sieve/managesieve"
	msclient "github.com/foxcpp/go-sieve/managesieve/client"
	mssasl "github.com/foxcpp/go-sieve/managesieve/sasl"
	mssrv "github.com/foxcpp/go-sieve/managesieve/server"
	"github.com/xdg-go/scram"
)

func TestClientServerRoundTrip(t *testing.T) {
	memOpts := mssrv.MemorySessionOptions{
		SieveExtensions: []string{"fileinto", "vacation"},
	}
	srv := mssrv.New(mssrv.BackendFunc(func(_ *mssrv.Conn, _ string) (mssrv.Storage, error) {
		return mssrv.NewMemorySession(memOpts), nil
	}), &mssrv.Options{
		Capabilities: func(*mssrv.Conn) []managesieve.CapabilityItem { return memOpts.Capabilities() },
		InsecureAuth: true,
	})
	srv.EnableAuth("PLAIN", mssrv.PlainAuth(func(identity, username, password string) (string, error) {
		if identity != "" && identity != username {
			return "", &managesieve.Error{Type: managesieve.StatusNO, Text: "invalid identity"}
		}
		if username != "alice" || password != "secret" {
			return "", &managesieve.Error{Type: managesieve.StatusNO, Text: "invalid credentials"}
		}
		return username, nil
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = srv.Close()
	})

	go func() {
		_ = srv.Serve(ln)
	}()

	c, err := msclient.Dial(ln.Addr().String(), &msclient.Options{AllowInsecureAuth: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	if !c.Caps().Set.Has(managesieve.CapImplementation) {
		t.Fatalf("missing IMPLEMENTATION capability")
	}

	if err := c.Authenticate(sasl.NewPlainClient("", "alice", "secret")); err != nil {
		t.Fatalf("Authenticate failed: %v", err)
	}
	if !c.Caps().Set.Has(managesieve.CapOwner) {
		t.Fatalf("missing OWNER capability after auth")
	}

	script := []byte("keep;")
	if err := c.PutScript("main", bytes.NewReader(script), int64(len(script))); err != nil {
		t.Fatalf("PutScript failed: %v", err)
	}

	list, err := c.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts failed: %v", err)
	}
	if len(list) != 1 || list[0].Name != "main" || list[0].Active {
		t.Fatalf("unexpected list before active: %#v", list)
	}

	if err := c.SetActive("main"); err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}

	list, err = c.ListScripts()
	if err != nil {
		t.Fatalf("ListScripts after active failed: %v", err)
	}
	if len(list) != 1 || !list[0].Active {
		t.Fatalf("unexpected list after active: %#v", list)
	}

	got, err := c.GetScript("main")
	if err != nil {
		t.Fatalf("GetScript failed: %v", err)
	}
	if string(got) != string(script) {
		t.Fatalf("wrong script body: got %q want %q", got, script)
	}

	if err := c.CheckScript(bytes.NewReader(script), int64(len(script))); err != nil {
		t.Fatalf("CheckScript failed: %v", err)
	}

	if err := c.RenameScript("main", "renamed"); err != nil {
		t.Fatalf("RenameScript failed: %v", err)
	}
	if err := c.SetActive(""); err != nil {
		t.Fatalf("SetActive(empty) failed: %v", err)
	}
	if err := c.DeleteScript("renamed"); err != nil {
		t.Fatalf("DeleteScript failed: %v", err)
	}

	if err := c.Logout(); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}
}

func TestSCRAMSHA1RoundTrip(t *testing.T) {
	cred, err := mssasl.CredentialsSCRAMSHA1("alice", "secret", scram.KeyFactors{Salt: "fixedsalt", Iters: 4096})
	if err != nil {
		t.Fatalf("CredentialsSCRAMSHA1 failed: %v", err)
	}

	memOpts := mssrv.MemorySessionOptions{SieveExtensions: []string{"fileinto", "vacation"}}
	srv := mssrv.New(mssrv.BackendFunc(func(_ *mssrv.Conn, _ string) (mssrv.Storage, error) {
		return mssrv.NewMemorySession(memOpts), nil
	}), &mssrv.Options{
		Capabilities: func(*mssrv.Conn) []managesieve.CapabilityItem { return memOpts.Capabilities() },
		InsecureAuth: true,
	})
	srv.EnableAuth(mssasl.MechSCRAMSHA1, mssrv.SCRAMSHA1Auth(func(username string) (scram.StoredCredentials, error) {
		if username != "alice" {
			return scram.StoredCredentials{}, &managesieve.Error{Type: managesieve.StatusNO, Text: "unknown user"}
		}
		return cred, nil
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = ln.Close()
		_ = srv.Close()
	})

	go func() {
		_ = srv.Serve(ln)
	}()

	c, err := msclient.Dial(ln.Addr().String(), &msclient.Options{AllowInsecureAuth: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })

	sc, err := mssasl.NewSCRAMSHA1Client("alice", "secret", "")
	if err != nil {
		t.Fatalf("NewSCRAMSHA1Client failed: %v", err)
	}
	if err := c.Authenticate(sc); err != nil {
		t.Fatalf("SCRAM Authenticate failed: %v", err)
	}
}
