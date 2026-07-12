package interp

import (
	"fmt"
	"strings"

	"github.com/foxcpp/go-sieve/parser"
)

func loadSet(script *Script, pcmd parser.Cmd) (Cmd, error) {
	if !script.RequiresExtension("variables") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'variables'")
	}
	cmd := CmdSet{}

	modifiers := map[int]func(string) string{}
	var conflictingMods bool

	err := LoadSpec(script, &Spec{
		Tags: valueModifierTags(modifiers, &conflictingMods),
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Name = strings.ToLower(val[0])
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

	if conflictingMods {
		return nil, parser.ErrorAt(pcmd.Position, "conflicting value modifiers")
	}

	if modifiers[15] != nil && !script.RequiresExtension("enotify") {
		return nil, parser.ErrorAt(pcmd.Position, ":encodeurl requires 'enotify'")
	}

	settable, _ := script.IsVarUsable(cmd.Name)
	if !settable {
		return nil, parser.ErrorAt(pcmd.Position, "cannot set this variable")
	}

	cmd.ModifyValue = buildValueModifier(script, modifiers)

	return cmd, err
}

func loadStringTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("variables") {
		return nil, fmt.Errorf("missing require 'variables'")
	}

	loaded := TestString{matcherTest: newMatcherTest()}
	var key []string
	err := LoadSpec(s, loaded.addSpecTags(&Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Source = val
				},
				MinStrCount: 1,
			},
			{
				MatchStr: func(val []string) {
					key = val
				},
				MinStrCount: 1,
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	return loaded, nil
}
