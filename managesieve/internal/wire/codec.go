package wire

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/foxcpp/go-sieve/managesieve"
)

type Encoder struct {
	w       *bufio.Writer
	err     error
	literal bool
}

func NewEncoder(w *bufio.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) setErr(err error) {
	if e.err == nil {
		e.err = err
	}
}

func (e *Encoder) writeString(s string) *Encoder {
	if e.err != nil {
		return e
	}
	if e.literal {
		e.setErr(fmt.Errorf("wire: cannot write while literal is open"))
		return e
	}
	_, err := e.w.WriteString(s)
	e.setErr(err)
	return e
}

func (e *Encoder) Atom(s string) *Encoder {
	return e.writeString(s)
}

func (e *Encoder) SP() *Encoder {
	return e.writeString(" ")
}

func (e *Encoder) Quoted(s string) *Encoder {
	if e.err != nil {
		return e
	}
	var b strings.Builder
	b.Grow(2 + len(s))
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' || ch == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(ch)
	}
	b.WriteByte('"')
	return e.writeString(b.String())
}

func (e *Encoder) StringClient(s string) *Encoder {
	return e.string(s, true)
}

func (e *Encoder) StringServer(s string) *Encoder {
	return e.string(s, false)
}

func (e *Encoder) string(s string, plus bool) *Encoder {
	if validQuoted(s) {
		return e.Quoted(s)
	}
	wc := e.Literal(int64(len(s)), plus)
	_, werr := io.WriteString(wc, s)
	cerr := wc.Close()
	if werr != nil {
		e.setErr(werr)
	}
	if cerr != nil {
		e.setErr(cerr)
	}
	return e
}

func (e *Encoder) Literal(size int64, plus bool) io.WriteCloser {
	e.writeString("{")
	e.writeString(strconv.FormatInt(size, 10))
	if plus {
		e.writeString("+")
	}
	e.writeString("}\r\n")
	if e.err != nil {
		return errorWriter{err: e.err}
	}
	if err := e.w.Flush(); err != nil {
		e.setErr(err)
		return errorWriter{err: err}
	}

	e.literal = true
	return &literalWriter{enc: e, remain: size}
}

func (e *Encoder) CRLF() error {
	e.writeString("\r\n")
	if e.err != nil {
		return e.err
	}
	return e.w.Flush()
}

type errorWriter struct {
	err error
}

func (w errorWriter) Write(_ []byte) (int, error) {
	return 0, w.err
}

func (w errorWriter) Close() error {
	return w.err
}

type literalWriter struct {
	enc    *Encoder
	remain int64
}

func (w *literalWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remain {
		return 0, fmt.Errorf("wire: wrote too many bytes in literal")
	}
	n, err := w.enc.w.Write(p)
	w.remain -= int64(n)
	return n, err
}

func (w *literalWriter) Close() error {
	w.enc.literal = false
	if w.remain != 0 {
		return fmt.Errorf("wire: wrote too few bytes in literal (%d remaining)", w.remain)
	}
	return nil
}

type Decoder struct {
	r *bufio.Reader

	MaxLiteralBytes         int64
	MaxAtomBytes            int
	MaxQuotedBytes          int
	MaxCommandArgs          int
	MaxNonLiteralCommandLen int64
}

func NewDecoder(r *bufio.Reader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) PeekByte() (byte, error) {
	b, err := d.r.Peek(1)
	if err != nil {
		return 0, err
	}
	return b[0], nil
}

func (d *Decoder) ExpectCRLF() error {
	b, err := d.r.ReadByte()
	if err != nil {
		return err
	}
	if b == '\r' {
		b, err = d.r.ReadByte()
		if err != nil {
			return err
		}
	}
	if b != '\n' {
		return fmt.Errorf("wire: expected CRLF")
	}
	return nil
}

func (d *Decoder) SkipSP() error {
	for {
		b, err := d.PeekByte()
		if err != nil {
			return err
		}
		if b != ' ' {
			return nil
		}
		_, _ = d.r.ReadByte()
	}
}

func (d *Decoder) ReadAtom() (string, error) {
	return d.readAtom(nil)
}

func (d *Decoder) readAtom(commandBytes *int64) (string, error) {
	if err := d.SkipSP(); err != nil {
		return "", err
	}
	var b strings.Builder
	for {
		ch, err := d.PeekByte()
		if err != nil {
			if err == io.EOF && b.Len() > 0 {
				return b.String(), nil
			}
			return "", err
		}
		if !isAtomChar(ch) {
			break
		}
		if d.MaxAtomBytes > 0 && b.Len() >= d.MaxAtomBytes {
			return "", fmt.Errorf("wire: atom exceeds limit %d", d.MaxAtomBytes)
		}
		_, _ = d.r.ReadByte()
		b.WriteByte(ch)
		if err := d.addCommandBytes(commandBytes, 1); err != nil {
			return "", err
		}
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("wire: expected atom")
	}
	return b.String(), nil
}

