package interp

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
)

type locLoader struct {
	DummyPolicy
	scripts map[string]string // "location:name" -> source
	allow   map[string][]string
}

func (l locLoader) load(loc, name string) (io.ReadCloser, error) {
	src, ok := l.scripts[loc+":"+name]
	if !ok {
		return nil, nil
	}
	return io.NopCloser(strings.NewReader(src)), nil
}

func (l locLoader) LoadPersonalScript(_ context.Context, n string) (io.ReadCloser, error) {
	return l.load("personal", n)
}

func (l locLoader) LoadGlobalScript(_ context.Context, n string) (io.ReadCloser, error) {
	return l.load("global", n)
}

func (l locLoader) IncludeAllowed(loc string) func(string) bool {
	set := l.allow[loc]
	return func(ext string) bool { return slices.Contains(set, ext) }
}

// An included script's ihave answers from its own location's set, not from
// the including script's (#2290).
func TestIncludeAllowedByLocation(t *testing.T) {
	inner := `require ["ihave", "fileinto"]; if ihave "editheader" { fileinto "Wide"; } else { fileinto "Narrow"; }`
	allow := map[string][]string{
		"personal": {"ihave", "fileinto"},
		"global":   {"ihave", "fileinto", "editheader"},
	}
	for _, c := range []struct{ loc, want string }{{"personal", "Narrow"}, {"global", "Wide"}} {
		loader := locLoader{scripts: map[string]string{c.loc + ":inner": inner}, allow: allow}
		main := `require "include"; include :` + c.loc + ` "inner";`
		s, err := loadWithAllowed(t, main, []string{"include", "ihave", "fileinto", "editheader"})
		if err != nil {
			t.Fatal(err)
		}
		d := NewRuntimeData(s, loader, EnvelopeStatic{}, MessageStatic{})
		if err := s.Execute(context.Background(), d); err != nil {
			t.Fatalf("%s: %v", c.loc, err)
		}
		if !slices.Equal(d.Mailboxes, []string{c.want}) {
			t.Errorf(":%s include filed into %v, want %s", c.loc, d.Mailboxes, c.want)
		}
	}
}
