package app

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

func (application *Application) dashboard(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result apigen.Dashboard
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionReportsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			row, loadErr := application.reporting.Dashboard(request.Context(), workspace, workspaceID)
			if loadErr != nil {
				return loadErr
			}
			result = apigen.Dashboard{
				Currency: row.Currency, OpenPipelineMinor: row.OpenPipelineMinor,
				WeightedForecastMinor: row.WeightedForecastMinor, WonCount: int(row.WonCount),
				LostCount: int(row.LostCount), OverdueTasks: int(row.OverdueTasks),
			}
			if len(row.DealsByStage) > 0 {
				var raw []struct {
					AmountMinor int64  `json:"amountMinor"`
					Count       int    `json:"count"`
					StageId     string `json:"stageId"`
					StageName   string `json:"stageName"`
				}
				if err := json.Unmarshal(row.DealsByStage, &raw); err != nil {
					return err
				}
				mapped := make([]struct {
					AmountMinor int64     `json:"amountMinor"`
					Count       int       `json:"count"`
					StageId     uuid.UUID `json:"stageId"`
					StageName   string    `json:"stageName"`
				}, 0, len(raw))
				for _, stage := range raw {
					id, err := uuid.Parse(stage.StageId)
					if err != nil {
						return err
					}
					mapped = append(mapped, struct {
						AmountMinor int64     `json:"amountMinor"`
						Count       int       `json:"count"`
						StageId     uuid.UUID `json:"stageId"`
						StageName   string    `json:"stageName"`
					}{stage.AmountMinor, stage.Count, id, stage.StageName})
				}
				result.DealsByStage = &mapped
			}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "private, max-age=15")
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) globalSearch(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []apigen.SearchResult
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			rows, loadErr := application.search.Global(request.Context(), workspace, workspaceID, request.URL.Query().Get("q"))
			if loadErr != nil {
				return loadErr
			}
			result = make([]apigen.SearchResult, 0, len(rows))
			for _, row := range rows {
				snippet := row.Snippet
				result = append(result, apigen.SearchResult{
					EntityId: apiID(row.EntityID), EntityType: apigen.SearchResultEntityType(row.EntityType),
					Title: row.Title, Subtitle: row.Subtitle, Snippet: &snippet, Rank: row.Rank,
				})
			}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) listAudit(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []apigen.AuditEvent
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID, httpx.RequestID(request.Context()), tenancy.PermissionAuditRead,
		func(workspace *tenancy.WorkspaceTx) error {
			rows, loadErr := application.audit.List(request.Context(), workspace, workspaceID, 100)
			if loadErr != nil {
				return loadErr
			}
			result = make([]apigen.AuditEvent, 0, len(rows))
			for _, row := range rows {
				var summary map[string]interface{}
				if err := json.Unmarshal(row.Summary, &summary); err != nil {
					return err
				}
				result = append(result, apigen.AuditEvent{
					Id: apiID(row.ID), ActorId: apiIDPointer(row.ActorUserID), Action: row.Action,
					EntityType: row.EntityType, EntityId: apiID(row.EntityID), RequestId: row.RequestID,
					Summary: &summary, OccurredAt: row.OccurredAt.Time.UTC(),
				})
			}
			return nil
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}
