package interp

import (
	"context"
	"fmt"
	"strings"
)

// NotificationMethod provides access to notification infrastructure (RFC 5435).
// Implementations attach it to d.Policy.
type NotificationMethod interface {
	// NotifyMethodValid returns true if the notification method URI is supported
	// and syntactically/semantically valid.
	NotifyMethodValid(ctx context.Context, method string) bool
	// NotifyMethodCapability returns the value of a named capability for the given
	// method URI. Returns ("", false) if the capability is unknown or unavailable.
	NotifyMethodCapability(ctx context.Context, method, capability string) (string, bool)
}

// CmdNotify implements the notify command (RFC 5435).
type CmdNotify struct {
	Method     string
	From       string
	Importance string // "1" = high, "2" = normal (default), "3" = low
	Options    []string
	Message    string
}

func (c CmdNotify) Execute(ctx context.Context, d *RuntimeData) error {
	method := expandVars(d, c.Method)
	if method == "" {
		return fmt.Errorf("notify: method must not be empty")
	}

	importance := c.Importance
	if importance == "" {
		importance = "2"
	}

	opts := expandVarsList(d, c.Options)

	if err := d.OnAction(ctx, ActionNotify{
		Method:     method,
		From:       expandVars(d, c.From),
		Importance: importance,
		Options:    opts,
		Message:    expandVars(d, c.Message),
	}, d); err != nil {
		return err
	}
	return nil
}

// TestValidNotifyMethod implements the valid_notify_method test (RFC 5435 §5).
type TestValidNotifyMethod struct {
	URIs []string
}

func (t TestValidNotifyMethod) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	nm, ok := d.Policy.(NotificationMethod)
	if !ok {
		return false, nil
	}
	for _, uri := range t.URIs {
		uri = expandVars(d, uri)
		if !nm.NotifyMethodValid(ctx, uri) {
			return false, nil
		}
	}
	return true, nil
}

// TestNotifyMethodCapability implements the notify_method_capability test (RFC 5435 §5).
type TestNotifyMethodCapability struct {
	matcherTest
	URI        string
	Capability string
}

func (t TestNotifyMethodCapability) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	nm, ok := d.Policy.(NotificationMethod)
	if !ok {
		return false, nil
	}

	uri := expandVars(d, t.URI)
	cap := strings.ToLower(expandVars(d, t.Capability))

	val, ok := nm.NotifyMethodCapability(ctx, uri, cap)
	if !ok {
		return false, nil
	}

	return t.matcherTest.tryMatch(d, val)
}

// percentEncode encodes a string per RFC 3986 §2.1, encoding all octets not in
// the unreserved set (ALPHA / DIGIT / "-" / "." / "_" / "~").
// Used by the :encodeurl modifier (RFC 5435 §6).
func percentEncode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, c := range []byte(s) {
		if isURIUnreserved(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isURIUnreserved(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '.' || c == '_' || c == '~'
}
