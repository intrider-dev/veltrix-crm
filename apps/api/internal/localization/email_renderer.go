package localization

import (
	"errors"
	"fmt"
	"strings"
)

// CatalogEmailRenderer renders security-sensitive built-in email templates
// from the same checked RU/EN catalogs as API messages. Tenant-editable
// content uses ContentService instead and cannot override authentication mail.
type CatalogEmailRenderer struct {
	ProductName string
}

func (renderer CatalogEmailRenderer) RenderEmail(
	templateKey, locale string,
	params map[string]string,
) (string, string, error) {
	if templateKey != "email.passwordReset" {
		return "", "", fmt.Errorf("unsupported email template %q", templateKey)
	}
	values := make(map[string]any, len(params)+1)
	for key, value := range params {
		values[key] = value
	}
	values["productName"] = renderer.ProductName
	subject := Translate(locale, templateKey+".subject", values)
	body := Translate(locale, templateKey+".body", values)
	if subject == templateKey+".subject" || body == templateKey+".body" {
		return "", "", errors.New("email template catalog entry is missing")
	}
	if strings.ContainsAny(subject, "\r\n") {
		return "", "", errors.New("rendered email subject contains a line break")
	}
	for _, rendered := range []string{subject, body} {
		remaining, err := ExtractPlaceholders(rendered)
		if err != nil || len(remaining) > 0 {
			return "", "", errors.New("email template has unresolved placeholders")
		}
	}
	return subject, body, nil
}
