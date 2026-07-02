package interp

import (
	"context"

	"github.com/foxcpp/go-sieve/parser"
)

const YariloDebugExtension = "vnd.yarilo.debug"

type CmdDebugLog struct {
	Message string
}

func (c CmdDebugLog) Execute(_ context.Context, d *RuntimeData) error {
	if d.Script.opts.DebugLog == nil {
		return nil
	}
	msg := expandVars(d, c.Message)
	d.Script.opts.DebugLog(msg)
	return nil
}

func loadDebugLog(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdDebugLog{}
	err := LoadSpec(s, &Spec{
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Message = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}
	if !usedVarsAreValid(s, cmd.Message) {
		return nil, parser.ErrorAt(pcmd.Position, "invalid variable used: %v", cmd.Message)
	}
	return cmd, nil
}
