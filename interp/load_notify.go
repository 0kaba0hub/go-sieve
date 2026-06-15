package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

func loadNotify(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("enotify") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'enotify'")
	}

	cmd := CmdNotify{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"from": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.From = val[0]
				},
			},
			"importance": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					cmd.Importance = val[0]
				},
			},
			"options": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Options = val
				},
			},
			"message": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Message = val[0]
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Method = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	switch cmd.Importance {
	case "", "1", "2", "3":
	default:
		return nil, parser.ErrorAt(pcmd.Position, "notify: :importance must be \"1\", \"2\", or \"3\"")
	}

	return cmd, nil
}

func loadValidNotifyMethod(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("enotify") {
		return nil, parser.ErrorAt(test.Position, "missing require 'enotify'")
	}

	t := TestValidNotifyMethod{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					t.URIs = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func loadNotifyMethodCapability(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("enotify") {
		return nil, parser.ErrorAt(test.Position, "missing require 'enotify'")
	}

	t := TestNotifyMethodCapability{matcherTest: newMatcherTest()}
	var key []string
	err := LoadSpec(s, t.matcherTest.addSpecTags(&Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					t.URI = val[0]
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					t.Capability = val[0]
				},
			},
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					key = val
				},
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if err := t.matcherTest.setKey(s, key); err != nil {
		return nil, err
	}

	return t, nil
}
