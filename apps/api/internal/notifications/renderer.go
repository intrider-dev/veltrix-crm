package notifications

import (
	"errors"
	"fmt"
	"strings"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/localization"
)

type CatalogRenderer struct {
	ProductName string
}

func (renderer CatalogRenderer) Render(
	locale, workspaceName, messageKey string,
	params map[string]any,
) (string, string, error) {
	if messageKey != "notifications.activity.reminder" &&
		messageKey != "notifications.comment.mention" {
		return "", "", fmt.Errorf("unsupported notification template %q", messageKey)
	}
	message := localization.Translate(locale, messageKey, params)
	if message == messageKey {
		return "", "", errors.New("notification template catalog entry is missing")
	}
	values := map[string]any{
		"message":       message,
		"productName":   renderer.ProductName,
		"workspaceName": workspaceName,
	}
	subject := localization.Translate(locale, "notifications.email.subject", values)
	body := localization.Translate(locale, "notifications.email.body", values)
	if subject == "notifications.email.subject" || body == "notifications.email.body" {
		return "", "", errors.New("notification email catalog entry is missing")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", "", errors.New("notification subject contains a line break")
	}
	for _, rendered := range []string{message, subject, body} {
		remaining, err := localization.ExtractPlaceholders(rendered)
		if err != nil || len(remaining) > 0 {
			return "", "", errors.New("notification template has unresolved placeholders")
		}
	}
	return subject, body, nil
}
