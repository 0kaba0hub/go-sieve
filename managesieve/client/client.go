package client

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/foxcpp/go-sieve/managesieve"
	"github.com/foxcpp/go-sieve/managesieve/internal/wire"
)

const defaultDialTimeout = 30 * time.Second

type Options struct {
	TLSConfig         *tls.Config
	DebugWriter       io.Writer
	Dialer            *net.Dialer
	AllowInsecureAuth bool
}

func (o *Options) tlsConfig() *tls.Config {
	if o != nil && o.TLSConfig != nil {
		return o.TLSConfig.Clone()
	}
	return new(tls.Config)
}

func (o *Options) dialer() *net.Dialer {
	if o != nil && o.Dialer != nil {
		return o.Dialer
	}
	return &net.Dialer{Timeout: defaultDialTimeout}
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

type Client struct {
	conn net.Conn
	opts Options

	br  *bufio.Reader
	bw  *bufio.Writer
	dec *wire.Decoder

	mu    sync.Mutex
	state managesieve.ConnState
	caps  managesieve.Capabilities
}

const defaultMaxLiteralBytes = 32 << 20
const defaultMaxAtomBytes = 1024
const defaultMaxQuotedBytes = 1024
const defaultMaxCommandArgs = 32
const defaultMaxNonLiteralCommandLen = 16 << 10

func New(conn net.Conn, options *Options) (*Client, error) {
	if options == nil {
		options = &Options{}
	}
	rw := options.wrapReadWriter(conn)
	br := bufio.NewReader(rw)
	bw := bufio.NewWriter(rw)
	c := &Client{
		conn:  conn,
		opts:  *options,
		br:    br,
		bw:    bw,
		dec:   wire.NewDecoder(br),
		state: managesieve.ConnStateNotAuthenticated,
	}
	c.dec.MaxLiteralBytes = defaultMaxLiteralBytes
	c.dec.MaxAtomBytes = defaultMaxAtomBytes
	c.dec.MaxQuotedBytes = defaultMaxQuotedBytes
	c.dec.MaxCommandArgs = defaultMaxCommandArgs
	c.dec.MaxNonLiteralCommandLen = defaultMaxNonLiteralCommandLen
	if err := c.readCapabilitiesGreetingLocked(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

func Dial(address string, options *Options) (*Client, error) {
	if options == nil {
		options = &Options{}
	}
	conn, err := options.dialer().Dial("tcp", address)
	if err != nil {
		return nil, err
	}
	return New(conn, options)
}

func DialTLS(address string, options *Options) (*Client, error) {
	if options == nil {
		options = &Options{}
	}
	tlsCfg := options.tlsConfig()
	dialer := options.dialer()
	conn, err := tls.DialWithDialer(dialer, "tcp", address, tlsCfg)
	if err != nil {
		return nil, err
	}
	return New(conn, options)
}

func DialStartTLS(address string, options *Options) (*Client, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	startTLSCfg := (*tls.Config)(nil)
	if options != nil {
		startTLSCfg = options.tlsConfig()
	} else {
		startTLSCfg = new(tls.Config)
	}
	if startTLSCfg.ServerName == "" {
		startTLSCfg.ServerName = host
	}

	c, err := Dial(address, options)
	if err != nil {
		return nil, err
	}
	if err := c.StartTLS(startTLSCfg); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = managesieve.ConnStateLogout
	return c.conn.Close()
}

func (c *Client) State() managesieve.ConnState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

func (c *Client) Caps() managesieve.Capabilities {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.caps
}

func (c *Client) StartTLS(cfg *tls.Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != managesieve.ConnStateNotAuthenticated {
		return fmt.Errorf("managesieve/client: STARTTLS only valid in not-authenticated state")
	}
	if !c.caps.StartTLS {
		return fmt.Errorf("managesieve/client: server doesn't advertise STARTTLS")
	}

	if err := c.writeSimpleCommandLocked("STARTTLS", nil); err != nil {
		return err
	}
	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return err
	}
	if status.Type != managesieve.StatusOK {
		return (*managesieve.Error)(&status)
	}

	tlsCfg := cfg
	if tlsCfg == nil {
		tlsCfg = c.opts.tlsConfig()
	}
	if tlsCfg.ServerName == "" {
		host, _, err := net.SplitHostPort(c.conn.RemoteAddr().String())
		if err == nil {
			tlsCfg.ServerName = host
		}
	}
	tlsConn := tls.Client(c.conn, tlsCfg)
	if err := tlsConn.Handshake(); err != nil {
		return err
	}

	c.conn = tlsConn
	rw := c.opts.wrapReadWriter(tlsConn)
	c.br = bufio.NewReader(rw)
	c.bw = bufio.NewWriter(rw)
	c.dec = wire.NewDecoder(c.br)
	c.dec.MaxLiteralBytes = defaultMaxLiteralBytes
	c.dec.MaxAtomBytes = defaultMaxAtomBytes
	c.dec.MaxQuotedBytes = defaultMaxQuotedBytes
	c.dec.MaxCommandArgs = defaultMaxCommandArgs
	c.dec.MaxNonLiteralCommandLen = defaultMaxNonLiteralCommandLen

	return c.readCapabilitiesResponseLocked()
}

func (c *Client) Authenticate(sc sasl.Client) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != managesieve.ConnStateNotAuthenticated {
		return fmt.Errorf("managesieve/client: AUTHENTICATE only valid in not-authenticated state")
	}
	if _, ok := c.conn.(*tls.Conn); !ok && !c.opts.AllowInsecureAuth {
		return fmt.Errorf("managesieve/client: TLS is required for authentication")
	}

	mech, ir, err := sc.Start()
	if err != nil {
		return err
	}

	enc := wire.NewEncoder(c.bw)
	enc.Atom("AUTHENTICATE").SP().StringClient(strings.ToUpper(mech))
	if ir != nil {
		enc.SP().StringClient(base64.StdEncoding.EncodeToString(ir))
	}
	if err := enc.CRLF(); err != nil {
		return err
	}

	var capItems []managesieve.CapabilityItem
	for {
		b, err := c.dec.PeekByte()
		if err != nil {
			return err
		}
		if b == '"' || b == '{' {
			line, err := c.dec.ReadString()
			if err != nil {
				return err
			}

			pb, err := c.dec.PeekByte()
			if err != nil {
				return err
			}
			if pb == ' ' {
				value, err := c.dec.ReadString()
				if err != nil {
					return err
				}
				if err := c.dec.ExpectCRLF(); err != nil {
					return err
				}
				capItems = append(capItems, managesieve.CapabilityItem{Name: line, Value: &value})
				continue
			}
			if err := c.dec.ExpectCRLF(); err != nil {
				return err
			}
			if len(capItems) > 0 || isCapabilityName(line) {
				capItems = append(capItems, managesieve.CapabilityItem{Name: line})
				continue
			}

			challenge, err := decodeB64(line)
			if err != nil {
				return err
			}
			resp, err := sc.Next(challenge)
			if err != nil {
				return err
			}

			enc = wire.NewEncoder(c.bw)
			enc.StringClient(base64.StdEncoding.EncodeToString(resp))
			if err := enc.CRLF(); err != nil {
				return err
			}
			continue
		}

		status, err := c.dec.ReadStatusLine()
		if err != nil {
			return err
		}
		if status.Type != managesieve.StatusOK {
			return (*managesieve.Error)(&status)
		}
		if status.Code == managesieve.ResponseCodeSASL && len(status.CodeArgs) > 0 {
			finalMsg, err := decodeB64(status.CodeArgs[0])
			if err != nil {
				return err
			}
			if _, err := sc.Next(finalMsg); err != nil {
				return err
			}
		}
		c.state = managesieve.ConnStateAuthenticated
		if len(capItems) != 0 {
			c.caps = managesieve.NewCapabilities(capItems)
			return nil
		}
		return c.refreshCapabilitiesLocked()
	}
}

func (c *Client) Capability() (managesieve.Capabilities, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.refreshCapabilitiesLocked(); err != nil {
		return managesieve.Capabilities{}, err
	}
	return c.caps, nil
}

func (c *Client) ListScripts() ([]managesieve.ScriptInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.requireAuthenticatedLocked(); err != nil {
		return nil, err
	}
	if err := c.writeSimpleCommandLocked("LISTSCRIPTS", nil); err != nil {
		return nil, err
	}

	var list []managesieve.ScriptInfo
	for {
		info, status, err := c.dec.ReadScriptOrStatus()
		if err != nil {
			return nil, err
		}
		if info != nil {
			list = append(list, *info)
			continue
		}
		if status.Type != managesieve.StatusOK {
			return nil, (*managesieve.Error)(status)
		}
		return list, nil
	}
}

func (c *Client) GetScript(name string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.requireAuthenticatedLocked(); err != nil {
		return nil, err
	}
	if err := c.writeSimpleCommandLocked("GETSCRIPT", []commandArg{strArg(name)}); err != nil {
		return nil, err
	}

	b, err := c.dec.PeekByte()
	if err != nil {
		return nil, err
	}
	if b != '"' && b != '{' {
		status, err := c.dec.ReadStatusLine()
		if err != nil {
			return nil, err
		}
		return nil, (*managesieve.Error)(&status)
	}

	script, err := c.dec.ReadString()
	if err != nil {
		return nil, err
	}
	if err := c.dec.ExpectCRLF(); err != nil {
		return nil, err
	}
	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return nil, err
	}
	if status.Type != managesieve.StatusOK {
		return nil, (*managesieve.Error)(&status)
	}
	return []byte(script), nil
}

