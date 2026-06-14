package interp

import (
	"context"
	"strings"
)

// HeaderEdit records a single addheader or deleteheader operation.
type HeaderEdit struct {
	Add       bool
	FieldName string
	Value     string
	Last      bool
	Index     int
}

// protectedHeaders cannot be deleted per RFC 5293 §5.
var protectedHeaders = map[string]struct{}{
	"received":       {},
	"auto-submitted": {},
}

func isValidHeaderName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c < 33 || c > 126 || c == ':' {
			return false
		}
	}
	return true
}

// CmdAddHeader implements the RFC 5293 addheader command.
type CmdAddHeader struct {
	FieldName string
	Value     string
	Last      bool
}

func (c CmdAddHeader) Execute(_ context.Context, d *RuntimeData) error {
	fieldName := expandVars(d, c.FieldName)
	if !isValidHeaderName(fieldName) {
		return nil
	}
	d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
		Add:       true,
		FieldName: fieldName,
		Value:     expandVars(d, c.Value),
		Last:      c.Last,
	})
	return nil
}

// CmdDeleteHeader implements the RFC 5293 deleteheader command.
type CmdDeleteHeader struct {
	matcherTest
	FieldName     string
	ValuePatterns []string
	Index         int
	Last          bool
}

func (c CmdDeleteHeader) Execute(_ context.Context, d *RuntimeData) error {
	fieldName := expandVars(d, c.FieldName)
	if !isValidHeaderName(fieldName) {
		return nil
	}
	if _, protected := protectedHeaders[strings.ToLower(fieldName)]; protected {
		return nil
	}

	if len(c.ValuePatterns) == 0 {
		d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
			FieldName: fieldName,
			Index:     c.Index,
			Last:      c.Last,
		})
		return nil
	}

	values, err := d.Msg.HeaderGet(fieldName)
	if err != nil {
		return nil
	}
	values = applyHeaderEditsToValues(d, fieldName, values)

	if len(values) == 0 {
		return nil
	}

	if c.Index > 0 {
		idx := c.Index - 1
		if c.Last {
			idx = len(values) - c.Index
		}
		if idx < 0 || idx >= len(values) {
			return nil
		}
		ok, err := c.matcherTest.tryMatch(d, strings.TrimSpace(values[idx]))
		if err != nil || !ok {
			return nil
		}
		d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
			FieldName: fieldName,
			Index:     c.Index,
			Last:      c.Last,
		})
		return nil
	}

	for _, val := range values {
		ok, err := c.matcherTest.tryMatch(d, strings.TrimSpace(val))
		if err != nil {
			continue
		}
		if ok {
			d.HeaderEdits = append(d.HeaderEdits, HeaderEdit{
				FieldName: fieldName,
				Value:     val,
			})
		}
	}
	return nil
}

// applyHeaderEditsToValues replays recorded edits onto a header value slice.
func applyHeaderEditsToValues(d *RuntimeData, fieldName string, values []string) []string {
	if len(d.HeaderEdits) == 0 {
		return values
	}

	result := make([]string, len(values))
	copy(result, values)

	for _, edit := range d.HeaderEdits {
		if !strings.EqualFold(edit.FieldName, fieldName) {
			continue
		}
		if edit.Add {
			if edit.Last {
				result = append(result, edit.Value)
			} else {
				result = append([]string{edit.Value}, result...)
			}
			continue
		}
		// delete
		if edit.Index > 0 {
			idx := edit.Index - 1
			if edit.Last {
				idx = len(result) - edit.Index
			}
			if idx >= 0 && idx < len(result) {
				result = append(result[:idx], result[idx+1:]...)
			}
		} else if edit.Value != "" {
			out := result[:0:len(result)]
			deleted := false
			for _, v := range result {
				if !deleted && v == edit.Value {
					deleted = true
					continue
				}
				out = append(out, v)
			}
			result = out
		} else {
			result = nil
		}
	}

	return result
}
