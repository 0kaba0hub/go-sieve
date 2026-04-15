package server

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/foxcpp/go-sieve/managesieve"
	"github.com/foxcpp/go-sieve/managesieve/internal/wire"
)

type Conn struct {
	server *Server

	mu sync.Mutex

	conn net.Conn
	br   *bufio.Reader
	bw   *bufio.Writer
	dec  *wire.Decoder

	session Storage
	owner   string
	state   managesieve.ConnState
	closed  bool
}

func newConn(conn net.Conn, s *Server) *Conn {
	rw := s.options.wrapReadWriter(conn)
	br := bufio.NewReader(rw)
	bw := bufio.NewWriter(rw)
	dec := wire.NewDecoder(br)
	dec.MaxLiteralBytes = s.options.MaxLiteralBytes
	dec.MaxAtomBytes = s.options.MaxAtomBytes
	dec.MaxQuotedBytes = s.options.MaxQuotedBytes
	dec.MaxCommandArgs = s.options.MaxCommandArgs
	dec.MaxNonLiteralCommandLen = s.options.MaxNonLiteralCommandLen
	return &Conn{
		server: s,
		conn:   conn,
		br:     br,
		bw:     bw,
		dec:    dec,
		state:  managesieve.ConnStateNotAuthenticated,
	}
}

func (c *Conn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.state = managesieve.ConnStateLogout
	conn := c.conn
	sess := c.session
	c.mu.Unlock()

	if sess != nil {
		_ = sess.Close()
	}
	return conn.Close()
}

func (c *Conn) serve() {
	c.server.mu.Lock()
	c.server.conns[c] = struct{}{}
	c.server.mu.Unlock()
	defer func() {
		c.server.mu.Lock()
		delete(c.server.conns, c)
		c.server.mu.Unlock()
		_ = c.Close()
	}()

	c.setDeadline()
	if err := c.writeCapabilityResponse(managesieve.StatusResponse{Type: managesieve.StatusOK}); err != nil {
		c.server.logger().Printf("managesieve/server: failed greeting: %v", err)
		return
	}

	for {
		c.setDeadline()
		name, args, err := c.dec.ReadCommand()
		if isTimeoutError(err) {
			c.setWriteDeadline()
			_ = c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusBYE, Text: "Idle timeout"})
			return
		}
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled) {
			return
		}
		if err != nil {
			_ = c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusNO, Text: "Syntax error"})
			return
		}

		if err := c.handleCommand(name, args); err != nil {
			if isTimeoutError(err) {
				c.setWriteDeadline()
				_ = c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusBYE, Text: "Idle timeout"})
				return
			}
			var msErr *managesieve.Error
			if errors.As(err, &msErr) {
				_ = c.writeStatus(managesieve.StatusResponse(*msErr))
				continue
			}
			c.server.logger().Printf("managesieve/server: command %s failed: %v", name, err)
			_ = c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusNO, Text: "Internal server error"})
		}

		if c.state == managesieve.ConnStateLogout {
			return
		}
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (c *Conn) setDeadline() {
	timeout := c.server.options.IdleTimeout
	if c.state == managesieve.ConnStateNotAuthenticated {
		timeout = c.server.options.AuthTimeout
	}
	if timeout <= 0 {
		_ = c.conn.SetDeadline(time.Time{})
		return
	}
	_ = c.conn.SetDeadline(time.Now().Add(timeout))
}

func (c *Conn) setWriteDeadline() {
	timeout := c.server.options.IdleTimeout
	if timeout <= 0 {
		timeout = time.Second
	}
	_ = c.conn.SetDeadline(time.Now().Add(timeout))
}

