package server

import (
	"bytes"
	"context"
	"sort"
	"strconv"
	"sync"
	"unicode/utf8"

	"github.com/foxcpp/go-sieve"
	"github.com/foxcpp/go-sieve/managesieve"
	"strings"
)

type MemorySessionOptions struct {
	Implementation string
	Version        string
	Language       string
	Notify         []string

	SieveExtensions []string
	SASLMechanisms  []string

	MaxRedirects *uint32
	MaxScripts   int
	MaxScriptLen int64

	Load func(script []byte) error
}

type MemorySession struct {
	mu sync.Mutex

	opts MemorySessionOptions

	scripts map[string][]byte
	active  string
}

func NewMemorySession(opts MemorySessionOptions) *MemorySession {
	if opts.Implementation == "" {
		opts.Implementation = "go-sieve managesieve"
	}
	if opts.Version == "" {
		opts.Version = "1.0"
	}
	if opts.Load == nil {
		opts.Load = func(script []byte) error {
			_, err := sieve.Load(bytes.NewReader(script), sieve.DefaultOptions())
			return err
		}
	}
	return &MemorySession{
		opts:    opts,
		scripts: map[string][]byte{},
	}
}

func (s *MemorySession) Close() error { return nil }

func (o MemorySessionOptions) Capabilities() []managesieve.CapabilityItem {
	items := []managesieve.CapabilityItem{
		{Name: string(managesieve.CapImplementation), Value: strPtr(o.Implementation)},
		{Name: string(managesieve.CapSieve), Value: strPtr(strings.Join(o.SieveExtensions, " "))},
		{Name: string(managesieve.CapVersion), Value: strPtr(o.Version)},
	}
	if o.SASLMechanisms != nil {
		items = append(items, managesieve.CapabilityItem{Name: string(managesieve.CapSASL), Value: strPtr(strings.Join(o.SASLMechanisms, " "))})
	}
	if o.Language != "" {
		items = append(items, managesieve.CapabilityItem{Name: string(managesieve.CapLanguage), Value: strPtr(o.Language)})
	}
	if len(o.Notify) > 0 {
		items = append(items, managesieve.CapabilityItem{Name: string(managesieve.CapNotify), Value: strPtr(strings.Join(o.Notify, " "))})
	}
	if o.MaxRedirects != nil {
		items = append(items, managesieve.CapabilityItem{Name: string(managesieve.CapMaxRedirects), Value: strPtr(toStr(*o.MaxRedirects))})
	}
	return items
}

func (s *MemorySession) ListScripts(_ context.Context) ([]managesieve.ScriptInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]managesieve.ScriptInfo, 0, len(s.scripts))
	for name := range s.scripts {
		out = append(out, managesieve.ScriptInfo{Name: name, Active: name == s.active})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *MemorySession) GetScript(_ context.Context, name string) ([]byte, error) {
	if err := validateScriptName(name, false); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.scripts[name]
	if !ok {
		return nil, &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCodeNonexistent, Text: "No such script"}
	}
	return append([]byte(nil), v...), nil
}

func (s *MemorySession) PutScript(_ context.Context, name string, script []byte) error {
	if err := validateScriptName(name, false); err != nil {
		return err
	}
	if s.opts.MaxScriptLen > 0 && int64(len(script)) > s.opts.MaxScriptLen {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCode("QUOTA/MAXSIZE"), Text: "Script is too large"}
	}
	if err := s.opts.Load(script); err != nil {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: err.Error()}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.scripts[name]; !ok && s.opts.MaxScripts > 0 && len(s.scripts) >= s.opts.MaxScripts {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCode("QUOTA/MAXSCRIPTS"), Text: "Too many scripts"}
	}
	s.scripts[name] = append([]byte(nil), script...)
	return nil
}

func (s *MemorySession) CheckScript(_ context.Context, script []byte) error {
	if s.opts.MaxScriptLen > 0 && int64(len(script)) > s.opts.MaxScriptLen {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCode("QUOTA/MAXSIZE"), Text: "Script is too large"}
	}
	if err := s.opts.Load(script); err != nil {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: err.Error()}
	}
	return nil
}

func (s *MemorySession) SetActive(_ context.Context, name string) error {
	if err := validateScriptName(name, true); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == "" {
		s.active = ""
		return nil
	}
	if _, ok := s.scripts[name]; !ok {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCodeNonexistent, Text: "No such script"}
	}
	s.active = name
	return nil
}

func (s *MemorySession) DeleteScript(_ context.Context, name string) error {
	if err := validateScriptName(name, false); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if name == s.active {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCodeActive, Text: "Script is active"}
	}
	if _, ok := s.scripts[name]; !ok {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCodeNonexistent, Text: "No such script"}
	}
	delete(s.scripts, name)
	return nil
}

func (s *MemorySession) RenameScript(_ context.Context, oldName, newName string) error {
	if err := validateScriptName(oldName, false); err != nil {
		return err
	}
	if err := validateScriptName(newName, false); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.scripts[oldName]
	if !ok {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCodeNonexistent, Text: "No such script"}
	}
	if _, exists := s.scripts[newName]; exists {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCodeAlreadyExists, Text: "Target exists"}
	}
	delete(s.scripts, oldName)
	s.scripts[newName] = v
	if s.active == oldName {
		s.active = newName
	}
	return nil
}

func (s *MemorySession) HaveSpace(_ context.Context, name string, size int64) error {
	if err := validateScriptName(name, false); err != nil {
		return err
	}
	if size < 0 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid size"}
	}
	if s.opts.MaxScriptLen > 0 && size > s.opts.MaxScriptLen {
		return &managesieve.Error{Type: managesieve.StatusNO, Code: managesieve.ResponseCode("QUOTA/MAXSIZE"), Text: "Script is too large"}
	}
	return nil
}

func validateScriptName(name string, allowEmpty bool) error {
	if name == "" {
		if allowEmpty {
			return nil
		}
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Script name must not be empty"}
	}
	for _, r := range name {
		switch {
		case r >= 0x00 && r <= 0x1F:
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid script name"}
		case r == 0x7F:
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid script name"}
		case r >= 0x80 && r <= 0x9F:
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid script name"}
		case r == 0x2028 || r == 0x2029:
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid script name"}
		}
	}
	if len([]rune(name)) > 128 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Script name too long"}
	}
	if !utf8.ValidString(name) {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid UTF-8 in script name"}
	}
	return nil
}

func strPtr(v string) *string {
	return &v
}

func toStr(v uint32) string {
	return strconv.FormatUint(uint64(v), 10)
}

var _ Storage = (*MemorySession)(nil)
