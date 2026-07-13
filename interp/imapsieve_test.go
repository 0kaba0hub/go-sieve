package interp

import "testing"

func TestImapSieveRequireAccepted(t *testing.T) {
	s := &Script{extensions: supportedRequires}

	// require "imapsieve" must be accepted; scripts read imap.* items through
	// the environment test, resolved by the host Env at run time.
	testCmdLoader(t, s,
		`require ["imapsieve", "environment", "fileinto"];
keep;`,
		[]Cmd{CmdKeep{}})
}
