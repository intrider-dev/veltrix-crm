package localization

import "testing"

func TestResolveLocale(t *testing.T) {
	t.Parallel()
	if got := Resolve("ru", "en-US"); got != "ru" {
		t.Fatalf("Resolve preferred = %q", got)
	}
	if got := Resolve("", "de-DE, ru-RU;q=0.9"); got != "ru" {
		t.Fatalf("Resolve header = %q", got)
	}
	if got := Resolve("xx", "de"); got != "en" {
		t.Fatalf("Resolve fallback = %q", got)
	}
}

func TestTranslateFallbackAndParams(t *testing.T) {
	t.Parallel()
	if got := Translate("ru", "problems.problem.generic", map[string]any{"requestId": "req-1"}); got != "Произошла ошибка. ID запроса: req-1" {
		t.Fatalf("Translate = %q", got)
	}
	if got := Translate("xx", "auth.login.title", nil); got != "Sign in" {
		t.Fatalf("fallback = %q", got)
	}
}