func (c *Conn) Authorize(owner string) error {
	if owner == "" {
		owner = c.owner
	}
	sess, err := c.server.backend.Open(c, owner)
	if err != nil {
		return err
	}
	if c.session != nil {
		_ = c.session.Close()
	}
	c.session = sess
	c.owner = owner
	c.state = managesieve.ConnStateAuthenticated
	return nil
}
func (c *Conn) handleCommand(name string, args []string) error {
	switch strings.ToUpper(name) {
	case "CAPABILITY":
		if len(args) != 0 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		return c.writeCapabilityResponse(managesieve.StatusResponse{Type: managesieve.StatusOK})
	case "STARTTLS":
		return c.handleStartTLS(args)
	case "AUTHENTICATE":
		return c.handleAuthenticate(args)
	case "LOGOUT":
		if len(args) != 0 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		c.state = managesieve.ConnStateLogout
		return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
	case "NOOP":
		if len(args) > 1 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		var resp managesieve.StatusResponse
		resp.Type = managesieve.StatusOK
		if len(args) == 1 {
			resp.Code = managesieve.ResponseCodeTag
			resp.CodeArgs = []string{args[0]}
		}
		return c.writeStatus(resp)
	}

	if c.state != managesieve.ConnStateAuthenticated {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Authenticate first"}
	}

	ctx := context.Background()
	switch strings.ToUpper(name) {
	case "LISTSCRIPTS":
		return c.handleListScripts(ctx, args)
	case "GETSCRIPT":
		return c.handleGetScript(ctx, args)
	case "PUTSCRIPT":
		return c.handlePutScript(ctx, args)
	case "CHECKSCRIPT":
		return c.handleCheckScript(ctx, args)
	case "SETACTIVE":
		if len(args) != 1 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		if err := c.session.SetActive(ctx, args[0]); err != nil {
			return err
		}
		return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
	case "DELETESCRIPT":
		if len(args) != 1 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		if err := c.session.DeleteScript(ctx, args[0]); err != nil {
			return err
		}
		return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
	case "RENAMESCRIPT":
		if len(args) != 2 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		if err := c.session.RenameScript(ctx, args[0], args[1]); err != nil {
			return err
		}
		return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
	case "HAVESPACE":
		if len(args) != 2 {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
		}
		n, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid size"}
		}
		if err := c.session.HaveSpace(ctx, args[0], n); err != nil {
			return err
		}
		return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
	default:
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Unknown command"}
	}
}

func (c *Conn) handleStartTLS(args []string) error {
	if len(args) != 0 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
	}
	if c.state != managesieve.ConnStateNotAuthenticated {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Wrong state"}
	}
	if c.server.options.TLSConfig == nil {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "STARTTLS unavailable"}
	}
	if err := c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK}); err != nil {
		return err
	}

	tlsConn := tls.Server(c.conn, c.server.options.TLSConfig)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}
	rw := c.server.options.wrapReadWriter(tlsConn)
	c.conn = tlsConn
	c.br = bufio.NewReader(rw)
	c.bw = bufio.NewWriter(rw)
	c.dec = wire.NewDecoder(c.br)
	c.dec.MaxLiteralBytes = c.server.options.MaxLiteralBytes
	c.dec.MaxAtomBytes = c.server.options.MaxAtomBytes
	c.dec.MaxQuotedBytes = c.server.options.MaxQuotedBytes
	c.dec.MaxCommandArgs = c.server.options.MaxCommandArgs
	c.dec.MaxNonLiteralCommandLen = c.server.options.MaxNonLiteralCommandLen
	c.setDeadline()
	return c.writeCapabilityResponse(managesieve.StatusResponse{Type: managesieve.StatusOK})
}

func (c *Conn) handleAuthenticate(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
	}
	if c.state != managesieve.ConnStateNotAuthenticated {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Wrong state"}
	}
	if _, ok := c.conn.(*tls.Conn); !ok && !c.server.options.InsecureAuth {
		return &managesieve.Error{
			Type: managesieve.StatusNO,
			Code: managesieve.ResponseCodeEncryptNeeded,
			Text: "TLS required",
		}
	}

	mech := strings.ToUpper(args[0])
	newSASLServer := c.server.auths[mech]
	if newSASLServer == nil {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "SASL mechanism not supported"}
	}
	srv, err := newSASLServer(c)
	if err != nil {
		return err
	}

	var resp []byte
	if len(args) == 2 {
		resp, err = decodeB64(args[1])
		if err != nil {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Malformed SASL response"}
		}
	}

	for {
		challenge, done, err := srv.Next(resp)
		if err != nil {
			return err
		}
		if done {
			resp := managesieve.StatusResponse{Type: managesieve.StatusOK}
			if len(challenge) > 0 {
				resp.Code = managesieve.ResponseCodeSASL
				resp.CodeArgs = []string{base64.StdEncoding.EncodeToString(challenge)}
			}
			return c.writeCapabilityResponse(resp)
		}

		enc := wire.NewEncoder(c.bw)
		enc.StringServer(base64.StdEncoding.EncodeToString(challenge))
		if err := enc.CRLF(); err != nil {
			return err
		}

		in, err := c.dec.ReadString()
		if err != nil {
			return err
		}
		if err := c.dec.ExpectCRLF(); err != nil {
			return err
		}
		if in == "*" {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Authentication cancelled"}
		}
		resp, err = decodeB64(in)
		if err != nil {
			return &managesieve.Error{Type: managesieve.StatusNO, Text: "Malformed SASL response"}
		}
	}

}

