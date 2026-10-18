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

// TestIhave implements the ihave test (RFC 5463). Have is decided at load:
// every listed capability is supported and allowed by Options.Allowed.
type TestIhave struct {
	Capabilities []string
	Have         bool
}

func (t TestIhave) Check(_ context.Context, _ *RuntimeData) (bool, error) {
	return t.Have, nil
}

// CmdError implements the error command (RFC 5463).
// Terminates script execution with a runtime error.
type CmdError struct {
	Message string
}

func (c CmdError) Execute(_ context.Context, d *RuntimeData) error {
	return &ErrSieveError{Message: expandVars(d, c.Message)}
}
