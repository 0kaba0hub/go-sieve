package interp

import (
	"context"
)

// SpecialUseChecker is an optional interface for PolicyReader (RFC 8579).
// If Policy does not implement this, specialuse_exists always returns false
// and fileinto :specialuse falls back to the named mailbox.
type SpecialUseChecker interface {
	// SpecialUseMailbox returns the first mailbox in the user's personal namespace
	// that carries the given special-use attribute (e.g. `\Junk`).
	// Returns ("", false) if no such mailbox exists.
	SpecialUseMailbox(ctx context.Context, attr string) (mailbox string, ok bool)

	// MailboxHasSpecialUse returns true if the named mailbox carries the given
	// special-use attribute and the user has delivery access to it.
	MailboxHasSpecialUse(ctx context.Context, mailbox, attr string) (bool, error)
}

// TestSpecialUseExists implements the specialuse_exists test (RFC 8579 §3).
type TestSpecialUseExists struct {
	Mailbox string   // optional; "" = any mailbox
	Attrs   []string // special-use attributes to check
}

func (t TestSpecialUseExists) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(SpecialUseChecker)
	if !ok {
		return false, nil
	}

	if t.Mailbox == "" {
		for _, attr := range t.Attrs {
			_, found := checker.SpecialUseMailbox(ctx, expandVars(d, attr))
			if !found {
				return false, nil
			}
		}
		return true, nil
	}

	mailbox := expandVars(d, t.Mailbox)
	for _, attr := range t.Attrs {
		has, err := checker.MailboxHasSpecialUse(ctx, mailbox, expandVars(d, attr))
		if err != nil {
			return false, err
		}
		if !has {
			return false, nil
		}
	}
	return true, nil
}
