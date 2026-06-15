package interp

import (
	"context"
	"encoding/gob"
)

// SpamVirusChecker provides spam and virus test results for the current message
// (RFC 5235). Implementations attach it to d.Policy.
type SpamVirusChecker interface {
	// SpamScore returns the normalized spam score for the current message.
	// When percent is false the scale is 0–10; when true it is 0–100.
	// tested is false when the message was not checked or the result is
	// indeterminate — the returned value must be "0" in that case.
	SpamScore(ctx context.Context, percent bool) (value string, tested bool)
	// VirusScore returns the normalized virus score (0–5 scale).
	// tested is false when the message was not checked or indeterminate.
	VirusScore(ctx context.Context) (value string, tested bool)
}

// SpamTest implements the spamtest test (RFC 5235).
type SpamTest struct {
	matcherTest
	Percent bool // :percent tag — requires spamtestplus
}

func (t SpamTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	var (
		value  string
		tested bool
	)
	if checker, ok := d.Policy.(SpamVirusChecker); ok {
		value, tested = checker.SpamScore(ctx, t.Percent)
	} else {
		value, tested = "0", false
	}

	if t.isCount() {
		cnt := uint64(0)
		if tested {
			cnt = 1
		}
		return t.countMatches(d, cnt), nil
	}

	return t.tryMatch(d, value)
}

// VirusTest implements the virustest test (RFC 5235).
type VirusTest struct {
	matcherTest
}

func (t VirusTest) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	var (
		value  string
		tested bool
	)
	if checker, ok := d.Policy.(SpamVirusChecker); ok {
		value, tested = checker.VirusScore(ctx)
	} else {
		value, tested = "0", false
	}

	if t.isCount() {
		cnt := uint64(0)
		if tested {
			cnt = 1
		}
		return t.countMatches(d, cnt), nil
	}

	return t.tryMatch(d, value)
}

func init() {
	gob.Register(SpamTest{})
	gob.Register(VirusTest{})
}
