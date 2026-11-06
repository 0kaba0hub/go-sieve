package interp

import (
	"context"
	"slices"
	"testing"
)

type recordingPolicy struct {
	DummyPolicy
	asked []string
	allow int
}

func (p *recordingPolicy) RedirectAllowed(_ context.Context, _ *RuntimeData, addr string) (bool, error) {
	p.asked = append(p.asked, addr)
	return len(p.asked) <= p.allow, nil
}

// The policy alone decides the redirect limit: :list asks it too, and the
// interpreter adds no count of its own (Options.MaxRedirects is left 0 here).
func TestRedirectLimitIsThePolicys(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		allow     int
		wantAsked []string
		wantDone  int
	}{
		{"address and list both ask", `require ["extlists", "copy"]; redirect :copy "a@example.org"; redirect :list "team";`, 2, []string{"a@example.org", "team"}, 2},
		{"refused list is not applied", `require "extlists"; redirect :list "team";`, 0, []string{"team"}, 0},
		{"over the policy limit nothing errors here", `require "copy"; redirect :copy "a@example.org"; redirect :copy "b@example.org";`, 1, []string{"a@example.org", "b@example.org"}, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := loadWithAllowed(t, c.src, nil)
			if err != nil {
				t.Fatal(err)
			}
			p := &recordingPolicy{allow: c.allow}
			d := NewRuntimeData(s, p, EnvelopeStatic{}, MessageStatic{})
			if err := s.Execute(context.Background(), d); err != nil {
				t.Fatalf("execute: %v", err)
			}
			if !slices.Equal(p.asked, c.wantAsked) {
				t.Errorf("policy asked for %v, want %v", p.asked, c.wantAsked)
			}
			done := 0
			for _, a := range d.AppliedActions {
				if _, ok := a.(ActionRedirect); ok {
					done++
				}
			}
			if done != c.wantDone {
				t.Errorf("%d redirects applied, want %d", done, c.wantDone)
			}
		})
	}
}
