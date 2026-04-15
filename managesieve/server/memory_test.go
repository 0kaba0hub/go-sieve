package server

import (
	"context"
	"testing"

	"github.com/foxcpp/go-sieve/managesieve"
)

func TestMemorySessionNameValidationAndCodes(t *testing.T) {
	s := NewMemorySession(MemorySessionOptions{
		SieveExtensions: []string{"fileinto"},
	})

	ctx := context.Background()

	if err := s.PutScript(ctx, "", []byte("keep;")); err == nil {
		t.Fatalf("expected empty name to fail")
	}

	if err := s.PutScript(ctx, "ok", []byte("keep;")); err != nil {
		t.Fatalf("PutScript failed: %v", err)
	}
	if err := s.SetActive(ctx, "ok"); err != nil {
		t.Fatalf("SetActive failed: %v", err)
	}

	err := s.DeleteScript(ctx, "ok")
	if err == nil {
		t.Fatalf("expected deleting active script to fail")
	}
	msErr, ok := err.(*managesieve.Error)
	if !ok {
		t.Fatalf("expected managesieve.Error, got %T", err)
	}
	if msErr.Code != managesieve.ResponseCodeActive {
		t.Fatalf("wrong code: got %q want %q", msErr.Code, managesieve.ResponseCodeActive)
	}
}

func TestMemorySessionQuotaCodes(t *testing.T) {
	s := NewMemorySession(MemorySessionOptions{
		SieveExtensions: []string{"fileinto"},
		MaxScripts:      1,
		MaxScriptLen:    6,
	})

	ctx := context.Background()
	if err := s.PutScript(ctx, "a", []byte("keep;")); err != nil {
		t.Fatalf("first PutScript failed: %v", err)
	}

	err := s.PutScript(ctx, "b", []byte("keep;"))
	if err == nil {
		t.Fatalf("expected max scripts failure")
	}
	msErr, ok := err.(*managesieve.Error)
	if !ok {
		t.Fatalf("expected managesieve.Error, got %T", err)
	}
	if msErr.Code != managesieve.ResponseCode("QUOTA/MAXSCRIPTS") {
		t.Fatalf("wrong code for max scripts: %q", msErr.Code)
	}

	err = s.HaveSpace(ctx, "x", 99)
	if err == nil {
		t.Fatalf("expected max size failure")
	}
	msErr, ok = err.(*managesieve.Error)
	if !ok {
		t.Fatalf("expected managesieve.Error, got %T", err)
	}
	if msErr.Code != managesieve.ResponseCode("QUOTA/MAXSIZE") {
		t.Fatalf("wrong code for max size: %q", msErr.Code)
	}
}
