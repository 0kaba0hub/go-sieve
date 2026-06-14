package interp

import (
	"fmt"

	"github.com/foxcpp/go-sieve/parser"
)

func loadIhaveTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("ihave") {
		return nil, fmt.Errorf("missing require 'ihave'")
	}
	var caps []string
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				NoVariables: true, // RFC 5463: capabilities MUST be constant strings
				MatchStr: func(val []string) {
					caps = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}
	return TestIhave{Capabilities: caps}, nil
}

func loadError(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("ihave") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'ihave'")
	}
	var msg string
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					msg = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}
	return CmdError{Message: msg}, nil
}
