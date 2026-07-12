package interp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"io"
	"strings"

	message "github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"

	"github.com/foxcpp/go-sieve/parser"
)

// RawMessager is an optional Message extension exposing the complete raw
// RFC 2822 message bytes (headers + body). It is required to parse the MIME
// tree used by the RFC 5703 foreverypart / mime / extracttext extensions.
// A Message that does not implement it disables those extensions at runtime
// (foreverypart iterates nothing).
type RawMessager interface {
	RawMessageBytes() []byte
}

// mimePart is one node of the parsed MIME tree.
type mimePart struct {
	header      textproto.Header
	contentType string            // lowercased "type/subtype" (empty when absent)
	typ         string            // lowercased MIME type
	subtype     string            // lowercased MIME subtype
	params      map[string]string // Content-Type parameters
	body        []byte            // raw (undecoded) body of this entity
	children    []*mimePart
}

func (p *mimePart) isMultipart() bool {
	return strings.HasPrefix(p.contentType, "multipart/")
}

// descendants returns every descendant part in depth-first pre-order: each
// child, immediately followed by that child's own descendants. This is the
// iteration order RFC 5703 §3 requires for foreverypart.
func (p *mimePart) descendants() []*mimePart {
	var out []*mimePart
	for _, c := range p.children {
		out = append(out, c)
		out = append(out, c.descendants()...)
	}
	return out
}

// parseMIMETree parses a complete raw message (headers + body) into a MIME
// part tree.
func parseMIMETree(raw []byte) (*mimePart, error) {
	r := bufio.NewReader(bytes.NewReader(raw))
	hdr, err := textproto.ReadHeader(r)
	if err != nil {
		return nil, err
	}
	return parseMIMEEntity(hdr, r)
}

func parseMIMEEntity(hdr textproto.Header, body io.Reader) (*mimePart, error) {
	mh := message.Header{Header: hdr}
	ct, params, _ := mh.ContentType()
	ct = strings.ToLower(ct)
	typ, subtype := ct, ""
	if i := strings.IndexByte(ct, '/'); i >= 0 {
		typ, subtype = ct[:i], ct[i+1:]
	}
	raw, _ := io.ReadAll(body)
	node := &mimePart{
		header:      hdr,
		contentType: ct,
		typ:         typ,
		subtype:     subtype,
		params:      params,
		body:        raw,
	}
	if node.isMultipart() {
		if boundary := params["boundary"]; boundary != "" {
			mr := textproto.NewMultipartReader(bytes.NewReader(raw), boundary)
			for {
				part, err := mr.NextPart()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					break // stop at the first malformed part
				}
				child, cerr := parseMIMEEntity(part.Header, part)
				if cerr != nil {
					continue
				}
				node.children = append(node.children, child)
			}
		}
	}
	return node, nil
}

// currentMIMEPart returns the innermost foreverypart context, or nil when the
// evaluation is not inside any foreverypart loop.
func (d *RuntimeData) currentMIMEPart() *mimePart {
	if len(d.partStack) == 0 {
		return nil
	}
	return d.partStack[len(d.partStack)-1]
}

// mimeTree lazily parses and caches the message's MIME tree. Returns (nil, nil)
// when the Message does not expose raw bytes.
func (d *RuntimeData) mimeTree() (*mimePart, error) {
	if d.mimeTreeParsed {
		return d.mimeTreeCache, d.mimeTreeErr
	}
	d.mimeTreeParsed = true
	rm, ok := d.Msg.(RawMessager)
	if !ok {
		return nil, nil
	}
	raw := rm.RawMessageBytes()
	if raw == nil {
		return nil, nil
	}
	d.mimeTreeCache, d.mimeTreeErr = parseMIMETree(raw)
	return d.mimeTreeCache, d.mimeTreeErr
}

// CmdForEveryPart implements the foreverypart loop (RFC 5703 §3). It walks the
// MIME parts depth-first; a nested foreverypart iterates the descendants of the
// enclosing loop's current part.
type CmdForEveryPart struct {
	Name  string
	Block []Cmd
}

func (c CmdForEveryPart) Execute(ctx context.Context, d *RuntimeData) error {
	tree, err := d.mimeTree()
	if err != nil || tree == nil {
		return err
	}

	var set []*mimePart
	if len(d.partStack) == 0 {
		// Top-level loop: every part of the message, depth-first. A
		// single-part (non-multipart) message has exactly one part — itself.
		if tree.isMultipart() {
			set = tree.descendants()
		} else {
			set = []*mimePart{tree}
		}
	} else {
		// Nested loop: descendants of the enclosing loop's current part.
		// A terminal current part yields nothing, so the loop is a no-op.
		set = d.currentMIMEPart().descendants()
	}

	for _, p := range set {
		d.partStack = append(d.partStack, p)
		err := c.runBlock(ctx, d)
		d.partStack = d.partStack[:len(d.partStack)-1]
		if err != nil {
			var bs *breakSignal
			if errors.As(err, &bs) {
				if bs.name == "" || bs.name == c.Name {
					return nil // this loop is the target
				}
				return err // a named break for an outer loop — propagate
			}
			return err
		}
	}
	return nil
}

func (c CmdForEveryPart) runBlock(ctx context.Context, d *RuntimeData) error {
	for _, cmd := range c.Block {
		if err := cmd.Execute(ctx, d); err != nil {
			return err
		}
	}
	return nil
}

// CmdBreak implements the break command (RFC 5703 §3.3). It unwinds to the
// closest enclosing foreverypart loop, or the one whose :name matches.
type CmdBreak struct {
	Name string
}

func (c CmdBreak) Execute(_ context.Context, _ *RuntimeData) error {
	return &breakSignal{name: c.Name}
}

// breakSignal is the control-flow sentinel a break raises; CmdForEveryPart
// catches it. It escaping to Script.Execute means "break outside a loop".
type breakSignal struct {
	name string
}

func (b *breakSignal) Error() string { return "sieve: break outside a matching foreverypart loop" }

func loadForEveryPart(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdForEveryPart{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"name": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr:    func(v []string) { cmd.Name = v[0] },
			},
		},
		AddBlock: func(cmds []Cmd) { cmd.Block = cmds },
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}

func loadBreak(s *Script, pcmd parser.Cmd) (Cmd, error) {
	cmd := CmdBreak{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"name": {
				NeedsValue:  true,
				MinStrCount: 1,
				MatchStr:    func(v []string) { cmd.Name = v[0] },
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}

func init() {
	gob.Register(CmdForEveryPart{})
	gob.Register(CmdBreak{})
}
