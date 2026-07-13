package interp

// ImapSieveExtension is the RFC 6785 "imapsieve" capability. A script that runs
// in response to an IMAP event (message APPEND, COPY/MOVE, or flag change)
// requires it together with "environment" to read the imap.* items below.
const ImapSieveExtension = "imapsieve"

// RFC 6785 §4 environment items, resolved by the host's Env (GetEnvironment).
// The fork does not populate these — the IMAP server that triggers the script
// supplies them per event; the constants document the canonical names.
const (
	EnvImapCause        = "imap.cause"        // "APPEND", "COPY", or "FLAG"
	EnvImapMailbox      = "imap.mailbox"      // affected mailbox name
	EnvImapEmail        = "imap.email"        // envelope recipient (script owner)
	EnvImapUser         = "imap.user"         // authenticated user
	EnvImapChangedFlags = "imap.changedflags" // FLAG cause: the changed flags

	// Vendor items (yarilo namespace): COPY/MOVE source and destination mailbox.
	EnvVndMailboxFrom = "vnd.yarilo.mailbox-from"
	EnvVndMailboxTo   = "vnd.yarilo.mailbox-to"
)
