package interp

import (
	"bytes"
	"context"
	"encoding/gob"

	"github.com/emersion/go-message/textproto"

	"github.com/foxcpp/go-sieve/parser"
)

func init() {
	gob.Register(CmdReplace{})
	gob.Register(CmdEnclose{})
}

const mimeBoundary = "yarilo-mime-boundary"

// serializeMIME reconstructs a MIME part tree into raw RFC 2822 bytes. Header
// fields keep their original folding; multipart bodies are rebuilt from the
// child parts using the part's boundary.
func serializeMIME(p *mimePart) []byte {
	var buf bytes.Buffer
	if hdr, err := serializeHeader(p.header); err == nil {
		buf.WriteString(hdr)
	}
	buf.WriteString("\r\n")
	if len(p.children) > 0 {
		boundary := p.params["boundary"]
		if boundary == "" {
			boundary = mimeBoundary
		}
		for _, c := range p.children {
			buf.WriteString("--")
			buf.WriteString(boundary)
			buf.WriteString("\r\n")
			buf.Write(serializeMIME(c))
			buf.WriteString("\r\n")
		}
		buf.WriteString("--")
		buf.WriteString(boundary)
		buf.WriteString("--\r\n")
	} else {
		buf.Write(p.body)
	}
	return buf.Bytes()
}

// setPlainText turns a part into a text/plain UTF-8 part with the given body,
// preserving all non-content header fields (From/To/Subject/… on the message).
func setPlainText(p *mimePart, body string) {
	p.header.Del("Content-Type")
	p.header.Del("Content-Transfer-Encoding")
	p.header.Set("Content-Type", "text/plain; charset=utf-8")
	p.contentType = "text/plain"
	p.typ = "text"
	p.subtype = "plain"
	p.params = map[string]string{"charset": "utf-8"}
	p.body = []byte(body)
	p.children = nil
}

// parseReplacementEntity parses a :mime replacement (a full MIME entity —
// headers and content) into a part. Malformed input falls back to text/plain.
func parseReplacementEntity(s string) *mimePart {
	p, err := parseMIMETree([]byte(s))
	if err != nil || p == nil {
		fallback := &mimePart{header: textproto.Header{}}
		setPlainText(fallback, s)
		return fallback
	}
	return p
}

// CmdReplace implements the replace action (RFC 5703 §6).
type CmdReplace struct {
	Mime        bool
	Subject     string
	HasSubject  bool
	From        string
	HasFrom     bool
	Replacement string
}

func (c CmdReplace) Execute(_ context.Context, d *RuntimeData) error {
	root, _ := d.mimeTree()
	if root == nil {
		return nil
	}
	replacement := expandVars(d, c.Replacement)
	target := d.currentMIMEPart()

	if target == nil {
		// Outside foreverypart: replace the entire message.
		if c.Mime {
			d.mimeTreeCache = parseReplacementEntity(replacement)
		} else {
			setPlainText(root, replacement)
		}
		c.applySubjectFrom(d, d.mimeTreeCache)
	} else {
		// Inside foreverypart: replace the current part in place.
		if c.Mime {
			*target = *parseReplacementEntity(replacement)
		} else {
			setPlainText(target, replacement)
		}
	}

	d.FilteredMessage = serializeMIME(d.mimeTreeCache)
	return nil
}

// applySubjectFrom sets the :subject / :from headers on the message, preserving
// the originals as Original-Subject / Original-From. Ignored with :mime.
func (c CmdReplace) applySubjectFrom(d *RuntimeData, root *mimePart) {
	if c.Mime {
		return
	}
	if c.HasSubject {
		if old := root.header.Get("Subject"); old != "" {
			root.header.Set("Original-Subject", old)
		}
		root.header.Set("Subject", expandVars(d, c.Subject))
	}
	if c.HasFrom {
		if old := root.header.Get("From"); old != "" {
			root.header.Set("Original-From", old)
		}
		root.header.Set("From", expandVars(d, c.From))
	}
}

