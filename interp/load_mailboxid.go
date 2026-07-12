package interp

import (
	"fmt"

	"github.com/foxcpp/go-sieve/parser"
)

func loadMailboxIDExists(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("mailboxid") {
		return nil, fmt.Errorf("missing require 'mailboxid'")
	}

	t := TestMailboxIDExists{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					t.IDs = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	return t, nil
}
