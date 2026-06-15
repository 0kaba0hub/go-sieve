package interp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/foxcpp/go-sieve/lexer"
	"github.com/foxcpp/go-sieve/parser"
)

// ScriptLoader loads Sieve scripts by name for the include extension (RFC 6609).
// Implementations attach this to d.Policy by having the Policy object also implement
// ScriptLoader, or by embedding it directly.
type ScriptLoader interface {
	// LoadPersonalScript opens a script from the user's personal repository.
	// A nil reader with nil error means the script does not exist.
	LoadPersonalScript(ctx context.Context, name string) (io.ReadCloser, error)
	// LoadGlobalScript opens a script from the site-wide repository.
	LoadGlobalScript(ctx context.Context, name string) (io.ReadCloser, error)
}

// ErrReturn signals that a Sieve "return" command was executed.
// CmdInclude catches it to exit only the included script; when it reaches
// Script.Execute it is treated identically to ErrStop.
var ErrReturn = errors.New("sieve: return")

// IncludeState holds shared execution state for the include extension.
// One instance is created on first use and shared across all included scripts
// in the same top-level Execute call.
type IncludeState struct {
	stack []string       // script keys currently on the call stack
	once  map[string]bool // scripts already included via :once
}

func newIncludeState() *IncludeState {
	return &IncludeState{once: map[string]bool{}}
}

func (is *IncludeState) isPending(key string) bool {
	for _, k := range is.stack {
		if k == key {
			return true
		}
	}
	return false
}

func (is *IncludeState) push(key string) { is.stack = append(is.stack, key) }
func (is *IncludeState) pop()            { is.stack = is.stack[:len(is.stack)-1] }

// includeMaxDepth is the maximum nesting level (RFC requires minimum 3 including
// the top-level script; we allow one extra for headroom).
const includeMaxDepth = 4

// CmdInclude implements the "include" command (RFC 6609).
type CmdInclude struct {
	Location string // "personal" (default) or "global"
	Once     bool
	Optional bool
	Name     string
}

func (c CmdInclude) Execute(ctx context.Context, d *RuntimeData) error {
	loader, ok := d.Policy.(ScriptLoader)
	if !ok {
		return fmt.Errorf("include: Policy does not implement ScriptLoader")
	}

	if d.IncludeState == nil {
		d.IncludeState = newIncludeState()
	}
	state := d.IncludeState
	key := c.Location + ":" + c.Name

	// :once — skip silently if already included.
	if c.Once && state.once[key] {
		return nil
	}
	// Recursion detection — :once silently prevents re-entry.
	if state.isPending(key) {
		if c.Once {
			state.once[key] = true
			return nil
		}
		return fmt.Errorf("include: recursive inclusion of %q", c.Name)
	}
	if len(state.stack) >= includeMaxDepth {
		return fmt.Errorf("include: maximum nesting depth (%d) exceeded", includeMaxDepth)
	}

	// Load script source.
	var (
		r   io.ReadCloser
		err error
	)
	switch c.Location {
	case "personal":
		r, err = loader.LoadPersonalScript(ctx, c.Name)
	case "global":
		r, err = loader.LoadGlobalScript(ctx, c.Name)
	default:
		return fmt.Errorf("include: unknown location %q", c.Location)
	}
	if err != nil || r == nil {
		if c.Optional {
			return nil
		}
		if err != nil {
			return fmt.Errorf("include %q: %w", c.Name, err)
		}
		return fmt.Errorf("include %q: script not found", c.Name)
	}
	defer r.Close()

	src, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("include %q: read: %w", c.Name, err)
	}

	toks, err := lexer.Lex(bytes.NewReader(src), &lexer.Options{Filename: c.Name})
	if err != nil {
		return fmt.Errorf("include %q: lex: %w", c.Name, err)
	}
	cmds, err := parser.Parse(lexer.NewStream(toks), &parser.Options{})
	if err != nil {
		return fmt.Errorf("include %q: parse: %w", c.Name, err)
	}
	included, err := LoadScript(cmds, d.Script.opts)
	if err != nil {
		return fmt.Errorf("include %q: load: %w", c.Name, err)
	}

	// Execute with a fresh local scope; share all mutable global state.
	savedScript := d.Script
	savedVars := d.Variables
	savedGlobalDecls := d.globalVarDecls
	savedMatchVars := d.MatchVariables

	d.Script = included
	d.Variables = map[string]string{}
	d.globalVarDecls = nil
	d.MatchVariables = nil

	state.push(key)
	execErr := included.executeIncluded(ctx, d)
	state.pop()
	if c.Once {
		state.once[key] = true
	}

	// Restore per-script state.
	d.Script = savedScript
	d.Variables = savedVars
	d.globalVarDecls = savedGlobalDecls
	d.MatchVariables = savedMatchVars

	return execErr
}

// CmdReturn implements the "return" command (RFC 6609).
type CmdReturn struct{}

func (CmdReturn) Execute(_ context.Context, _ *RuntimeData) error {
	return ErrReturn
}

// CmdGlobal implements the "global" command (RFC 6609).
// It declares variable names as belonging to the global namespace for the
// duration of the current script execution.
type CmdGlobal struct {
	Names []string
}

func (c CmdGlobal) Execute(_ context.Context, d *RuntimeData) error {
	if d.globalVarDecls == nil {
		d.globalVarDecls = map[string]bool{}
	}
	if d.GlobalVars == nil {
		d.GlobalVars = map[string]string{}
	}
	for _, name := range c.Names {
		if strings.Contains(name, ".") {
			return fmt.Errorf("global: variable name must not contain namespace prefix: %q", name)
		}
		d.globalVarDecls[strings.ToLower(name)] = true
	}
	return nil
}
