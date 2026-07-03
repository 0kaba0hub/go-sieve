package interp

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxcpp/go-sieve/parser"
)

const YariloPipeExtension = "vnd.yarilo.pipe"

// isValidProgramName enforces spec §3: no slash, no control chars, non-empty,
// max 128 Unicode code points.
func isValidProgramName(name string) bool {
	if name == "" {
		return false
	}
	runes := []rune(name)
	if len(runes) > 128 {
		return false
	}
	for _, r := range runes {
		if r == '/' {
			return false
		}
		if r <= 0x1F || r == 0x7F || (r >= 0x80 && r <= 0x9F) || r == 0x2028 || r == 0x2029 {
			return false
		}
	}
	return true
}

// isValidPipeArg rejects CR and LF per spec §4.
func isValidPipeArg(arg string) bool {
	return !strings.ContainsAny(arg, "\r\n")
}

type CmdPipe struct {
	ProgramName string
	Args        []string
	Copy        bool
	Try         bool
}

func (c CmdPipe) Execute(ctx context.Context, d *RuntimeData) error {
	if !d.Script.RequiresExtension(YariloPipeExtension) {
		return fmt.Errorf("require '%s' to use pipe", YariloPipeExtension)
	}

	programName := expandVars(d, c.ProgramName)

	if !isValidProgramName(programName) {
		if c.Try {
			return nil
		}
		return fmt.Errorf("pipe: invalid program name %q", programName)
	}

	args := make([]string, 0, len(c.Args))
	for _, a := range c.Args {
		expanded := expandVars(d, a)
		if !isValidPipeArg(expanded) {
			if c.Try {
				return nil
			}
			return fmt.Errorf("pipe: invalid argument for program %q", programName)
		}
		args = append(args, expanded)
	}

	if d.PipedPrograms == nil {
		d.PipedPrograms = make(map[string]struct{})
	}
	if _, already := d.PipedPrograms[programName]; already {
		if c.Try {
			return nil
		}
		return fmt.Errorf("pipe: duplicate pipe to program %q not allowed", programName)
	}
	d.PipedPrograms[programName] = struct{}{}

	action := ActionPipe{
		ProgramName: programName,
		Args:        args,
		Copy:        c.Copy,
		Try:         c.Try,
	}

	if err := d.OnAction(ctx, action, d); err != nil {
		if c.Try {
			delete(d.PipedPrograms, programName)
			return nil
		}
		return err
	}

	if !c.Copy {
		d.ImplicitKeep = false
	}
	return nil
}

func loadPipe(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension(YariloPipeExtension) {
		return nil, parser.ErrorAt(pcmd.Position, "pipe requires '%s'", YariloPipeExtension)
	}

	cmd := CmdPipe{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"copy": {
				MatchBool: func() { cmd.Copy = true },
			},
			"try": {
				MatchBool: func() { cmd.Try = true },
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					cmd.ProgramName = val[0]
				},
			},
			{
				Optional:    true,
				MinStrCount: 0,
				MaxStrCount: -1,
				MatchStr: func(val []string) {
					cmd.Args = val
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if !isValidProgramName(cmd.ProgramName) {
		return nil, parser.ErrorAt(pcmd.Position, "pipe: invalid program name %q", cmd.ProgramName)
	}

	if len(cmd.Args) > 16 {
		return nil, parser.ErrorAt(pcmd.Position, "pipe: too many arguments (max 16)")
	}
	for _, a := range cmd.Args {
		if len(a) > 1024 {
			return nil, parser.ErrorAt(pcmd.Position, "pipe: argument too long (max 1024 bytes)")
		}
		if !usedVarsAreValid(s, a) {
			return nil, parser.ErrorAt(pcmd.Position, "pipe: invalid variable in argument: %v", a)
		}
	}

	if cmd.Copy && !s.RequiresExtension("copy") {
		return nil, parser.ErrorAt(pcmd.Position, "pipe :copy requires 'copy' extension")
	}

	return cmd, nil
}

