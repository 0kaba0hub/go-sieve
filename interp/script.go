package interp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/lexer"
)

type Cmd interface {
	Execute(ctx context.Context, d *RuntimeData) error
}

type Options struct {
	MaxRedirects int

	// MaxActions caps the total number of actions a script may apply
	// (fileinto, redirect, keep, ...). Zero means unlimited. Enforced by
	// DefaultOnAction as actions accumulate, so execution aborts as soon as
	// the limit is exceeded.
	MaxActions int

	MaxVariableCount   int
	MaxVariableNameLen int
	MaxVariableLen     int

	// SubAddressSep is the set of separator characters for subaddress parsing
	// (RFC 5233). Each character in the string is treated as a separator.
	// Defaults to "+" if empty.
	SubAddressSep string

	// If specified - enables vnd.dovecot.testsuite extension
	// and will execute tests.
	T             *testing.T
	DisabledTests []string

	// DebugLog is called by the debug_log command (vnd.yarilo.debug extension).
	// When nil the command is a no-op.
	DebugLog func(msg string)
}

type Script struct {
	extensions map[string]struct{}
	cmd        []Cmd

	// ihave is set when require "ihave" is declared (RFC 5463).
	// All parse-time extension guards are bypassed so scripts can use any
	// extension inside an ihave-guarded block without listing it in require.
	ihave bool

	opts *Options
}

var ErrStop = errors.New("interpreter: stop called")

func (s Script) Extensions() []string {
	exts := make([]string, 0, len(s.extensions))
	for ext := range s.extensions {
		exts = append(exts, ext)
	}
	return exts
}

func (s Script) RequiresExtension(name string) bool {
	if s.ihave {
		return true
	}
	_, ok := s.extensions[name]
	return ok
}

func (s Script) IsVarUsable(variableName string) (settable, gettable bool) {
	if len(variableName) > s.opts.MaxVariableNameLen {
		return false, false
	}

	namespace, name, ok := strings.Cut(strings.ToLower(variableName), ".")
	if !ok {
		name = namespace
		namespace = ""
	}

	switch namespace {
	case "env":
		if !s.RequiresExtension(YariloEnvironmentExtension) {
			return false, false
		}
		return false, true
	case "envelope":
		if !s.RequiresExtension("envelope") {
			return false, false
		}
		if !lexer.IsValidIdentifier(name) {
			return false, false
		}
		return false, true
	case "global":
		if !s.RequiresExtension("include") {
			return false, false
		}
		if !lexer.IsValidIdentifier(name) {
			return false, false
		}
		return true, true
	case "":
		if !lexer.IsValidIdentifier(name) {
			return false, false
		}
		return true, true
	default:
		return false, false
	}
}

func (s Script) Execute(ctx context.Context, d *RuntimeData) error {
	for _, c := range s.cmd {
		if err := c.Execute(ctx, d); err != nil {
			// ErrStop and ErrReturn both terminate the top-level script
			// (RFC 5228 §3.3; RFC 6609 §3.2).
			if errors.Is(err, ErrStop) || errors.Is(err, ErrReturn) {
				return nil
			}
			return err
		}
	}

	implicitKeep := d.ImplicitKeep
	for _, act := range d.AppliedActions {
		if act.cancelsImplicitKeep() {
			implicitKeep = false
		}
	}

	if implicitKeep {
		if err := d.OnAction(ctx, ActionKeep{
			Implicit: true,
			Flags:    d.Flags,
		}, d); err != nil {
			return err
		}
	}

	return nil
}

// executeIncluded runs the script as an included script (RFC 6609).
// ErrReturn exits only this script; ErrStop propagates to stop all execution.
func (s Script) executeIncluded(ctx context.Context, d *RuntimeData) error {
	for _, c := range s.cmd {
		if err := c.Execute(ctx, d); err != nil {
			if errors.Is(err, ErrReturn) {
				return nil
			}
			return err
		}
	}
	return nil
}
