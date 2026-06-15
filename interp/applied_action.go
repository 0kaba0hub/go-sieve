package interp

type AppliedAction interface {
	testActionName() string
	cancelsImplicitKeep() bool
}

type ActionKeep struct {
	Implicit bool
	Flags    Flags
}

func (ActionKeep) testActionName() string    { return "keep" }
func (ActionKeep) cancelsImplicitKeep() bool { return true }

type ActionDiscard struct{}

func (ActionDiscard) testActionName() string    { return "discard" }
func (ActionDiscard) cancelsImplicitKeep() bool { return true }

type ActionFileInto struct {
	Mailbox    string
	Flags      Flags
	Copy       bool
	Create     bool   // mailbox extension (RFC 5490)
	SpecialUse string // special-use extension (RFC 8579); "" = not used
}

func (ActionFileInto) testActionName() string            { return "fileinto" }
func (a ActionFileInto) cancelsImplicitKeep() bool { return !a.Copy }

type ActionRedirect struct {
	Address  string
	Copy     bool
	ListName string // non-empty when redirect :list was used (RFC 6134)
}

func (ActionRedirect) testActionName() string              { return "redirect" }
func (a ActionRedirect) cancelsImplicitKeep() bool { return !a.Copy }

type ActionReject struct {
	Reason string
}

func (ActionReject) testActionName() string    { return "reject" }
func (ActionReject) cancelsImplicitKeep() bool { return true }

type ActionEReject struct {
	Reason string
}

func (ActionEReject) testActionName() string    { return "ereject" }
func (ActionEReject) cancelsImplicitKeep() bool { return true }

// ActionNotify is emitted by the notify command (RFC 5435).
// The MTA is responsible for dispatching the notification.
type ActionNotify struct {
	Method     string
	From       string
	Importance string   // "1" high, "2" normal, "3" low
	Options    []string // method-specific key=value pairs
	Message    string
}

func (ActionNotify) testActionName() string    { return "notify" }
func (ActionNotify) cancelsImplicitKeep() bool { return false }
