package app

import "testing"

func TestWebhookDeliveryStatusViewMatchesOpenAPI(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"queued":     "pending",
		"delivering": "delivering",
		"succeeded":  "delivered",
		"failed":     "retrying",
		"dead":       "dead",
	}
	for databaseStatus, apiStatus := range tests {
		databaseStatus, apiStatus := databaseStatus, apiStatus
		t.Run(databaseStatus, func(t *testing.T) {
			t.Parallel()
			if actual := webhookDeliveryStatusView(databaseStatus); actual != apiStatus {
				t.Fatalf("webhookDeliveryStatusView(%q) = %q, want %q", databaseStatus, actual, apiStatus)
			}
		})
	}
}
