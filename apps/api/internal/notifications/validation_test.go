package notifications

import (
	"errors"
	"strings"
	"testing"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

func TestValidateInputDefaultsVersionAndDelivery(t *testing.T) {
	t.Parallel()
	input, encoded, err := validateInput(Input{MessageKey: "notifications.activity.reminder"})
	if err != nil {
		t.Fatal(err)
	}
	if input.TemplateVersion != 1 || input.Delivery != DeliveryInApp || string(encoded) != "{}" {
		t.Fatalf("validated input = %+v, params = %s", input, encoded)
	}
}

func TestValidateInputRejectsHeaderLikeKeyAndOversizeParams(t *testing.T) {
	t.Parallel()
	_, _, err := validateInput(Input{
		MessageKey: "ok\r\nBcc:bad", MessageParams: map[string]any{"value": strings.Repeat("x", 33_000)},
	})
	var validation *errx.ValidationError
	if !errors.As(err, &validation) || len(validation.Fields) != 2 {
		t.Fatalf("error = %#v", err)
	}
}
