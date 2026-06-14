package interp

import (
	"context"
)

// MailboxChecker is an optional extension of PolicyReader for mailboxexists.
// If Policy does not implement this interface, mailboxexists always returns true.
type MailboxChecker interface {
	MailboxExists(ctx context.Context, mailbox string) (bool, error)
}

// MailboxExistsTest implements the RFC 5490 mailboxexists test.
type MailboxExistsTest struct {
	Mailboxes []string
}

func (m MailboxExistsTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(MailboxChecker)
	if !ok {
		return true, nil
	}
	for _, mailbox := range m.Mailboxes {
		mailbox = expandVars(d, mailbox)
		exists, err := checker.MailboxExists(ctx, mailbox)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}
