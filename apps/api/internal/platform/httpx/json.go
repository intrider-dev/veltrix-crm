package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
)

const DefaultJSONLimit = 1 << 20

func DecodeJSON[T any](writer http.ResponseWriter, request *http.Request, limit int64) (T, []byte, error) {
	var destination T
	mediaType := strings.ToLower(strings.TrimSpace(strings.SplitN(request.Header.Get("Content-Type"), ";", 2)[0]))
	if mediaType != "application/json" {
		return destination, nil, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/headers/Content-Type",
			Code:    "validation.content_type.json_required",
		}}}
	}
	if limit <= 0 {
		limit = DefaultJSONLimit
	}
	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, limit))
	if err != nil {
		return destination, nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/", Code: "validation.body.too_large"}}}
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&destination); err != nil {
		return destination, nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/", Code: "validation.json.invalid"}}}
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return destination, nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/", Code: "validation.json.single_value"}}}
	}
	return destination, body, nil
}

func WriteJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func WriteJSONBytes(writer http.ResponseWriter, status int, body []byte) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_, _ = writer.Write(body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		_, _ = writer.Write([]byte{'\n'})
	}
}
