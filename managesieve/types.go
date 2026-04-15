package managesieve

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ConnState int

const (
	ConnStateNotAuthenticated ConnState = iota
	ConnStateAuthenticated
	ConnStateLogout
)

func (s ConnState) String() string {
	switch s {
	case ConnStateNotAuthenticated:
		return "not-authenticated"
	case ConnStateAuthenticated:
		return "authenticated"
	case ConnStateLogout:
		return "logout"
	default:
		return fmt.Sprintf("state(%d)", s)
	}
}

type StatusType string

const (
	StatusOK  StatusType = "OK"
	StatusNO  StatusType = "NO"
	StatusBYE StatusType = "BYE"
)

type ResponseCode string

const (
	ResponseCodeAuthTooWeak    ResponseCode = "AUTH-TOO-WEAK"
	ResponseCodeEncryptNeeded  ResponseCode = "ENCRYPT-NEEDED"
	ResponseCodeQuota          ResponseCode = "QUOTA"
	ResponseCodeReferral       ResponseCode = "REFERRAL"
	ResponseCodeSASL           ResponseCode = "SASL"
	ResponseCodeTransitionNeed ResponseCode = "TRANSITION-NEEDED"
	ResponseCodeTryLater       ResponseCode = "TRYLATER"
	ResponseCodeActive         ResponseCode = "ACTIVE"
	ResponseCodeNonexistent    ResponseCode = "NONEXISTENT"
	ResponseCodeAlreadyExists  ResponseCode = "ALREADYEXISTS"
	ResponseCodeTag            ResponseCode = "TAG"
	ResponseCodeWarnings       ResponseCode = "WARNINGS"
)

type StatusResponse struct {
	Type     StatusType
	Code     ResponseCode
	CodeArgs []string
	Text     string
}

type Error StatusResponse

func (e *Error) Error() string {
	if e == nil {
		return "managesieve: <nil>"
	}
	var b strings.Builder
	b.WriteString("managesieve: ")
	b.WriteString(string(e.Type))
	if e.Code != "" {
		b.WriteString(" (")
		b.WriteString(string(e.Code))
		if len(e.CodeArgs) > 0 {
			b.WriteByte(' ')
			b.WriteString(strings.Join(e.CodeArgs, " "))
		}
		b.WriteByte(')')
	}
	if e.Text != "" {
		b.WriteByte(' ')
		b.WriteString(e.Text)
	}
	return b.String()
}

type Capability string

const (
	CapImplementation Capability = "IMPLEMENTATION"
	CapSASL           Capability = "SASL"
	CapSieve          Capability = "SIEVE"
	CapStartTLS       Capability = "STARTTLS"
	CapMaxRedirects   Capability = "MAXREDIRECTS"
	CapNotify         Capability = "NOTIFY"
	CapLanguage       Capability = "LANGUAGE"
	CapOwner          Capability = "OWNER"
	CapVersion        Capability = "VERSION"
	CapUnauthenticate Capability = "UNAUTHENTICATE"
)

type CapSet map[Capability]struct{}

func (set CapSet) Has(cap Capability) bool {
	_, ok := set[cap]
	return ok
}

func (set CapSet) Copy() CapSet {
	out := make(CapSet, len(set))
	for k := range set {
		out[k] = struct{}{}
	}
	return out
}

func (set CapSet) Slice() []Capability {
	out := make([]Capability, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

type CapabilityItem struct {
	Name  string
	Value *string
}

type Capabilities struct {
	Items map[string]*string
	Set   CapSet

	Implementation string
	SASL           []string
	Sieve          []string
	StartTLS       bool
	MaxRedirects   *uint32
	Notify         []string
	Language       string
	Owner          string
	Version        string
}

func NewCapabilities(items []CapabilityItem) Capabilities {
	out := Capabilities{
		Items: make(map[string]*string, len(items)),
		Set:   make(CapSet, len(items)),
	}

	for _, item := range items {
		name := strings.ToUpper(item.Name)
		out.Set[Capability(name)] = struct{}{}
		out.Items[name] = item.Value

		switch Capability(name) {
		case CapImplementation:
			if item.Value != nil {
				out.Implementation = *item.Value
			}
		case CapSASL:
			if item.Value != nil {
				out.SASL = splitWS(*item.Value)
			}
		case CapSieve:
			if item.Value != nil {
				out.Sieve = splitWS(*item.Value)
			}
		case CapStartTLS:
			out.StartTLS = true
		case CapMaxRedirects:
			if item.Value != nil {
				if n, ok := parseUint32(*item.Value); ok {
					out.MaxRedirects = &n
				}
			}
		case CapNotify:
			if item.Value != nil {
				out.Notify = splitWS(*item.Value)
			}
		case CapLanguage:
			if item.Value != nil {
				out.Language = *item.Value
			}
		case CapOwner:
			if item.Value != nil {
				out.Owner = *item.Value
			}
		case CapVersion:
			if item.Value != nil {
				out.Version = *item.Value
			}
		}
	}

	return out
}

func (c Capabilities) Value(name Capability) (string, bool) {
	v, ok := c.Items[strings.ToUpper(string(name))]
	if !ok || v == nil {
		return "", false
	}
	return *v, true
}

func splitWS(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return strings.Fields(v)
}

func parseUint32(v string) (uint32, bool) {
	n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

type ScriptInfo struct {
	Name   string
	Active bool
}
