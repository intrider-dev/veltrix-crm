package reporting

import (
	"os"
	"strings"
	"testing"
)

func TestReportingSQLUsesBoundedAggregates(t *testing.T) {
	t.Parallel()
	contents, err := os.ReadFile("../../queries/reporting_advanced.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(contents))
	for _, required := range []string{
		"workspace_id = sqlc.arg(workspace_id)",
		"updated_at >= sqlc.arg(period_start)",
		"occurred_at >= sqlc.arg(period_start)",
		"limit 366", "limit 500", "limit 200", "limit 100",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("reporting SQL is missing %q", required)
		}
	}
}
