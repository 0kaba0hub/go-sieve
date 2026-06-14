package interp

import (
	"context"
	"fmt"
)

// ErrSieveError is returned when the Sieve `error` command (RFC 5463) executes.
type ErrSieveError struct {
	Message string
}

func (e *ErrSieveError) Error() string {
	return fmt.Sprintf("sieve: error: %s", e.Message)
}

// TestIhave implements the ihave test (RFC 5463).
// Returns true if all listed capabilities are supported by the interpreter.
type TestIhave struct {
	Capabilities []string
}

func (t TestIhave) Check(_ context.Context, _ *RuntimeData) (bool, error) {
	for _, cap := range t.Capabilities {
		if _, ok := supportedRequires[cap]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// CmdError implements the error command (RFC 5463).
// Terminates script execution with a runtime error.
type CmdError struct {
	Message string
}

func (c CmdError) Execute(_ context.Context, d *RuntimeData) error {
	return &ErrSieveError{Message: expandVars(d, c.Message)}
}