func (c *Client) PutScript(name string, r io.Reader, size int64) error {
	if size < 0 {
		return fmt.Errorf("managesieve/client: negative script size")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.requireAuthenticatedLocked(); err != nil {
		return err
	}

	enc := wire.NewEncoder(c.bw)
	enc.Atom("PUTSCRIPT").SP().StringClient(name).SP()
	lw := enc.Literal(size, true)
	if _, err := io.Copy(lw, r); err != nil {
		_ = lw.Close()
		return err
	}
	if err := lw.Close(); err != nil {
		return err
	}
	if err := enc.CRLF(); err != nil {
		return err
	}

	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return err
	}
	if status.Type != managesieve.StatusOK {
		return (*managesieve.Error)(&status)
	}
	return nil
}

func (c *Client) CheckScript(r io.Reader, size int64) error {
	if size < 0 {
		return fmt.Errorf("managesieve/client: negative script size")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.requireAuthenticatedLocked(); err != nil {
		return err
	}

	enc := wire.NewEncoder(c.bw)
	enc.Atom("CHECKSCRIPT").SP()
	lw := enc.Literal(size, true)
	if _, err := io.Copy(lw, r); err != nil {
		_ = lw.Close()
		return err
	}
	if err := lw.Close(); err != nil {
		return err
	}
	if err := enc.CRLF(); err != nil {
		return err
	}

	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return err
	}
	if status.Type != managesieve.StatusOK {
		return (*managesieve.Error)(&status)
	}
	return nil
}

