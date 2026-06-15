package interp

import (
	"context"
	"encoding/gob"
)

// MetadataChecker provides access to IMAP METADATA annotations (RFC 5490 §4).
// Implementations attach it to d.Policy.
type MetadataChecker interface {
	// GetMailboxMetadata retrieves the value of an annotation on a mailbox.
	// Returns ("", false, nil) when the annotation does not exist (NIL value).
	GetMailboxMetadata(ctx context.Context, mailbox, annotation string) (value string, exists bool, err error)
	// GetServerMetadata retrieves the value of a server-level annotation.
	// Returns ("", false, nil) when the annotation does not exist (NIL value).
	GetServerMetadata(ctx context.Context, annotation string) (value string, exists bool, err error)
}

// TestMetadata implements the metadata test (RFC 5490 §3.3).
type TestMetadata struct {
	matcherTest
	Mailbox    string
	Annotation string
}

func (t TestMetadata) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(MetadataChecker)
	if !ok {
		return false, nil
	}

	mailbox := expandVars(d, t.Mailbox)
	annotation := expandVars(d, t.Annotation)

	value, exists, err := checker.GetMailboxMetadata(ctx, mailbox, annotation)
	if err != nil {
		return false, err
	}

	if t.isCount() {
		cnt := uint64(0)
		if exists {
			cnt = 1
		}
		return t.countMatches(d, cnt), nil
	}

	if !exists {
		return false, nil
	}
	return t.tryMatch(d, value)
}

// TestMetadataExists implements the metadataexists test (RFC 5490 §3.4).
type TestMetadataExists struct {
	Mailbox     string
	Annotations []string
}

func (t TestMetadataExists) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(MetadataChecker)
	if !ok {
		return false, nil
	}

	mailbox := expandVars(d, t.Mailbox)
	for _, ann := range t.Annotations {
		ann = expandVars(d, ann)
		_, exists, err := checker.GetMailboxMetadata(ctx, mailbox, ann)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

// TestServerMetadata implements the servermetadata test (RFC 5490 §4.1).
type TestServerMetadata struct {
	matcherTest
	Annotation string
}

func (t TestServerMetadata) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(MetadataChecker)
	if !ok {
		return false, nil
	}

	annotation := expandVars(d, t.Annotation)

	value, exists, err := checker.GetServerMetadata(ctx, annotation)
	if err != nil {
		return false, err
	}

	if t.isCount() {
		cnt := uint64(0)
		if exists {
			cnt = 1
		}
		return t.countMatches(d, cnt), nil
	}

	if !exists {
		return false, nil
	}
	return t.tryMatch(d, value)
}

// TestServerMetadataExists implements the servermetadataexists test (RFC 5490 §4.2).
type TestServerMetadataExists struct {
	Annotations []string
}

func (t TestServerMetadataExists) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(MetadataChecker)
	if !ok {
		return false, nil
	}

	for _, ann := range t.Annotations {
		ann = expandVars(d, ann)
		_, exists, err := checker.GetServerMetadata(ctx, ann)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func init() {
	gob.Register(TestMetadata{})
	gob.Register(TestMetadataExists{})
	gob.Register(TestServerMetadata{})
	gob.Register(TestServerMetadataExists{})
}
