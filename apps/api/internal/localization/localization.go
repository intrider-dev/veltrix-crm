package localization

import (
	"fmt"
	"strings"
)

const fallbackLocale = "en"

func Resolve(preferred string, acceptLanguage string) string {
	if isSupported(preferred) {
		return preferred
	}
	for _, candidate := range strings.Split(acceptLanguage, ",") {
		base := strings.ToLower(strings.TrimSpace(strings.SplitN(candidate, ";", 2)[0]))
		if index := strings.IndexByte(base, '-'); index >= 0 {
			base = base[:index]
		}
		if isSupported(base) {
			return base
		}
	}
	return fallbackLocale
}

func Translate(locale, key string, params map[string]any) string {
	if !isSupported(locale) {
		locale = fallbackLocale
	}
	message, ok := catalogs[locale][key]
	if !ok {
		message = catalogs[fallbackLocale][key]
	}
	if message == "" {
		return key
	}
	for name, value := range params {
		message = strings.ReplaceAll(message, "{"+name+"}", fmt.Sprint(value))
	}
	return message
}

func isSupported(locale string) bool {
	_, ok := catalogs[locale]
	return ok
}