func (c *Client) SetActive(name string) error {
	return c.runSimpleAuthenticated("SETACTIVE", strArg(name))
}

func (c *Client) DeleteScript(name string) error {
	return c.runSimpleAuthenticated("DELETESCRIPT", strArg(name))
}

func (c *Client) RenameScript(oldName, newName string) error {
	return c.runSimpleAuthenticated("RENAMESCRIPT", strArg(oldName), strArg(newName))
}

func (c *Client) HaveSpace(name string, size int64) error {
	if size < 0 {
		return fmt.Errorf("managesieve/client: negative script size")
	}
	return c.runSimpleAuthenticated("HAVESPACE", strArg(name), numArg(size))
}

func (c *Client) Noop(tag string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	args := []commandArg{}
	if tag != "" {
		args = append(args, strArg(tag))
	}
	if err := c.writeSimpleCommandLocked("NOOP", args); err != nil {
		return err
	}
	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return err
	}
	if status.Type != managesieve.StatusOK {
		return (*managesieve.Error)(&status)
	}
	return nil
}

func (c *Client) Logout() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.writeSimpleCommandLocked("LOGOUT", nil); err != nil {
		return err
	}
	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return err
	}
	c.state = managesieve.ConnStateLogout
	if status.Type != managesieve.StatusOK && status.Type != managesieve.StatusBYE {
		return (*managesieve.Error)(&status)
	}
	_ = c.conn.Close()
	return nil
}

