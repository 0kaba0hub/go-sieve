package interp

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/foxcpp/go-sieve/lexer"
	"github.com/foxcpp/go-sieve/parser"
)

const YariloFilterExtension = "vnd.yarilo.filter"

// FilterExecutor runs an external program for the vnd.yarilo.filter command/test.
// Returns a non-nil reader when the program produced output (message substitution),
// nil reader for pass-through (empty output / non-zero exit), or an error on failure.
type FilterExecutor interface {
	Filter(ctx context.Context, programName string, args []string, msg io.Reader) (io.Reader, error)
}

// ActionFilter is recorded in AppliedActions by the filter command.
// The host engine reads FilteredMessage from RuntimeData after Execute returns.
type ActionFilter struct {
	ProgramName string
	Args        []string
}

func (ActionFilter) testActionName() string    { return "filter" }
func (ActionFilter) cancelsImplicitKeep() bool { return false }

// CmdFilter is the filter command (standalone statement).
type CmdFilter struct {
	ProgramName string
	Args        []string
}

func (c CmdFilter) Execute(ctx context.Context, d *RuntimeData) error {
	if !d.Script.RequiresExtension(YariloFilterExtension) {
		return fmt.Errorf("require '%s' to use filter", YariloFilterExtension)
	}
	programName := expandVars(d, c.ProgramName)
	if !isValidProgramName(programName) {
		return fmt.Errorf("filter: invalid program name %q", programName)
	}
	args := expandFilterArgs(d, c.Args)
	if err := runFilter(ctx, d, programName, args); err != nil {
		return err
	}
	return d.OnAction(ctx, ActionFilter{ProgramName: programName, Args: args}, d)
}

// rawMessageBytes extracts raw message bytes from the Message if it is a MessageStatic.
func rawMessageBytes(m Message) []byte {
	if ms, ok := m.(MessageStatic); ok {
		return ms.RawMessage
	}
	return nil
}

// TestFilter is the filter test (used inside if/elsif/elsif conditions).
// Returns true when the program produced output, false for pass-through.
type TestFilter struct {
	ProgramName string
	Args        []string
}

func (t TestFilter) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	if !d.Script.RequiresExtension(YariloFilterExtension) {
		return false, fmt.Errorf("require '%s' to use filter", YariloFilterExtension)
	}
	programName := expandVars(d, t.ProgramName)
	if !isValidProgramName(programName) {
		return false, fmt.Errorf("filter: invalid program name %q", programName)
	}
	args := expandFilterArgs(d, t.Args)
	if err := runFilter(ctx, d, programName, args); err != nil {
		return false, err
	}
	matched := d.FilteredMessage != nil
	if err := d.OnAction(ctx, ActionFilter{ProgramName: programName, Args: args}, d); err != nil {
		return false, err
	}
	return matched, nil
}

// runFilter calls FilterExecutor and stores the result in d.FilteredMessage.
// nil output = pass-through; d.FilteredMessage is cleared in that case.
func runFilter(ctx context.Context, d *RuntimeData, programName string, args []string) error {
	if d.FilterExecutor == nil {
		return fmt.Errorf("filter: no FilterExecutor configured")
	}
	msg := rawMessageBytes(d.Msg)
	out, err := d.FilterExecutor.Filter(ctx, programName, args, bytes.NewReader(msg))
	if err != nil {
		return fmt.Errorf("filter: program %q failed: %w", programName, err)
	}
	if out == nil {
		d.FilteredMessage = nil
		return nil
	}
	data, err := io.ReadAll(out)
	if err != nil {
		return fmt.Errorf("filter: read program output: %w", err)
	}
	if len(data) == 0 {
		d.FilteredMessage = nil
	} else {
		d.FilteredMessage = data
	}
	return nil
}

func expandFilterArgs(d *RuntimeData, args []string) []string {
	expanded := make([]string, len(args))
	for i, a := range args {
		expanded[i] = expandVars(d, a)
	}
	return expanded
}

func loadFilterCommon(s *Script, pname *string, args *[]string, pos lexer.Position, pargs []parser.Arg, tests []parser.Test, block []parser.Cmd) error {
	spec := &Spec{
		Tags: map[string]SpecTag{},
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
		return parser.ErrorAt(pos, "filter: invalid program name %q", *pname)
	}
	if len(*args) > 16 {
		return parser.ErrorAt(pos, "filter: too many arguments (max 16)")
	}
	for _, a := range *args {
		if len(a) > 1024 {
			return parser.ErrorAt(pos, "filter: argument too long (max 1024 bytes)")
		}
		if !usedVarsAreValid(s, a) {
			return parser.ErrorAt(pos, "filter: invalid variable in argument: %v", a)
		}
	}
	return nil
}

func loadFilterCmd(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension(YariloFilterExtension) {
		return nil, parser.ErrorAt(pcmd.Position, "filter requires '%s'", YariloFilterExtension)
	}
	cmd := CmdFilter{}
	if err := loadFilterCommon(s, &cmd.ProgramName, &cmd.Args, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block); err != nil {
		return nil, err
	}
	return cmd, nil
}

func loadFilterTest(s *Script, ptest parser.Test) (Test, error) {
	if !s.RequiresExtension(YariloFilterExtension) {
		return nil, parser.ErrorAt(ptest.Position, "filter requires '%s'", YariloFilterExtension)
	}
	t := TestFilter{}
	if err := loadFilterCommon(s, &t.ProgramName, &t.Args, ptest.Position, ptest.Args, nil, nil); err != nil {
		return nil, err
	}
	return t, nil
}
