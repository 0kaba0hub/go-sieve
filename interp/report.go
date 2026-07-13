package interp

import (
	"context"
	"fmt"
	"strings"

	"github.com/foxcpp/go-sieve/parser"
)

const YariloReportExtension = "vnd.yarilo.report"

// isValidFeedbackType enforces a non-empty MIME token for the ARF feedback-type
// (RFC 5965 §7.1 registry values such as "abuse" / "fraud" / "not-spam"): no
// whitespace, controls, or MIME tspecials.
func isValidFeedbackType(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r <= 0x20 || r == 0x7F {
			return false
		}
		if strings.ContainsRune("()<>@,;:\\\"/[]?=", r) {
			return false
		}
	}
	return true
}

// CmdReport implements the report command (vnd.yarilo.report): it emits an ARF
// (RFC 5965) abuse report about the current message to a target address.
type CmdReport struct {
	FeedbackType string
	Message      string
	Target       string
	HeadersOnly  bool
}

func (c CmdReport) Execute(ctx context.Context, d *RuntimeData) error {
	if !d.Script.RequiresExtension(YariloReportExtension) {
		return fmt.Errorf("require '%s' to use report", YariloReportExtension)
	}

	feedbackType := expandVars(d, c.FeedbackType)
	if !isValidFeedbackType(feedbackType) {
		return fmt.Errorf("report: invalid feedback-type %q", feedbackType)
	}

	// report is a side-effect action; it does not cancel implicit keep.
	if err := d.OnAction(ctx, ActionReport{
		FeedbackType: feedbackType,
		Message:      expandVars(d, c.Message),
		Target:       expandVars(d, c.Target),
		HeadersOnly:  c.HeadersOnly,
	}, d); err != nil {
		return err
	}
	return nil
}

func loadReport(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension(YariloReportExtension) {
		return nil, parser.ErrorAt(pcmd.Position, "report requires '%s'", YariloReportExtension)
	}

	cmd := CmdReport{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"headers_only": {
				MatchBool: func() { cmd.HeadersOnly = true },
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr:    func(val []string) { cmd.FeedbackType = val[0] },
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr:    func(val []string) { cmd.Message = val[0] },
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr:    func(val []string) { cmd.Target = val[0] },
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}
