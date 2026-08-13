package dbgen

import (
	"strings"
	"testing"
)

func TestCompanyPageQueryKeepsFiltersAndStableCursorOrdering(t *testing.T) {
	for _, fragment := range []string{
		"domain_normalized LIKE",
		"status = $3",
		"(updated_at, id) < ($4::timestamptz, $5::uuid)",
		"ORDER BY updated_at DESC, id DESC",
	} {
		if !strings.Contains(listCompaniesPage, fragment) {
			t.Errorf("ListCompaniesPage is missing %q", fragment)
		}
	}
}
