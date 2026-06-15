package interp

import (
	"fmt"

	"github.com/foxcpp/go-sieve/lexer"
	"github.com/foxcpp/go-sieve/parser"
)

func loadInclude(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("include") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'include'")
	}

	cmd := CmdInclude{Location: "personal"}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"personal": {
				MatchBool: func() { cmd.Location = "personal" },
			},
			"global": {
				MatchBool: func() { cmd.Location = "global" },
			},
			"once": {
				MatchBool: func() { cmd.Once = true },
			},
			"optional": {
				MatchBool: func() { cmd.Optional = true },
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true, // RFC 6609: script name MUST be a constant string
				MatchStr: func(val []string) {
					cmd.Name = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if !isValidIncludeScriptName(cmd.Name) {
		return nil, parser.ErrorAt(pcmd.Position, "include: invalid script name %q", cmd.Name)
	}

	return cmd, nil
}

// isValidIncludeScriptName rejects names that would allow path traversal.
// ManageSieve script names are arbitrary strings, but we forbid "/" and
// sequences like ".." to prevent the ScriptLoader from being tricked into
// loading unintended files.
func isValidIncludeScriptName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '/' || r == '\\' || r == 0 {
			return false
		}
	}
	// Reject bare "." and ".." as well as sequences like "../".
	for _, part := range splitPath(name) {
		if part == "." || part == ".." {
			return false
		}
	}
	return true
}

func splitPath(s string) []string {
	var parts []string
	cur := ""
	for _, r := range s {
		if r == '/' || r == '\\' {
			parts = append(parts, cur)
			cur = ""
		} else {
			cur += string(r)
		}
	}
	parts = append(parts, cur)
	return parts
}

func loadReturn(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("include") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'include'")
	}
	err := LoadSpec(s, &Spec{}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}
	return CmdReturn{}, nil
}

func loadGlobal(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("include") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'include'")
	}
	if !s.RequiresExtension("variables") {
		return nil, parser.ErrorAt(pcmd.Position, "global: requires both 'include' and 'variables'")
	}

	cmd := CmdGlobal{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				NoVariables: true, // RFC 6609: names must be constant strings
				MatchStr: func(val []string) {
					cmd.Names = val
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	for _, name := range cmd.Names {
		if !lexer.IsValidIdentifier(name) {
			return nil, fmt.Errorf("global: invalid variable name %q (must be a valid identifier)", name)
		}
	}

	return cmd, nil
}
