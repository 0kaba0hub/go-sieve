package interp

import (
	"fmt"
	"strings"

	"github.com/foxcpp/go-sieve/parser"
)

func loadRequire(s *Script, pcmd parser.Cmd) (Cmd, error) {
	var exts []string
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				Optional: false,
				MatchStr: func(val []string) {
					exts = val
				},
				MinStrCount: 1,
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	for _, ext := range exts {
		if ext == DovecotTestExtension {
			if s.opts.T == nil {
				return nil, fmt.Errorf("testing environment is not available, cannot use vnd.dovecot.testsuite")
			}
			s.extensions[DovecotTestExtension] = struct{}{}
			continue
		}

		if _, ok := supportedRequires[ext]; !ok {
			return nil, fmt.Errorf("loadRequire: unsupported extension: %v", ext)
		}
		s.extensions[ext] = struct{}{}
		switch ext {
		case "imapflags":
			s.extensions["imap4flags"] = struct{}{}
		case "vacation-seconds":
			s.extensions["vacation"] = struct{}{}
		}
	}
	return nil, nil
}

func loadIf(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdIf{}
	err := loadCondBlock(s, pcmd, &cmd.Test, &cmd.Block)
	return cmd, err
}

func loadElsif(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdElsif{}
	err := loadCondBlock(s, pcmd, &cmd.Test, &cmd.Block)
	return cmd, err
}

// loadCondBlock loads an if/elsif. Under a false ihave the block is not loaded
// and never runs; under a true one it may use what the test names (RFC 5463).
func loadCondBlock(s *Script, pcmd parser.Cmd, test *Test, block *[]Cmd) error {
	if len(pcmd.Tests) == 1 && strings.EqualFold(pcmd.Tests[0].Id, "ihave") {
		t, err := LoadTest(s, pcmd.Tests[0])
		if err != nil {
			return err
		}
		ih := t.(TestIhave)
		if !ih.Have {
			*test = t
			return nil
		}
		for _, c := range ih.Capabilities {
			s.guarded[c]++
		}
		defer func() {
			for _, c := range ih.Capabilities {
				s.guarded[c]--
			}
		}()
	}
	return LoadSpec(s, &Spec{
		AddTest:  func(t Test) { *test = t },
		AddBlock: func(cmds []Cmd) { *block = cmds },
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
}

func loadElse(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdElse{}
	err := LoadSpec(s, &Spec{
		AddBlock: func(cmds []Cmd) {
			cmd.Block = cmds
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}

func loadStop(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdStop{}
	err := LoadSpec(s, &Spec{}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}
