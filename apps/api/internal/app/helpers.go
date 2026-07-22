package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/identity"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/idempotency"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

type idempotentResult struct {
	status  int
	body    []byte
	version int64
}

func (application *Application) workspaceID(request *http.Request) (ids.UUID, error) {
	return parsePathID(request, "workspaceId")
}

func parsePathID(request *http.Request, name string) (ids.UUID, error) {
	id, err := ids.Parse(chi.URLParam(request, name))
	if err != nil {
		return ids.UUID{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/path/" + name, Code: "validation.uuid.invalid",
		}}}
	}
	return id, nil
}

func parseOptionalID(value, pointer string) (*ids.UUID, error) {
	if value == "" {
		return nil, nil
	}
	id, err := ids.Parse(value)
	if err != nil {
		return nil, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: pointer, Code: "validation.uuid.invalid"}}}
	}
	return &id, nil
}

func parseLimit(request *http.Request, fallback int) (int, error) {
	raw := request.URL.Query().Get("limit")
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 100 {
		return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/query/limit", Code: "validation.range"}}}
	}
	return value, nil
}

func parseETag(request *http.Request) (int64, error) {
	raw := strings.TrimSpace(request.Header.Get("If-Match"))
	if len(raw) < 3 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/headers/If-Match", Code: "validation.etag.invalid"}}}
	}
	version, err := strconv.ParseInt(raw[1:len(raw)-1], 10, 64)
	if err != nil || version < 1 {
		return 0, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/headers/If-Match", Code: "validation.etag.invalid"}}}
	}
	return version, nil
}

func setETag(writer http.ResponseWriter, version int64) {
	writer.Header().Set("ETag", fmt.Sprintf("\"%d\"", version))
}

func metadata(request *http.Request, workspaceID ids.UUID, principal identity.Principal) events.Metadata {
	return events.Metadata{
		WorkspaceID: workspaceID,
		ActorID:     principal.UserID,
		RequestID:   httpx.RequestID(request.Context()),
		IPAddress:   remoteIP(request),
		UserAgent:   request.UserAgent(),
	}
}

func (application *Application) runIdempotent(
	writer http.ResponseWriter,
	request *http.Request,
	workspaceID ids.UUID,
	permission tenancy.Permission,
	operation string,
	rawBody []byte,
	status int,
	mutation func(*tenancy.WorkspaceTx, events.Metadata) (any, int64, error),
) {
	principal, _ := httpx.Principal(request.Context())
	key := request.Header.Get("Idempotency-Key")
	result := idempotentResult{}
	err := application.tenancy.WithWorkspace(
		request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), permission,
		func(workspace *tenancy.WorkspaceTx) error {
			replay, err := idempotency.Reserve(request.Context(), workspace.Queries, workspaceID, principal.UserID, key, operation, rawBody)
			if err != nil {
				return err
			}
			if replay != nil {
				result.status = replay.Status
				result.body = append([]byte(nil), replay.Body...)
				var versioned struct {
					Version int64 `json:"version"`
				}
				_ = json.Unmarshal(result.body, &versioned)
				result.version = versioned.Version
				return nil
			}
			value, version, err := mutation(workspace, metadata(request, workspaceID, principal))
			if err != nil {
				return err
			}
			encoded, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("encode idempotent response: %w", err)
			}
			if err := idempotency.Complete(request.Context(), workspace.Queries, workspaceID, key, status, encoded); err != nil {
				return err
			}
			result = idempotentResult{status: status, body: encoded, version: version}
			return nil
		},
	)
	if err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	if result.version > 0 {
		setETag(writer, result.version)
	}
	httpx.WriteJSONBytes(writer, result.status, result.body)
}

func writeError(application *Application, writer http.ResponseWriter, request *http.Request, err error) bool {
	if err == nil {
		return false
	}
	httpx.WriteProblem(writer, request, application.logger, err)
	return true
}
