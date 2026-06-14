package interp

import (
	"fmt"

	"github.com/foxcpp/go-sieve/parser"
)

func loadSpecialUseExists(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("special-use") {
		return nil, fmt.Errorf("missing require 'special-use'")
	}

	t := TestSpecialUseExists{}
	var firstArg, secondArg []string

	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					firstArg = val
				},
			},
			{
				Optional:    true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					secondArg = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if len(secondArg) > 0 {
		// specialuse_exists <mailbox> <attrs>
		if len(firstArg) != 1 {
			return nil, parser.ErrorAt(test.Position, "specialuse_exists: mailbox must be a single string")
		}
		t.Mailbox = firstArg[0]
		t.Attrs = secondArg
	} else {
		// specialuse_exists <attrs>
		t.Attrs = firstArg
	}

	return t, nil
}
