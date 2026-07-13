package interp

import (
	"testing"
)

func TestLoadReport(t *testing.T) {
	s := &Script{extensions: supportedRequires}

	testCmdLoader(t, s,
		`require ["vnd.yarilo.report", "environment"];
report "abuse" "user marked as spam" "abuse@example.com";`,
		[]Cmd{CmdReport{
			FeedbackType: "abuse",
			Message:      "user marked as spam",
			Target:       "abuse@example.com",
		}})

	testCmdLoader(t, s,
		`require ["vnd.yarilo.report", "environment"];
report :headers_only "not-spam" "false positive" "fbl@example.com";`,
		[]Cmd{CmdReport{
			FeedbackType: "not-spam",
			Message:      "false positive",
			Target:       "fbl@example.com",
			HeadersOnly:  true,
		}})
}

func TestIsValidFeedbackType(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"abuse", true},
		{"not-spam", true},
		{"fraud", true},
		{"", false},
		{"has space", false},
		{"semi;colon", false},
		{"at@sign", false},
	}
	for _, tc := range tests {
		if got := isValidFeedbackType(tc.in); got != tc.want {
			t.Errorf("isValidFeedbackType(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
