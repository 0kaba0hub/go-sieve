package server

import (
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/go-sieve/managesieve"
)

var errClosed = errors.New("managesieve/server: server closed")

type Logger interface {
	Printf(format string, args ...interface{})
}

type Backend interface {
	Open(*Conn, string) (Storage, error)
}

type BackendFunc func(*Conn, string) (Storage, error)

func (f BackendFunc) Open(conn *Conn, username string) (Storage, error) {
	return f(conn, username)
}

type Options struct {
	Capabilities func(*Conn) []managesieve.CapabilityItem
	TLSConfig    *tls.Config

	InsecureAuth bool
	DebugWriter  io.Writer
	Logger       Logger

	AuthTimeout time.Duration
	IdleTimeout time.Duration

	MaxLiteralBytes         int64
	MaxAtomBytes            int
	MaxQuotedBytes          int
	MaxCommandArgs          int
	MaxNonLiteralCommandLen int64
}

func (o *Options) wrapReadWriter(rw io.ReadWriter) io.ReadWriter {
	if o == nil || o.DebugWriter == nil {
		return rw
	}
	return struct {
		io.Reader
		io.Writer
	}{
		Reader: io.TeeReader(rw, o.DebugWriter),
		Writer: io.MultiWriter(rw, o.DebugWriter),
	}
}

type Server struct {
	backend Backend
	options Options
	auths   map[string]SASLServerFactory

	mu        sync.Mutex
	listeners map[net.Listener]struct{}
	conns     map[*Conn]struct{}
	closed    bool
	wg        sync.WaitGroup
}

type SASLServerFactory func(*Conn) (sasl.Server, error)

func New(backend Backend, options *Options) *Server {
	if options == nil {
		options = &Options{}
	}
	if options.AuthTimeout <= 0 {
		options.AuthTimeout = 30 * time.Second
	}
	if options.IdleTimeout <= 0 {
		options.IdleTimeout = 30 * time.Minute
	}
	if options.MaxLiteralBytes <= 0 {
		options.MaxLiteralBytes = 32 << 20
	}
	if options.MaxAtomBytes <= 0 {
		options.MaxAtomBytes = 1024
	}
	if options.MaxQuotedBytes <= 0 {
		options.MaxQuotedBytes = 1024
	}
	if options.MaxCommandArgs <= 0 {
		options.MaxCommandArgs = 32
	}
	if options.MaxNonLiteralCommandLen <= 0 {
		options.MaxNonLiteralCommandLen = 16 << 10
	}
	return &Server{
		backend:   backend,
		options:   *options,
		auths:     map[string]SASLServerFactory{},
		listeners: map[net.Listener]struct{}{},
		conns:     map[*Conn]struct{}{},
	}
}

func (s *Server) EnableAuth(name string, f SASLServerFactory) {
	s.auths[name] = f
}

func (s *Server) logger() Logger {
	if s.options.Logger != nil {
		return s.options.Logger
	}
	return log.Default()
}

func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errClosed
	}
	s.listeners[ln] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.listeners, ln)
		s.mu.Unlock()
	}()

	s.wg.Add(1)
	defer s.wg.Done()

	for {
		conn, err := ln.Accept()
		if errors.Is(err, net.ErrClosed) {
			return nil
		}
		if err != nil {
			return err
		}
		go newConn(conn, s).serve()
	}
}

func (s *Server) ListenAndServe(addr string) error {
	if addr == "" {
		addr = ":4190"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) ListenAndServeTLS(addr string) error {
	if addr == "" {
		addr = ":4190"
	}
	if s.options.TLSConfig == nil {
		return errors.New("managesieve/server: TLSConfig is required")
	}
	ln, err := tls.Listen("tcp", addr, s.options.TLSConfig)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errClosed
	}
	s.closed = true
	listeners := make([]net.Listener, 0, len(s.listeners))
	for l := range s.listeners {
		listeners = append(listeners, l)
	}
	conns := make([]*Conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.mu.Unlock()

	for _, l := range listeners {
		_ = l.Close()
	}
	for _, c := range conns {
		_ = c.Close()
	}

	s.wg.Wait()
	return nil
}
