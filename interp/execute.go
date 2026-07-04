package interp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/foxcpp/go-sieve/lexer"
	"github.com/foxcpp/go-sieve/parser"
)

const YariloExecuteExtension = "vnd.yarilo.execute"

// ExecuteExecutor runs an external program for the vnd.yarilo.execute command/test.
//
// input is the data to write to the program's stdin:
//   - nil   → empty stdin (no :input / :pipe tag)
//   - bytes from :input <string> or :pipe (current message)
//
// Returns:
//   - output: stdout bytes (always captured; caller decides whether to store)
//   - ok:     true when exit code is 0 (success / test-true)
//   - err:    non-nil on infrastructure failure (program not found, I/O error)
type ExecuteExecutor interface {
	Execute(ctx context.Context, programName string, args []string, input io.Reader) (output []byte, ok bool, err error)
}

// executeInput describes the stdin source for the execute command.
type executeInput int

const (
	executeInputNone    executeInput = iota // no :input / :pipe — empty stdin
	executeInputString                      // :input <string>
	executeInputMessage                     // :pipe — current message
)

// CmdExecute is the execute command (standalone statement).
type CmdExecute struct {
	ProgramName string
	Args        []string
	Input       executeInput
	InputData   string // set when Input == executeInputString
	OutputVar   string // set when :output <varname> is given; "" = discard
}

func (c CmdExecute) Execute(ctx context.Context, d *RuntimeData) error {
	if !d.Script.RequiresExtension(YariloExecuteExtension) {
		return fmt.Errorf("require '%s' to use execute", YariloExecuteExtension)
	}
	programName := expandVars(d, c.ProgramName)
	if !isValidProgramName(programName) {
		return fmt.Errorf("execute: invalid program name %q", programName)
	}
	args := expandExecuteArgs(d, c.Args)
	_, ok, err := runExecute(ctx, d, programName, args, c.Input, c.InputData, c.OutputVar)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("execute: program %q returned non-zero exit", programName)
	}
	return nil
}

// TestExecute is the execute test (used inside if/elsif conditions).
// Returns true when the program exits with code 0, false otherwise.
type TestExecute struct {
	ProgramName string
	Args        []string
	Input       executeInput
	InputData   string
	OutputVar   string
}

func (t TestExecute) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	if !d.Script.RequiresExtension(YariloExecuteExtension) {
		return false, fmt.Errorf("require '%s' to use execute", YariloExecuteExtension)
	}
	programName := expandVars(d, t.ProgramName)
	if !isValidProgramName(programName) {
		return false, fmt.Errorf("execute: invalid program name %q", programName)
	}
	args := expandExecuteArgs(d, t.Args)
	_, ok, err := runExecute(ctx, d, programName, args, t.Input, t.InputData, t.OutputVar)
	if err != nil {
		return false, err
	}
	return ok, nil
}

// runExecute calls ExecuteExecutor, optionally stores output in a Sieve variable,
// and returns the raw output bytes and ok flag.
func runExecute(ctx context.Context, d *RuntimeData, programName string, args []string, inputMode executeInput, inputData string, outputVar string) ([]byte, bool, error) {
	if d.ExecuteExecutor == nil {
		return nil, false, fmt.Errorf("execute: no ExecuteExecutor configured")
	}

	var stdin io.Reader
	switch inputMode {
	case executeInputString:
		stdin = strings.NewReader(expandVars(d, inputData))
	case executeInputMessage:
		stdin = bytes.NewReader(rawMessageBytes(d.Msg))
	default:
		stdin = nil
	}

	out, ok, err := d.ExecuteExecutor.Execute(ctx, programName, args, stdin)
	if err != nil {
		return nil, false, fmt.Errorf("execute: program %q failed: %w", programName, err)
	}

	if outputVar != "" && len(out) > 0 {
		varName := expandVars(d, outputVar)
		if setErr := d.SetVar(varName, string(out)); setErr != nil {
			return out, ok, fmt.Errorf("execute: set output variable %q: %w", varName, setErr)
		}
	}

	return out, ok, nil
}

func expandExecuteArgs(d *RuntimeData, args []string) []string {
	expanded := make([]string, len(args))
	for i, a := range args {
		expanded[i] = expandVars(d, a)
	}
	return expanded
}

// loadExecuteCommon parses tags and positional arguments shared by CmdExecute / TestExecute.
func loadExecuteCommon(s *Script, pname *string, args *[]string, input *executeInput, inputData *string, outputVar *string, pos lexer.Position, pargs []parser.Arg, tests []parser.Test, block []parser.Cmd) error {
	spec := &Spec{
		Tags: map[string]SpecTag{
			"pipe": {
				MatchBool: func() { *input = executeInputMessage },
			},
			"input": {
				NeedsValue: true,
				MatchStr: func(val []string) {
					*input = executeInputString
					*inputData = val[0]
				},
				MinStrCount: 1,
				MaxStrCount: 1,
			},
			"output": {
				NeedsValue: true,
				MatchStr: func(val []string) {
					*outputVar = val[0]
				},
				MinStrCount: 1,
				MaxStrCount: 1,
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					*pname = val[0]
				},
			},
			{
				Optional:    true,
				MinStrCount: 0,
				MaxStrCount: -1,
				MatchStr: func(val []string) {
					*args = val
				},
			},
		},
	}
	if err := LoadSpec(s, spec, pos, pargs, tests, block); err != nil {
		return err
	}
	if !isValidProgramName(*pname) {
		return parser.ErrorAt(pos, "execute: invalid program name %q", *pname)
	}
	if len(*args) > 16 {
		return parser.ErrorAt(pos, "execute: too many arguments (max 16)")
	}
	for _, a := range *args {
		if len(a) > 1024 {
			return parser.ErrorAt(pos, "execute: argument too long (max 1024 bytes)")
		}
		if !usedVarsAreValid(s, a) {
			return parser.ErrorAt(pos, "execute: invalid variable in argument: %v", a)
		}
	}
	if *outputVar != "" && !s.RequiresExtension("variables") {
		return parser.ErrorAt(pos, "execute: :output requires 'variables' extension")
	}
	return nil
}

func loadExecuteCmd(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension(YariloExecuteExtension) {
		return nil, parser.ErrorAt(pcmd.Position, "execute requires '%s'", YariloExecuteExtension)
	}
	cmd := CmdExecute{}
	if err := loadExecuteCommon(s, &cmd.ProgramName, &cmd.Args, &cmd.Input, &cmd.InputData, &cmd.OutputVar, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block); err != nil {
		return nil, err
	}
	return cmd, nil
}

func loadExecuteTest(s *Script, ptest parser.Test) (Test, error) {
	if !s.RequiresExtension(YariloExecuteExtension) {
		return nil, parser.ErrorAt(ptest.Position, "execute requires '%s'", YariloExecuteExtension)
	}
	t := TestExecute{}
	if err := loadExecuteCommon(s, &t.ProgramName, &t.Args, &t.Input, &t.InputData, &t.OutputVar, ptest.Position, ptest.Args, nil, nil); err != nil {
		return nil, err
	}
	return t, nil
}
