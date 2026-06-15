package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

func loadMetadataTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("mboxmetadata") {
		return nil, parser.ErrorAt(test.Position, "missing require 'mboxmetadata'")
	}

	t := TestMetadata{matcherTest: newMatcherTest()}
	var key []string

	err := LoadSpec(s, t.matcherTest.addSpecTags(&Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					t.Mailbox = val[0]
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					t.Annotation = val[0]
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

func loadMetadataExistsTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("mboxmetadata") {
		return nil, parser.ErrorAt(test.Position, "missing require 'mboxmetadata'")
	}

	t := TestMetadataExists{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					t.Mailbox = val[0]
				},
			},
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					t.Annotations = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func loadServerMetadataTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("servermetadata") {
		return nil, parser.ErrorAt(test.Position, "missing require 'servermetadata'")
	}

	t := TestServerMetadata{matcherTest: newMatcherTest()}
	var key []string

	err := LoadSpec(s, t.matcherTest.addSpecTags(&Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					t.Annotation = val[0]
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

func loadServerMetadataExistsTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("servermetadata") {
		return nil, parser.ErrorAt(test.Position, "missing require 'servermetadata'")
	}

	t := TestServerMetadataExists{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					t.Annotations = val
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	return t, nil
}
