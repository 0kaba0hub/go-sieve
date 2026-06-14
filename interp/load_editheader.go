package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

func loadAddHeader(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("editheader") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'editheader'")
	}

	cmd := CmdAddHeader{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"last": {
				MatchBool: func() {
					cmd.Last = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.FieldName = val[0]
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Value = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func loadDeleteHeader(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("editheader") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'editheader'")
	}

	cmd := CmdDeleteHeader{matcherTest: newMatcherTest()}

	spec := cmd.matcherTest.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"index": {
				NeedsValue: true,
				MatchNum: func(val int) {
					cmd.Index = val
				},
			},
			"last": {
				MatchBool: func() {
					cmd.Last = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.FieldName = val[0]
				},
			},
			{
				Optional:    true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.ValuePatterns = val
				},
			},
		},
	})

	if err := LoadSpec(s, spec, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block); err != nil {
		return nil, err
	}

	if cmd.Last && cmd.Index == 0 {
		return nil, parser.ErrorAt(pcmd.Position, "deleteheader: :last requires :index")
	}

	if len(cmd.ValuePatterns) > 0 {
		if err := cmd.matcherTest.setKey(s, cmd.ValuePatterns); err != nil {
			return nil, parser.ErrorAt(pcmd.Position, "deleteheader: %v", err)
		}
	}

	return cmd, nil
}
