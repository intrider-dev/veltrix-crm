package notifications

import "testing"

func TestCatalogRendererRendersRecipientLocale(t *testing.T) {
	t.Parallel()
	renderer := CatalogRenderer{ProductName: "CRM"}
	subject, body, err := renderer.Render("ru", "Продажи", "notifications.activity.reminder", map[string]any{
		"title": "Связаться с клиентом",
	})
	if err != nil {
		t.Fatal(err)
	}
	if subject != "Продажи: уведомление CRM" {
		t.Fatalf("subject = %q", subject)
	}
	if body == "" || body == "notifications.email.body" {
		t.Fatalf("body = %q", body)
	}
}

func TestCatalogRendererRejectsUnresolvedParameters(t *testing.T) {
	t.Parallel()
	renderer := CatalogRenderer{ProductName: "CRM"}
	if _, _, err := renderer.Render("en", "Sales", "notifications.comment.mention", nil); err == nil {
		t.Fatal("renderer accepted unresolved mention parameters")
	}
}
