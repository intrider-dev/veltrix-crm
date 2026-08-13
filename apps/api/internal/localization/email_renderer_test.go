package localization

import (
	"strings"
	"testing"
)

func TestCatalogEmailRendererUsesRequestedLocale(t *testing.T) {
	t.Parallel()
	renderer := CatalogEmailRenderer{ProductName: "Example CRM"}
	subject, body, err := renderer.RenderEmail("email.passwordReset", "ru", map[string]string{
		"resetUrl": "https://example.invalid/reset?token=opaque", "expiresMinutes": "60",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(subject, "Сброс") || !strings.Contains(body, "60") || !strings.Contains(body, "Example CRM") {
		t.Fatalf("unexpected localized message: %q / %q", subject, body)
	}
}

func TestCatalogEmailRendererRejectsUnknownTemplate(t *testing.T) {
	t.Parallel()
	if _, _, err := (CatalogEmailRenderer{}).RenderEmail("email.unknown", "en", nil); err == nil {
		t.Fatal("unknown security email template must be rejected")
	}
}