func (c *Client) runSimpleAuthenticated(name string, args ...commandArg) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.requireAuthenticatedLocked(); err != nil {
		return err
	}
	if err := c.writeSimpleCommandLocked(name, args); err != nil {
		return err
	}
	status, err := c.dec.ReadStatusLine()
	if err != nil {
		return err
	}
	if status.Type != managesieve.StatusOK {
		return (*managesieve.Error)(&status)
	}
	return nil
}

func (c *Client) requireAuthenticatedLocked() error {
	if c.state != managesieve.ConnStateAuthenticated {
		return fmt.Errorf("managesieve/client: command requires authenticated state")
	}
	return nil
}

func (c *Client) readCapabilitiesGreetingLocked() error {
	var items []managesieve.CapabilityItem
	for {
		item, status, err := c.dec.ReadCapabilityOrStatus()
		if err != nil {
			return err
		}
		if item != nil {
			items = append(items, *item)
			continue
		}
		if status.Type != managesieve.StatusOK {
			c.state = managesieve.ConnStateLogout
			return (*managesieve.Error)(status)
		}
		c.caps = managesieve.NewCapabilities(items)
		return nil
	}
}

func (c *Client) refreshCapabilitiesLocked() error {
	if err := c.writeSimpleCommandLocked("CAPABILITY", nil); err != nil {
		return err
	}
	return c.readCapabilitiesResponseLocked()
}

func (c *Client) readCapabilitiesResponseLocked() error {
	var items []managesieve.CapabilityItem
	for {
		item, status, err := c.dec.ReadCapabilityOrStatus()
		if err != nil {
			return err
		}
		if item != nil {
			items = append(items, *item)
			continue
		}
		if status.Type != managesieve.StatusOK {
			return (*managesieve.Error)(status)
		}
		c.caps = managesieve.NewCapabilities(items)
		return nil
	}
}

type argKind int

const (
	argString argKind = iota
	argNumber
)

type commandArg struct {
	kind  argKind
	value string
}

func strArg(v string) commandArg {
	return commandArg{kind: argString, value: v}
}

func numArg(v int64) commandArg {
	return commandArg{kind: argNumber, value: fmt.Sprintf("%d", v)}
}

func (c *Client) writeSimpleCommandLocked(name string, args []commandArg) error {
	enc := wire.NewEncoder(c.bw)
	enc.Atom(strings.ToUpper(name))
	for _, arg := range args {
		enc.SP()
		switch arg.kind {
		case argNumber:
			enc.Atom(arg.value)
		default:
			enc.StringClient(arg.value)
		}
	}
	return enc.CRLF()
}

func decodeB64(v string) ([]byte, error) {
	if v == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("managesieve/client: invalid base64 challenge: %w", err)
	}
	return b, nil
}

func isCapabilityName(v string) bool {
	known := []string{
		string(managesieve.CapImplementation),
		string(managesieve.CapSASL),
		string(managesieve.CapSieve),
		string(managesieve.CapStartTLS),
		string(managesieve.CapMaxRedirects),
		string(managesieve.CapNotify),
		string(managesieve.CapLanguage),
		string(managesieve.CapOwner),
		string(managesieve.CapVersion),
		string(managesieve.CapUnauthenticate),
	}
	sort.Strings(known)
	v = strings.ToUpper(v)
	i := sort.SearchStrings(known, v)
	return i < len(known) && known[i] == v
}
