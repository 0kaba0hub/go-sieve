package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

func loadValidExtList(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("extlists") {
		return nil, parser.ErrorAt(test.Position, "missing require 'extlists'")
	}

	t := TestValidExtList{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					t.Names = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}
	return t, nil
}
