package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/interp"
)

// TestVacation runs the vacation action through the script API and reads the
// response the runtime records for the envelope sender.
func TestVacation(t *testing.T) {
	cases := []struct {
		name   string
		script string
		from   string
		want   *interp.VacationResponse // nil: no response recorded
	}{
		{
			name:   "basic",
			script: `require ["vacation"]; vacation "I'm on vacation.";`,
			from:   "sender@example.org",
			want:   &interp.VacationResponse{Subject: "Automated reply", Body: "I'm on vacation.", Days: 7, Handle: "default"},
		},
		{
			name: "with parameters",
			script: `require ["vacation"];
			vacation :days 14 :subject "Out of Office" :from "me@example.com"
				:addresses ["me@example.com", "me2@example.com"]
				:mime :handle "vacation-001"
				"I'm on vacation until next week.";`,
			from: "sender@example.org",
			want: &interp.VacationResponse{From: "me@example.com", Subject: "Out of Office", Body: "I'm on vacation until next week.", IsMime: true, Days: 14, Handle: "vacation-001"},
		},
		{
			name:   "no response to own address",
			script: `require ["vacation"]; vacation :addresses ["me@example.com"] "I'm on vacation.";`,
			from:   "me@example.com",
			want:   nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := sieve.DefaultOptions()
			opts.Lexer.Filename = "inline"
			parsed, err := sieve.Load(strings.NewReader(tc.script), opts)
			if err != nil {
				t.Fatal(err)
			}
			env := interp.EnvelopeStatic{From: tc.from, To: "recipient@example.com"}
			data := sieve.NewRuntimeData(parsed, interp.DummyPolicy{}, env, interp.MessageStatic{})
			if err := parsed.Execute(context.Background(), data); err != nil {
				t.Fatal(err)
			}
			got, ok := data.VacationResponses[tc.from]
			if tc.want == nil {
				if ok {
					t.Fatalf("a response was recorded for the script's own address: %+v", got)
				}
				return
			}
			if !ok {
				t.Fatalf("no response recorded for %s; responses: %+v", tc.from, data.VacationResponses)
			}
			got.FccFlags = nil // not set by any case; the field is not comparable
			if fmt.Sprintf("%+v", got) != fmt.Sprintf("%+v", *tc.want) {
				t.Errorf("response\n got %+v\nwant %+v", got, *tc.want)
			}
		})
	}
}

// TestVacationDirectly tests the vacation functionality directly using the Go API
func TestVacationDirectly(t *testing.T) {
	ctx := context.Background()

	// Test basic vacation command
	script := `require ["vacation"];
	
	vacation "I'm on vacation.";
	`

	opts := sieve.DefaultOptions()
	opts.Lexer.Filename = "inline"

	parsedScript, err := sieve.Load(strings.NewReader(script), opts)
	if err != nil {
		t.Fatal(err)
	}

	// Create runtime data with a static envelope
	env := interp.EnvelopeStatic{
		From: "sender@example.com",
		To:   "recipient@example.com",
	}

	data := sieve.NewRuntimeData(parsedScript, interp.DummyPolicy{}, env, interp.MessageStatic{})

	// Execute the script
	err = parsedScript.Execute(ctx, data)
	if err != nil {
		t.Fatal(err)
	}

	// Check that a vacation response was added
	if len(data.VacationResponses) != 1 {
		t.Fatalf("Expected 1 vacation response, got %d", len(data.VacationResponses))
	}

	// Check the response details
	resp, ok := data.VacationResponses["sender@example.com"]
	if !ok {
		t.Fatalf("No vacation response for sender@example.com")
	}
	if resp.Body != "I'm on vacation." {
		t.Errorf("Expected body 'I'm on vacation.', got '%s'", resp.Body)
	}
	if resp.Subject != "Automated reply" {
		t.Errorf("Expected subject 'Automated reply', got '%s'", resp.Subject)
	}
	if resp.Days != 7 {
		t.Errorf("Expected days 7, got %d", resp.Days)
	}
}