func (d *Decoder) ReadString() (string, error) {
	return d.readString(nil)
}

func (d *Decoder) readString(commandBytes *int64) (string, error) {
	if err := d.SkipSP(); err != nil {
		return "", err
	}
	b, err := d.PeekByte()
	if err != nil {
		return "", err
	}
	if b == '"' {
		return d.readQuoted(commandBytes)
	}
	if b == '{' {
		return d.readLiteral(commandBytes)
	}
	return "", fmt.Errorf("wire: expected string")
}

func (d *Decoder) readQuoted(commandBytes *int64) (string, error) {
	_, _ = d.r.ReadByte()
	var b strings.Builder
	for {
		ch, err := d.r.ReadByte()
		if err != nil {
			return "", err
		}
		if ch == '"' {
			return b.String(), nil
		}
		if ch == '\\' {
			ch, err = d.r.ReadByte()
			if err != nil {
				return "", err
			}
		}
		if d.MaxQuotedBytes > 0 && b.Len() >= d.MaxQuotedBytes {
			return "", fmt.Errorf("wire: quoted string exceeds limit %d", d.MaxQuotedBytes)
		}
		b.WriteByte(ch)
		if err := d.addCommandBytes(commandBytes, 1); err != nil {
			return "", err
		}
	}
}

func (d *Decoder) readLiteral(commandBytes *int64) (string, error) {
	_, _ = d.r.ReadByte()
	var nbuf strings.Builder
	for {
		ch, err := d.r.ReadByte()
		if err != nil {
			return "", err
		}
		if ch >= '0' && ch <= '9' {
			nbuf.WriteByte(ch)
			continue
		}
		if ch == '+' {
			ch, err = d.r.ReadByte()
			if err != nil {
				return "", err
			}
		}
		if ch != '}' {
			return "", fmt.Errorf("wire: malformed literal")
		}
		break
	}
	if nbuf.Len() == 0 {
		return "", fmt.Errorf("wire: malformed literal size")
	}

	sz, err := strconv.ParseInt(nbuf.String(), 10, 64)
	if err != nil {
		return "", err
	}
	if sz < 0 || sz > math.MaxUint32 {
		return "", fmt.Errorf("wire: invalid literal size %d", sz)
	}
	if d.MaxLiteralBytes > 0 && sz > d.MaxLiteralBytes {
		return "", fmt.Errorf("wire: literal size %d exceeds limit %d", sz, d.MaxLiteralBytes)
	}
	if sz > maxAllocBytes() {
		return "", fmt.Errorf("wire: literal size %d cannot be allocated", sz)
	}
	if err := d.addCommandBytes(commandBytes, sz); err != nil {
		return "", err
	}
	if err := d.ExpectCRLF(); err != nil {
		return "", err
	}

	buf := make([]byte, sz)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func (d *Decoder) ReadToken() (string, error) {
	return d.readToken(nil)
}

func (d *Decoder) readToken(commandBytes *int64) (string, error) {
	if err := d.SkipSP(); err != nil {
		return "", err
	}
	b, err := d.PeekByte()
	if err != nil {
		return "", err
	}
	if b == '"' || b == '{' {
		return d.readString(commandBytes)
	}
	return d.readAtom(commandBytes)
}

func (d *Decoder) ReadCommand() (string, []string, error) {
	var commandBytes int64
	name, err := d.readAtom(&commandBytes)
	if err != nil {
		return "", nil, err
	}

	args := make([]string, 0, 4)
	for {
		b, err := d.PeekByte()
		if err != nil {
			return "", nil, err
		}
		if b == '\r' || b == '\n' {
			if err := d.ExpectCRLF(); err != nil {
				return "", nil, err
			}
			return strings.ToUpper(name), args, nil
		}
		if d.MaxCommandArgs > 0 && len(args) >= d.MaxCommandArgs {
			return "", nil, fmt.Errorf("wire: command has too many arguments")
		}
		arg, err := d.readToken(&commandBytes)
		if err != nil {
			return "", nil, err
		}
		args = append(args, arg)
	}
}

func (d *Decoder) addCommandBytes(commandBytes *int64, n int64) error {
	if commandBytes == nil {
		return nil
	}
	*commandBytes += n
	if d.MaxNonLiteralCommandLen > 0 && *commandBytes > d.MaxNonLiteralCommandLen {
		return fmt.Errorf("wire: command exceeds limit %d", d.MaxNonLiteralCommandLen)
	}
	return nil
}

func (d *Decoder) ReadStatusLine() (managesieve.StatusResponse, error) {
	var out managesieve.StatusResponse
	name, err := d.ReadAtom()
	if err != nil {
		return out, err
	}
	out.Type = managesieve.StatusType(strings.ToUpper(name))
	if out.Type != managesieve.StatusOK && out.Type != managesieve.StatusNO && out.Type != managesieve.StatusBYE {
		return out, fmt.Errorf("wire: invalid status type %q", name)
	}

	b, err := d.PeekByte()
	if err != nil {
		return out, err
	}
	if b == ' ' {
		if err := d.SkipSP(); err != nil {
			return out, err
		}
		b, err = d.PeekByte()
		if err != nil {
			return out, err
		}
		if b == '(' {
			_, _ = d.r.ReadByte()
			codeName, err := d.ReadAtom()
			if err != nil {
				return out, err
			}
			out.Code = managesieve.ResponseCode(strings.ToUpper(codeName))
			for {
				if err := d.SkipSP(); err != nil {
					return out, err
				}
				pb, err := d.PeekByte()
				if err != nil {
					return out, err
				}
				if pb == ')' {
					_, _ = d.r.ReadByte()
					break
				}
				arg, err := d.ReadToken()
				if err != nil {
					return out, err
				}
				out.CodeArgs = append(out.CodeArgs, arg)
			}
			if err := d.SkipSP(); err != nil {
				if err == io.EOF {
					return out, nil
				}
				return out, err
			}
		}

		pb, err := d.PeekByte()
		if err != nil {
			return out, err
		}
		if pb != '\r' && pb != '\n' {
			text, err := d.ReadToken()
			if err != nil {
				return out, err
			}
			out.Text = text
		}
	}

	if err := d.ExpectCRLF(); err != nil {
		return out, err
	}
	return out, nil
}

func (d *Decoder) ReadCapabilityOrStatus() (*managesieve.CapabilityItem, *managesieve.StatusResponse, error) {
	b, err := d.PeekByte()
	if err != nil {
		return nil, nil, err
	}
	if b == '"' || b == '{' {
		name, err := d.ReadString()
		if err != nil {
			return nil, nil, err
		}
		item := &managesieve.CapabilityItem{Name: name}

		pb, err := d.PeekByte()
		if err != nil {
			return nil, nil, err
		}
		if pb == ' ' {
			val, err := d.ReadString()
			if err != nil {
				return nil, nil, err
			}
			item.Value = &val
		}
		if err := d.ExpectCRLF(); err != nil {
			return nil, nil, err
		}
		return item, nil, nil
	}

	status, err := d.ReadStatusLine()
	if err != nil {
		return nil, nil, err
	}
	return nil, &status, nil
}

func (d *Decoder) ReadScriptOrStatus() (*managesieve.ScriptInfo, *managesieve.StatusResponse, error) {
	b, err := d.PeekByte()
	if err != nil {
		return nil, nil, err
	}
	if b == '"' || b == '{' {
		name, err := d.ReadString()
		if err != nil {
			return nil, nil, err
		}
		info := &managesieve.ScriptInfo{Name: name}
		pb, err := d.PeekByte()
		if err != nil {
			return nil, nil, err
		}
		if pb == ' ' {
			tok, err := d.ReadAtom()
			if err != nil {
				return nil, nil, err
			}
			if strings.EqualFold(tok, "ACTIVE") {
				info.Active = true
			}
		}
		if err := d.ExpectCRLF(); err != nil {
			return nil, nil, err
		}
		return info, nil, nil
	}

	status, err := d.ReadStatusLine()
	if err != nil {
		return nil, nil, err
	}
	return nil, &status, nil
}

func isAtomChar(ch byte) bool {
	if ch <= 0x20 || ch == 0x7F {
		return false
	}
	switch ch {
	case '(', ')', '{', ' ', '"', '\\':
		return false
	default:
		return true
	}
}

func validQuoted(s string) bool {
	if len(s) > 1024 {
		return false
	}
	for _, r := range s {
		if r == 0 || r == '\r' || r == '\n' {
			return false
		}
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func maxAllocBytes() int64 {
	maxInt := int64(^uint(0) >> 1)
	return maxInt
}
