package interp

import (
	"fmt"
	"strings"

	"github.com/foxcpp/go-sieve/parser"
)

func loadDateTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("date") {
		return nil, fmt.Errorf("missing require 'date'")
	}

	loaded := DateTest{matcherTest: newMatcherTest()}
	var key []string
	var zoneCnt int

	spec := loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"zone": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.Zone = val[0]
					zoneCnt++
				},
			},
			"originalzone": {
				MatchBool: func() {
					loaded.OriginalZone = true
					zoneCnt++
				},
			},
			"index": {
				NeedsValue: true,
				MatchNum: func(val int) {
					loaded.Index = val
				},
			},
			"last": {
				MatchBool: func() {
					loaded.Last = true
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.Header = val[0]
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.DatePart = DatePart(strings.ToLower(val[0]))
				},
			},
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					key = val
				},
			},
		},
	})

	if err := LoadSpec(s, spec, test.Position, test.Args, test.Tests, nil); err != nil {
		return nil, err
	}

	if zoneCnt > 1 {
		return nil, fmt.Errorf("date: cannot specify both :zone and :originalzone")
	}
	if loaded.Zone != "" {
		if _, err := parseZoneOffset(loaded.Zone); err != nil {
			return nil, fmt.Errorf("date: %v", err)
		}
	}
	if _, ok := validDateParts[loaded.DatePart]; !ok {
		return nil, fmt.Errorf("date: invalid date-part: %s", loaded.DatePart)
	}
	if loaded.Last && loaded.Index == 0 {
		return nil, fmt.Errorf("date: :last requires :index")
	}
	if loaded.Index > 0 && !s.RequiresExtension("index") {
		return nil, fmt.Errorf("date: missing require 'index' for :index argument")
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	return loaded, nil
}

func loadCurrentDateTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("date") {
		return nil, fmt.Errorf("missing require 'date'")
	}

	loaded := CurrentDateTest{matcherTest: newMatcherTest()}
	var key []string

	spec := loaded.addSpecTags(&Spec{
		Tags: map[string]SpecTag{
			"zone": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.Zone = val[0]
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					loaded.DatePart = DatePart(strings.ToLower(val[0]))
				},
			},
			{
				MinStrCount: 1,
				MatchStr: func(val []string) {
					key = val
				},
			},
		},
	})

	if err := LoadSpec(s, spec, test.Position, test.Args, test.Tests, nil); err != nil {
		return nil, err
	}

	if loaded.Zone != "" {
		if _, err := parseZoneOffset(loaded.Zone); err != nil {
			return nil, fmt.Errorf("currentdate: %v", err)
		}
	}
	if _, ok := validDateParts[loaded.DatePart]; !ok {
		return nil, fmt.Errorf("currentdate: invalid date-part: %s", loaded.DatePart)
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	return loaded, nil
}
