package server

import (
	"context"

	"github.com/foxcpp/go-sieve/managesieve"
)

type Storage interface {
	Close() error

	ListScripts(ctx context.Context) ([]ScriptInfo, error)
	GetScript(ctx context.Context, name string) ([]byte, error)
	PutScript(ctx context.Context, name string, script []byte) error
	CheckScript(ctx context.Context, script []byte) error
	SetActive(ctx context.Context, name string) error
	DeleteScript(ctx context.Context, name string) error
	RenameScript(ctx context.Context, oldName, newName string) error
	HaveSpace(ctx context.Context, name string, size int64) error
}

type ScriptInfo = managesieve.ScriptInfo

// ScriptWarnings is returned by Storage.PutScript or Storage.CheckScript when
// the script is accepted but compilation produced non-fatal warnings.
// The server translates this into an OK (WARNINGS) response per RFC 5804 §2.6.
type ScriptWarnings struct {
	Text string
}

func (w *ScriptWarnings) Error() string { return w.Text }
