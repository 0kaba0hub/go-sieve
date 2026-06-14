package interp

import (
	"fmt"
	"strconv"

	"github.com/foxcpp/go-sieve/parser"
)

func loadDuplicateTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("duplicate") {
		return nil, fmt.Errorf("missing require 'duplicate'")
	}

	t := &TestDuplicate{
		Seconds: duplicateDefaultSeconds,
	}

	var hasHeader, hasUniqueID bool

	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"handle": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					t.Handle = val[0]
				},
			},
			"header": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					hasHeader = true
					t.idKind = idSourceHeader
					t.idParam = val[0]
				},
			},
			"uniqueid": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					hasUniqueID = true
					t.idKind = idSourceUniqueID
					t.idParam = val[0]
				},
			},
			"seconds": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					n, err := strconv.ParseUint(val[0], 10, 32)
					if err == nil {
						t.Seconds = uint32(n)
					}
				},
			},
			"last": {
				MatchBool: func() {
					t.Last = true
				},
			},
		},
	}, test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if hasHeader && hasUniqueID {
		return nil, parser.ErrorAt(test.Position, "duplicate: :header and :uniqueid are mutually exclusive")
	}

	return t, nil
}