func loadReplace(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("replace") {
		return nil, parser.ErrorAt(pcmd.Position, `missing require "replace"`)
	}
	cmd := CmdReplace{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"mime": {MatchBool: func() { cmd.Mime = true }},
			"subject": {
				NeedsValue: true, MinStrCount: 1, MaxStrCount: 1,
				MatchStr: func(v []string) { cmd.Subject = v[0]; cmd.HasSubject = true },
			},
			"from": {
				NeedsValue: true, MinStrCount: 1, MaxStrCount: 1,
				MatchStr: func(v []string) { cmd.From = v[0]; cmd.HasFrom = true },
			},
		},
		Pos: []SpecPosArg{
			{MinStrCount: 1, MaxStrCount: 1, MatchStr: func(v []string) { cmd.Replacement = v[0] }},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}
	if cmd.Mime && (cmd.HasSubject || cmd.HasFrom) {
		return nil, parser.ErrorAt(pcmd.Position, "replace :mime ignores :subject/:from — remove them")
	}
	return cmd, nil
}

// CmdEnclose implements the enclose action (RFC 5703 §7): it wraps the whole
// message in a multipart/mixed with a text/plain cover and the original as a
// message/rfc822 part.
type CmdEnclose struct {
	Subject    string
	HasSubject bool
	Headers    []string
	Body       string
}

func (c CmdEnclose) Execute(_ context.Context, d *RuntimeData) error {
	root, _ := d.mimeTree()
	if root == nil {
		return nil
	}
	original := serializeMIME(root)

	cover := &mimePart{header: textproto.Header{}}
	setPlainText(cover, expandVars(d, c.Body))

	enclosed := &mimePart{header: textproto.Header{}, contentType: "message/rfc822", typ: "message", subtype: "rfc822", body: original}
	enclosed.header.Set("Content-Type", "message/rfc822")

	newRoot := &mimePart{
		header:      textproto.Header{},
		contentType: "multipart/mixed",
		typ:         "multipart",
		subtype:     "mixed",
		params:      map[string]string{"boundary": mimeBoundary},
		children:    []*mimePart{cover, enclosed},
	}
	newRoot.header.Set("Content-Type", `multipart/mixed; boundary="`+mimeBoundary+`"`)
	newRoot.header.Set("MIME-Version", "1.0")

	// Copy the requested headers from the original message; always carry the
	// envelope-relevant fields so the wrapped message stays deliverable.
	for _, h := range c.Headers {
		if v := root.header.Get(h); v != "" {
			newRoot.header.Set(h, v)
		}
	}
	for _, h := range []string{"From", "To", "Date"} {
		if newRoot.header.Get(h) == "" {
			if v := root.header.Get(h); v != "" {
				newRoot.header.Set(h, v)
			}
		}
	}
	if c.HasSubject {
		newRoot.header.Set("Subject", expandVars(d, c.Subject))
	} else if s := root.header.Get("Subject"); s != "" {
		newRoot.header.Set("Subject", s)
	}

	d.mimeTreeCache = newRoot
	d.FilteredMessage = serializeMIME(newRoot)
	return nil
}

func loadEnclose(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("enclose") {
		return nil, parser.ErrorAt(pcmd.Position, `missing require "enclose"`)
	}
	cmd := CmdEnclose{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"subject": {
				NeedsValue: true, MinStrCount: 1, MaxStrCount: 1,
				MatchStr: func(v []string) { cmd.Subject = v[0]; cmd.HasSubject = true },
			},
			"headers": {
				NeedsValue: true, MinStrCount: 1,
				MatchStr: func(v []string) { cmd.Headers = v },
			},
		},
		Pos: []SpecPosArg{
			{MinStrCount: 1, MaxStrCount: 1, MatchStr: func(v []string) { cmd.Body = v[0] }},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	return cmd, err
}
