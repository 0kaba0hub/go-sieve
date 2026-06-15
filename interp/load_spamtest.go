package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

func loadSpamTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("spamtest") && !s.RequiresExtension("spamtestplus") {
		return nil, parser.ErrorAt(test.Position, "missing require 'spamtest'")
	}

	t := SpamTest{matcherTest: newMatcherTest()}
	var key []string

	err := LoadSpec(s, t.matcherTest.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"percent": {
				MatchBool: func() { t.Percent = true },
			},
		},
		Pos: []SpecPosArg{
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

	if t.Percent && !s.RequiresExtension("spamtestplus") {
		return nil, parser.ErrorAt(test.Position, "spamtest :percent requires 'spamtestplus'")
	}

	if err := t.matcherTest.setKey(s, key); err != nil {
		return nil, err
	}

	return t, nil
}

func loadVirusTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("virustest") {
		return nil, parser.ErrorAt(test.Position, "missing require 'virustest'")
	}

	t := VirusTest{matcherTest: newMatcherTest()}
	var key []string

	err := LoadSpec(s, t.matcherTest.addSpecTags(&Spec{
		Pos: []SpecPosArg{
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
