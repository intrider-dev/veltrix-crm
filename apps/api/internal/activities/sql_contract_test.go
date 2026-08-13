package activities

import (
	"os"
	"strings"
	"testing"
)

func TestAdvancedActivitySQLStaysTenantScopedAndBounded(t *testing.T) {
	t.Parallel()
	contents, err := os.ReadFile("../../queries/activities_advanced.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(contents))
	for _, required := range []string{
		"workspace_id = sqlc.arg(workspace_id)",
		"for update of reminder",
		"limit sqlc.arg(page_limit)",
		"limit sqlc.arg(result_limit)",
		"on conflict (workspace_id, kind, idempotency_key)",
	} {
		if !strings.Contains(sql, required) {
			t.Errorf("activity SQL is missing %q", required)
		}
	}
}
