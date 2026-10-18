package interp

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/foxcpp/go-sieve/lexer"
	"github.com/foxcpp/go-sieve/parser"
)

func loadWithAllowed(t *testing.T, src string, allowed []string) (*Script, error) {
	t.Helper()
	toks, err := lexer.Lex(strings.NewReader(src), &lexer.Options{})
	if err != nil {
		t.Fatal("lex:", err)
	}
	cmds, err := parser.Parse(lexer.NewStream(toks), &parser.Options{})
	if err != nil {
		t.Fatal("parse:", err)
	}
	opts := &Options{}
	if allowed != nil {
		opts.Allowed = func(ext string) bool { return slices.Contains(allowed, ext) }
	}
	return LoadScript(cmds, opts)
}

// ihave answers from the host's allowed set, a false one leaves its block
// dead instead of failing the script, and require "ihave" opens nothing else.
func TestIhaveHoldsToTheAllowedSet(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		allowed []string // nil: no Allowed hook
		loadErr string   // "" : the script loads
		have    *bool    // the first command's ihave answer, when it is an if
		exts    []string // Extensions(), sorted, when the script loads
	}{
		{name: "allowed under ihave is used and reported",
			src:     `require "ihave"; if ihave "fileinto" { fileinto "X"; }`,
			allowed: []string{"ihave", "fileinto"}, have: ptr(true), exts: []string{"fileinto", "ihave"}},
		{name: "not allowed: ihave false, block dead, script loads",
			src:     `require "ihave"; if ihave "fileinto" { fileinto "X"; }`,
			allowed: []string{"ihave"}, have: ptr(false), exts: []string{"ihave"}},
		{name: "unknown command under a false ihave loads",
			src:     `require "ihave"; if ihave "no-such-ext" { no_such_cmd; }`,
			allowed: []string{"ihave"}, have: ptr(false), exts: []string{"ihave"}},
		{name: "no hook: every supported extension",
			src:  `require "ihave"; if ihave "fileinto" { fileinto "X"; }`,
			have: ptr(true), exts: []string{"fileinto", "ihave"}},
		{name: "execute outside a block still needs require",
			src:     `require "ihave"; execute "prog";`,
			allowed: []string{"ihave", "vnd.yarilo.execute"}, loadErr: "requires 'vnd.yarilo.execute'"},
		{name: "fileinto outside a block still needs require",
			src:     `require "ihave"; fileinto "X";`,
			allowed: []string{"ihave", "fileinto"}, loadErr: "fileinto"},
		{name: "the guard ends with its block",
			src:     `require "ihave"; if ihave "fileinto" { keep; } fileinto "X";`,
			allowed: []string{"ihave", "fileinto"}, loadErr: "fileinto"},
		{name: "variables is never reported (RFC 5463 §4)",
			src:     `require "ihave"; if ihave "variables" { keep; }`,
			allowed: []string{"ihave", "variables"}, have: ptr(false), exts: []string{"ihave"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := loadWithAllowed(t, tc.src, tc.allowed)
			if tc.loadErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.loadErr) {
					t.Fatalf("load err = %v, want one naming %q", err, tc.loadErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if tc.have != nil {
				cmd, ok := s.cmd[0].(CmdIf)
				if !ok {
					t.Fatalf("first command is %T, want CmdIf", s.cmd[0])
				}
				if got, _ := cmd.Test.Check(context.Background(), nil); got != *tc.have {
					t.Errorf("ihave = %v, want %v", got, *tc.have)
				}
				if !*tc.have && cmd.Block != nil {
					t.Errorf("a false ihave loaded its block: %v", cmd.Block)
				}
			}
			got := s.Extensions()
			slices.Sort(got)
			if !slices.Equal(got, tc.exts) {
				t.Errorf("Extensions() = %v, want %v", got, tc.exts)
			}
		})
	}
}

func ptr(b bool) *bool { return &b }