func (c *Conn) handleListScripts(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
	}
	list, err := c.session.ListScripts(ctx)
	if err != nil {
		return err
	}
	for _, it := range list {
		enc := wire.NewEncoder(c.bw)
		enc.StringServer(it.Name)
		if it.Active {
			enc.SP().Atom("ACTIVE")
		}
		if err := enc.CRLF(); err != nil {
			return err
		}
	}
	return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
}

func (c *Conn) handleGetScript(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
	}
	script, err := c.session.GetScript(ctx, args[0])
	if err != nil {
		return err
	}
	enc := wire.NewEncoder(c.bw)
	enc.StringServer(string(script))
	if err := enc.CRLF(); err != nil {
		return err
	}
	return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
}

func (c *Conn) handlePutScript(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
	}
	if err := c.session.PutScript(ctx, args[0], []byte(args[1])); err != nil {
		return err
	}
	return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
}

func (c *Conn) handleCheckScript(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return &managesieve.Error{Type: managesieve.StatusNO, Text: "Invalid arguments"}
	}
	if err := c.session.CheckScript(ctx, []byte(args[0])); err != nil {
		return err
	}
	return c.writeStatus(managesieve.StatusResponse{Type: managesieve.StatusOK})
}

func (c *Conn) writeCapabilityResponse(status managesieve.StatusResponse) error {
	items := c.availableCapabilities()
	if c.server.options.TLSConfig != nil {
		if _, isTLS := c.conn.(*tls.Conn); !isTLS && c.state == managesieve.ConnStateNotAuthenticated {
			if !hasCapability(items, string(managesieve.CapStartTLS)) {
				items = append(items, managesieve.CapabilityItem{Name: string(managesieve.CapStartTLS)})
			}
		}
	}

	for _, item := range items {
		enc := wire.NewEncoder(c.bw)
		enc.StringServer(item.Name)
		if item.Value != nil {
			enc.SP().StringServer(*item.Value)
		}
		if err := enc.CRLF(); err != nil {
			return err
		}
	}
	return c.writeStatus(status)
}

func (c *Conn) availableCapabilities() []managesieve.CapabilityItem {
	items := []managesieve.CapabilityItem{}
	if c.server.options.Capabilities != nil {
		items = append(items, c.server.options.Capabilities(c)...)
	}
	out := make([]managesieve.CapabilityItem, 0, len(items))
	for _, item := range items {
		if !strings.EqualFold(item.Name, string(managesieve.CapSASL)) && !strings.EqualFold(item.Name, string(managesieve.CapOwner)) {
			out = append(out, item)
		}
	}
	if c.state == managesieve.ConnStateAuthenticated && c.owner != "" {
		owner := c.owner
		out = append(out, managesieve.CapabilityItem{Name: string(managesieve.CapOwner), Value: &owner})
	}
	if c.state != managesieve.ConnStateAuthenticated {
		if item := c.saslCapability(); item != nil {
			out = append(out, *item)
		}
	}
	return out
}

func (c *Conn) saslCapability() *managesieve.CapabilityItem {
	mechs := make([]string, 0, len(c.server.auths))
	for mech := range c.server.auths {
		mechs = append(mechs, mech)
	}
	allowPlain := false
	if _, ok := c.conn.(*tls.Conn); ok || c.server.options.InsecureAuth {
		allowPlain = true
	}

	filtered := make([]string, 0, len(mechs))
	for _, mech := range mechs {
		if strings.EqualFold(mech, "PLAIN") && !allowPlain {
			continue
		}
		filtered = append(filtered, mech)
	}

	if len(filtered) == 0 {
		if c.state == managesieve.ConnStateNotAuthenticated && c.server.options.TLSConfig != nil && !allowPlain {
			v := ""
			return &managesieve.CapabilityItem{Name: string(managesieve.CapSASL), Value: &v}
		}
		return nil
	}

	v := strings.Join(filtered, " ")
	return &managesieve.CapabilityItem{Name: string(managesieve.CapSASL), Value: &v}
}

func (c *Conn) writeStatus(resp managesieve.StatusResponse) error {
	if resp.Type == "" {
		resp.Type = managesieve.StatusOK
	}
	enc := wire.NewEncoder(c.bw)
	enc.Atom(string(resp.Type))
	if resp.Code != "" {
		enc.SP().Atom("(").Atom(string(resp.Code))
		for _, arg := range resp.CodeArgs {
			enc.SP().StringServer(arg)
		}
		enc.Atom(")")
	}
	if resp.Text != "" {
		enc.SP().StringServer(resp.Text)
	}
	return enc.CRLF()
}

func hasCapability(items []managesieve.CapabilityItem, want string) bool {
	for _, item := range items {
		if strings.EqualFold(item.Name, want) {
			return true
		}
	}
	return false
}

func decodeB64(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
