package interp

import (
	"context"
	"strings"
)

// ExternalListChecker provides access to externally stored lists (RFC 6134).
// Implementations attach it to d.Policy.
type ExternalListChecker interface {
	// ListContains returns true if value is a member of the named external list.
	ListContains(ctx context.Context, listName, value string) (bool, error)
	// ListAddresses returns all email addresses in the named list, for redirect :list.
	// Returns nil, nil if the list cannot be enumerated (MTA will handle natively).
	ListAddresses(ctx context.Context, listName string) ([]string, error)
	// ListValid returns true if the named list is valid and accessible.
	ListValid(ctx context.Context, listName string) bool
}

// expandListName expands the shorthand notation defined in RFC 6134 §2.5.
// Names starting with ":" are shorthand for "urn:ietf:params:sieve:<rest>".
func expandListName(name string) string {
	if strings.HasPrefix(name, ":") {
		return "urn:ietf:params:sieve" + name
	}
	return name
}

// TestValidExtList implements the valid_ext_list test (RFC 6134 §2.7).
type TestValidExtList struct {
	Names []string
}

func (t TestValidExtList) Check(ctx context.Context, d *RuntimeData) (bool, error) {
	checker, ok := d.Policy.(ExternalListChecker)
	if !ok {
		return false, nil
	}
	for _, name := range t.Names {
		name = expandListName(expandVars(d, name))
		if !checker.ListValid(ctx, name) {
			return false, nil
		}
	}
	return true, nil
}
