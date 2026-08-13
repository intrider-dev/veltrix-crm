package localization

import (
	"reflect"
	"testing"
)

func TestExtractPlaceholders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		message string
		want    []string
		wantErr bool
	}{
		{name: "sorted unique", message: "Hello {contact.name}, {owner}! Again {owner}.", want: []string{"contact.name", "owner"}},
		{name: "escaped braces", message: "Use {{literal}} with {value}", want: []string{"value"}},
		{name: "unclosed", message: "Hello {name", wantErr: true},
		{name: "unmatched closing", message: "Hello name}", wantErr: true},
		{name: "invalid name", message: "Hello {0name}", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExtractPlaceholders(test.message)
			if (err != nil) != test.wantErr {
				t.Fatalf("ExtractPlaceholders() error = %v, wantErr %v", err, test.wantErr)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("ExtractPlaceholders() = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestContentCursorBindsFilters(t *testing.T) {
	t.Parallel()
	cursor, err := encodeContentCursor("email", "deal.won", "ru\x00email\x00draft\x00")
	if err != nil {
		t.Fatal(err)
	}
	namespace, key, err := decodeContentCursor(cursor, "ru\x00email\x00draft\x00")
	if err != nil || namespace != "email" || key != "deal.won" {
		t.Fatalf("decodeContentCursor() = %q, %q, %v", namespace, key, err)
	}
	if _, _, err := decodeContentCursor(cursor, "en\x00email\x00draft\x00"); err == nil {
		t.Fatal("cursor must not be reusable across filters")
	}
}

func TestNormalizeLocale(t *testing.T) {
	t.Parallel()
	for input, want := range map[string]string{"RU": "ru", "pt-BR": "pt-br", "zh-Hant-TW": "zh-hant-tw"} {
		got, ok := NormalizeLocale(input)
		if !ok || got != want {
			t.Fatalf("NormalizeLocale(%q) = %q, %v; want %q, true", input, got, ok, want)
		}
	}
	for _, invalid := range []string{"", "e", "../../ru", "en_US", "en--US"} {
		if _, ok := NormalizeLocale(invalid); ok {
			t.Fatalf("NormalizeLocale(%q) unexpectedly succeeded", invalid)
		}
	}
}
