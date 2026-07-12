package interp

import (
	"testing"
)

func TestLoadMailboxID(t *testing.T) {
	s := &Script{extensions: supportedRequires}

	// fileinto :mailboxid parses the tag and keeps the positional fallback.
	testCmdLoader(t, s,
		`require ["fileinto", "mailboxid"];
fileinto :mailboxid "F1234" "Fallback";`,
		[]Cmd{CmdFileInto{Mailbox: "Fallback", MailboxID: "F1234"}})

	// mailboxidexists test with a list of ids.
	testCmdLoader(t, s,
		`require ["fileinto", "mailboxid"];
if mailboxidexists ["A", "B"] { keep; }`,
		[]Cmd{CmdIf{
			Test:  TestMailboxIDExists{IDs: []string{"A", "B"}},
			Block: []Cmd{CmdKeep{}},
		}})
}
