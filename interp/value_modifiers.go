package interp

import (
	"strconv"
	"strings"

	"rsc.io/binaryregexp"
)

func modQuoteWildcard(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, chr := range s {
		switch chr {
		case '\\', '*', '?':
			b.WriteByte('\\')
			b.WriteRune(chr)
		default:
			b.WriteRune(chr)
		}
	}
	return b.String()
}

func modUpperFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	first := s[0]
	if first >= 'a' && first <= 'z' {
		first -= 'a' - 'A'
	}
	return string(first) + s[1:]
}

func modLowerFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	first := s[0]
	if first >= 'A' && first <= 'Z' {
		first += 'a' - 'A'
	}
	return string(first) + s[1:]
}

// valueModifierTags returns the RFC 5229 §5 value-modifier tags (:lower, :upper,
// :length, :quotewildcard, …) shared by the `set` command and, via RFC 5703 §5,
// `extracttext`. Modifiers accumulate into the precedence-keyed map; a second
// modifier at the same precedence sets *conflicting.
func valueModifierTags(modifiers map[int]func(string) string, conflicting *bool) map[string]SpecTag {
	set := func(prec int, fn func(string) string) {
		if modifiers[prec] != nil {
			*conflicting = true
		}
		modifiers[prec] = fn
	}
	return map[string]SpecTag{
		"length":        {MatchBool: func() { set(10, func(s string) string { return strconv.Itoa(len([]rune(s))) }) }},
		"quotewildcard": {MatchBool: func() { set(20, modQuoteWildcard) }},
		"quoteregex":    {MatchBool: func() { set(20, binaryregexp.QuoteMeta) }},
		"encodeurl":     {MatchBool: func() { set(15, percentEncode) }},
		"upperfirst":    {MatchBool: func() { set(30, modUpperFirst) }},
		"lowerfirst":    {MatchBool: func() { set(30, modLowerFirst) }},
		"upper":         {MatchBool: func() { set(40, strings.ToUpper) }},
		"lower":         {MatchBool: func() { set(40, strings.ToLower) }},
	}
}

// buildValueModifier composes the accumulated modifiers by descending
// precedence, applying the quotewildcard truncation guard afterwards.
func buildValueModifier(script *Script, modifiers map[int]func(string) string) func(string) string {
	return func(s string) string {
		lastPrec := 9999
		for _, prec := range [5]int{40, 30, 20, 15, 10} {
			if fun := modifiers[prec]; fun != nil {
				s = fun(s)
				lastPrec = prec
			}
		}
		if lastPrec == 20 && len(s) > script.opts.MaxVariableLen {
			until := script.opts.MaxVariableLen
			for until > 0 && s[until] >= 128 && s[until] < 192 {
				until--
			}
			if until > 0 && s[until-1] == '\\' {
				until--
			}
			s = s[:until]
		}
		return s
	}
}
