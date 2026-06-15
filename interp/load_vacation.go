package interp

import (
	"github.com/foxcpp/go-sieve/parser"
)

// loadVacation loads the vacation command as defined in RFC 5230 and RFC 6131.
func loadVacation(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("vacation") && !s.RequiresExtension("vacation-seconds") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'vacation'")
	}

	cmd := CmdVacation{}
	hasDays, hasSeconds := false, false
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"days": {
				NeedsValue: true,
				MatchNum: func(val int) {
					cmd.Days = val
					hasDays = true
				},
			},
			"seconds": {
				NeedsValue: true,
				MatchNum: func(val int) {
					cmd.Seconds = val
					hasSeconds = true
				},
			},
			"subject": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Subject = val[0]
				},
			},
			"from": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.From = val[0]
				},
			},
			"addresses": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Addresses = val
				},
			},
			"mime": {
				NeedsValue: false,
				MatchBool: func() {
					cmd.Mime = true
				},
			},
			"handle": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Handle = val[0]
				},
			},
			"fcc": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Fcc = val[0]
				},
			},
			"flags": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr: func(val []string) {
					cmd.FccFlags = canonicalFlags(val, nil, nil)
				},
			},
			"create": {
				MatchBool: func() {
					cmd.FccCreate = true
				},
			},
			"specialuse": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.FccSpecialUse = val[0]
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Reason = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if hasDays && hasSeconds {
		return nil, parser.ErrorAt(pcmd.Position, "vacation: :days and :seconds are mutually exclusive")
	}
	if hasSeconds && !s.RequiresExtension("vacation-seconds") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'vacation-seconds'")
	}
	if cmd.Fcc != "" && !s.RequiresExtension("fcc") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'fcc'")
	}
	if cmd.FccFlags != nil && !s.RequiresExtension("imap4flags") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'imap4flags'")
	}
	if cmd.FccCreate && !s.RequiresExtension("mailbox") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'mailbox'")
	}
	if cmd.FccSpecialUse != "" && !s.RequiresExtension("special-use") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'special-use'")
	}

	return cmd, nil
}
