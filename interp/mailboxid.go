package interp

import (
	"context"
)

// MailboxIDChecker is an optional interface for PolicyReader (RFC 9042).
// If Policy does not implement this, mailboxidexists always returns false
// and fileinto :mailboxid falls back to the named mailbox.
type MailboxIDChecker interface {
	// MailboxByID returns the name of the mailbox carrying the given
	// server-assigned MAILBOXID (RFC 8474 objectid) in the user's personal
	// namespace. Returns ("", false) if no mailbox has that id or the user
	// has no delivery access to it.
	MailboxByID(ctx context.Context, id string) (mailbox string, ok bool)
}

// TestMailboxIDExists implements the mailboxidexists test (RFC 9042 §4):
// true iff every listed MAILBOXID resolves to a mailbox the user can access.
type TestMailboxIDExists struct {
	IDs []string
}

func (t TestMailboxIDExists) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(MailboxIDChecker)
	if !ok {
		return false, nil
	}
	for _, id := range t.IDs {
		if _, found := checker.MailboxByID(ctx, expandVars(d, id)); !found {
			return false, nil
		}
	}
	return true, nil
}
