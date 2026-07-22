package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/activities"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/notifications"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/brand"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/reporting"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// registerCollaborationReportingRoutes is deliberately isolated so the main
// router needs one auditable wiring call rather than a long list of handlers.
func (application *Application) registerCollaborationReportingRoutes(router chi.Router) {
	router.Get("/activity-feed", application.activityFeed)
	router.Get("/timeline/{entityType}/{entityId}", application.entityTimeline)
	router.Get("/activities/{activityId}", application.getAdvancedActivity)
	router.Put("/activities/{activityId}", application.updateAdvancedActivity)
	router.Delete("/activities/{activityId}", application.deleteAdvancedActivity)
	router.Get("/activities/{activityId}/assignments", application.listTaskAssignments)
	router.Put("/activities/{activityId}/assignments", application.replaceTaskAssignments)
	router.Get("/activities/{activityId}/comments", application.listActivityComments)
	router.Post("/activities/{activityId}/comments", application.createActivityComment)
	router.Put("/comments/{commentId}", application.updateActivityComment)
	router.Delete("/comments/{commentId}", application.deleteActivityComment)
	router.Get("/activities/{activityId}/reminders", application.listActivityReminders)
	router.Post("/activities/{activityId}/reminders", application.createActivityReminder)
	router.Put("/reminders/{reminderId}", application.updateActivityReminder)
	router.Delete("/reminders/{reminderId}", application.cancelActivityReminder)
	router.Get("/calendar", application.calendarActivities)
	router.Get("/calendar.ics", application.exportCalendarICS)
	router.Get("/notifications", application.listNotifications)
	router.Put("/notifications/{notificationId}/read", application.markNotificationRead)
	router.Post("/notifications/read-all", application.markAllNotificationsRead)
	router.Get("/reports/period", application.periodReport)
	router.Get("/dashboard/preferences", application.getDashboardPreferences)
	router.Put("/dashboard/preferences", application.saveDashboardPreferences)
}

type activityWriteRequest struct {
	Type              string     `json:"type"`
	Title             string     `json:"title"`
	Body              *string    `json:"body"`
	RelatedType       *string    `json:"relatedType"`
	RelatedID         *uuid.UUID `json:"relatedId"`
	AssigneeID        *uuid.UUID `json:"assigneeId"`
	Status            string     `json:"status"`
	Priority          string     `json:"priority"`
	DueAt             *time.Time `json:"dueAt"`
	OccurredAt        time.Time  `json:"occurredAt"`
	EndsAt            *time.Time `json:"endsAt"`
	Location          *string    `json:"location"`
	RecurrenceRule    *string    `json:"recurrenceRule"`
	VisibilityScope   string     `json:"visibilityScope"`
	ScopeDepartmentID *uuid.UUID `json:"scopeDepartmentId"`
	ScopeUserID       *uuid.UUID `json:"scopeUserId"`
}

func (body activityWriteRequest) input() activities.AdvancedInput {
	return activities.AdvancedInput{
		Type: body.Type, Title: body.Title, Body: body.Body, RelatedType: body.RelatedType,
		RelatedID: internalIDPointer(body.RelatedID), AssigneeID: internalIDPointer(body.AssigneeID),
		Status: body.Status, Priority: body.Priority, DueAt: body.DueAt,
		OccurredAt: body.OccurredAt, EndsAt: body.EndsAt, Location: body.Location,
		RecurrenceRule: body.RecurrenceRule, VisibilityScope: body.VisibilityScope,
		ScopeDepartmentID: internalIDPointer(body.ScopeDepartmentID),
		ScopeUserID:       internalIDPointer(body.ScopeUserID),
	}
}

// createAdvancedActivity replaces the compact Phase-1 POST handler once the
// OpenAPI request schema contains the calendar and recurrence fields.
func (application *Application) createAdvancedActivity(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[activityWriteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate,
		"activities.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			activity, createErr := application.activities.CreateAdvanced(
				request.Context(), workspace, metadata, body.input(),
			)
			return activityResponse(activity), activity.Version, createErr
		})
}

func (application *Application) getAdvancedActivity(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.Activity
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.activities.Get(request.Context(), workspace, workspaceID, activityID)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, activityResponse(result))
}

func (application *Application) updateAdvancedActivity(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[activityWriteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.Activity
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = application.activities.Update(
				request.Context(), workspace, metadata(request, workspaceID, principal),
				activityID, version, body.input(),
			)
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, activityResponse(result))
}

func (application *Application) deleteAdvancedActivity(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsDelete,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.activities.Delete(request.Context(), workspace,
				metadata(request, workspaceID, principal), activityID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) activityFeed(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	assigneeID, err := parseOptionalID(request.URL.Query().Get("assigneeId"), "/query/assigneeId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.ActivityPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.activities.Feed(request.Context(), workspace, workspaceID,
				request.URL.Query().Get("type"), request.URL.Query().Get("status"), assigneeID,
				request.URL.Query().Get("cursor"), limit)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, activityPageResponse(result))
}

func (application *Application) entityTimeline(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	entityID, err := parsePathID(request, "entityId")
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.ActivityPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.activities.Timeline(request.Context(), workspace, workspaceID,
				chi.URLParam(request, "entityType"), entityID, request.URL.Query().Get("cursor"), limit)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, activityPageResponse(result))
}

type commentWriteRequest struct {
	Body             string      `json:"body"`
	MentionedUserIDs []uuid.UUID `json:"mentionedUserIds"`
}

func (body commentWriteRequest) input() activities.CommentInput {
	mentions := make([]ids.UUID, 0, len(body.MentionedUserIDs))
	for _, mention := range body.MentionedUserIDs {
		mentions = append(mentions, ids.UUID(mention))
	}
	return activities.CommentInput{Body: body.Body, MentionedUserIDs: mentions}
}

func (application *Application) createActivityComment(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[commentWriteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate,
		"activities.comments.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			notifier := notifications.NewService()
			comment, createErr := application.activities.CreateComment(
				request.Context(), workspace, metadata, activityID, principal.DisplayName, body.input(),
				func(ctx context.Context, mention activities.MentionNotification) error {
					params := map[string]any{}
					if err := json.Unmarshal(mention.MessageParams, &params); err != nil {
						return err
					}
					entityType := "activity"
					_, err := notifier.Create(ctx, workspace, workspaceID, notifications.Input{
						RecipientUserID: mention.RecipientUserID, MessageKey: mention.MessageKey,
						MessageParams: params, TemplateVersion: 1, EntityType: &entityType,
						EntityID: &mention.ActivityID, Delivery: notifications.DeliveryInApp,
					})
					return err
				},
			)
			return commentResponse(comment), comment.Version, createErr
		})
}

func (application *Application) listActivityComments(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.CommentPage
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.activities.ListComments(request.Context(), workspace,
				workspaceID, activityID, request.URL.Query().Get("cursor"), limit)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, commentPageResponse(result))
}

func (application *Application) updateActivityComment(writer http.ResponseWriter, request *http.Request) {
	workspaceID, commentID, err := collaborationIDs(application, request, "commentId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[commentWriteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.Comment
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = application.activities.UpdateComment(request.Context(), workspace,
				metadata(request, workspaceID, principal), commentID, version, body.input())
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, commentResponse(result))
}

func (application *Application) deleteActivityComment(writer http.ResponseWriter, request *http.Request) {
	workspaceID, commentID, err := collaborationIDs(application, request, "commentId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.activities.DeleteComment(request.Context(), workspace,
				metadata(request, workspaceID, principal), commentID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

type reminderWriteRequest struct {
	RecipientUserID uuid.UUID `json:"recipientUserId"`
	RemindAt        time.Time `json:"remindAt"`
	Channel         string    `json:"channel"`
}

func (body reminderWriteRequest) input() activities.ReminderInput {
	return activities.ReminderInput{
		RecipientUserID: ids.UUID(body.RecipientUserID), RemindAt: body.RemindAt, Channel: body.Channel,
	}
}

func (application *Application) createActivityReminder(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	body, raw, err := httpx.DecodeJSON[reminderWriteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	if writeError(application, writer, request, application.requireNotificationEmail(body.Channel)) {
		return
	}
	application.runIdempotent(writer, request, workspaceID, tenancy.PermissionRecordsCreate,
		"activities.reminders.create", raw, http.StatusCreated,
		func(workspace *tenancy.WorkspaceTx, metadata events.Metadata) (any, int64, error) {
			reminder, createErr := application.activities.CreateReminder(
				request.Context(), workspace, metadata, activityID, body.input(),
			)
			return reminderResponse(reminder), reminder.Version, createErr
		})
}

func (application *Application) listActivityReminders(writer http.ResponseWriter, request *http.Request) {
	workspaceID, activityID, err := collaborationIDs(application, request, "activityId")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result []activities.Reminder
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.activities.ListReminders(request.Context(), workspace, workspaceID, activityID)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	items := make([]any, 0, len(result))
	for _, reminder := range result {
		items = append(items, reminderResponse(reminder))
	}
	httpx.WriteJSON(writer, http.StatusOK, items)
}

func (application *Application) updateActivityReminder(writer http.ResponseWriter, request *http.Request) {
	workspaceID, reminderID, err := collaborationIDs(application, request, "reminderId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[reminderWriteRequest](writer, request, httpx.DefaultJSONLimit)
	if writeError(application, writer, request, err) {
		return
	}
	if writeError(application, writer, request, application.requireNotificationEmail(body.Channel)) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result activities.Reminder
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = application.activities.UpdateReminder(request.Context(), workspace,
				metadata(request, workspaceID, principal), reminderID, version, body.input())
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, reminderResponse(result))
}

func (application *Application) cancelActivityReminder(writer http.ResponseWriter, request *http.Request) {
	workspaceID, reminderID, err := collaborationIDs(application, request, "reminderId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsUpdate,
		func(workspace *tenancy.WorkspaceTx) error {
			return application.activities.CancelReminder(request.Context(), workspace, workspaceID, reminderID, version)
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (application *Application) calendarActivities(writer http.ResponseWriter, request *http.Request) {
	items, err := application.loadCalendar(request)
	if writeError(application, writer, request, err) {
		return
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, activityResponse(item))
	}
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func (application *Application) exportCalendarICS(writer http.ResponseWriter, request *http.Request) {
	items, err := application.loadCalendar(request)
	if writeError(application, writer, request, err) {
		return
	}
	exportItems := make([]activities.CalendarItem, 0, len(items))
	for _, item := range items {
		exportItems = append(exportItems, activities.CalendarItem{
			ID: item.ID.String(), Type: item.Type, Title: item.Title,
			Body: stringValue(item.Body), Location: stringValue(item.Location), Status: item.Status,
			Start: item.OccurredAt, End: item.EndsAt, Due: item.DueAt,
			RecurrenceRule: stringValue(item.RecurrenceRule), UpdatedAt: item.UpdatedAt,
		})
	}
	writer.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	writer.Header().Set("Content-Disposition", `attachment; filename="calendar.ics"`)
	writer.Header().Set("Cache-Control", "private, no-store")
	if err := activities.WriteICS(writer, activities.CalendarExport{
		ProductName: brand.Config.ProductName, ProductID: brand.Config.RepositoryName,
		GeneratedAt: time.Now().UTC(), Items: exportItems,
	}); err != nil {
		application.logger.Error("calendar export failed", "error_code", "ics_write_failed",
			"request_id", httpx.RequestID(request.Context()))
	}
}

func (application *Application) loadCalendar(request *http.Request) ([]activities.Activity, error) {
	workspaceID, err := application.workspaceID(request)
	if err != nil {
		return nil, err
	}
	start, err := parseQueryTime(request, "start")
	if err != nil {
		return nil, err
	}
	end, err := parseQueryTime(request, "end")
	if err != nil {
		return nil, err
	}
	principal, _ := httpx.Principal(request.Context())
	var result []activities.Activity
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.activities.Calendar(request.Context(), workspace, workspaceID, start, end)
			return loadErr
		})
	return result, err
}

func (application *Application) listNotifications(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	limit, err := parseLimit(request, 50)
	if writeError(application, writer, request, err) {
		return
	}
	unread, err := parseQueryBool(request, "unread")
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result notifications.Page
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = notifications.NewService().List(request.Context(), workspace, workspaceID,
				principal.UserID, unread, request.URL.Query().Get("cursor"), limit)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, notificationPageResponse(result))
}

func (application *Application) markNotificationRead(writer http.ResponseWriter, request *http.Request) {
	workspaceID, notificationID, err := collaborationIDs(application, request, "notificationId")
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result notifications.Notification
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = notifications.NewService().MarkRead(request.Context(), workspace,
				workspaceID, principal.UserID, notificationID, version)
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, notificationResponse(result))
}

func (application *Application) markAllNotificationsRead(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var updated int64
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionRecordsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			updated, updateErr = notifications.NewService().MarkAllRead(
				request.Context(), workspace, workspaceID, principal.UserID,
			)
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, map[string]int64{"updated": updated})
}

func (application *Application) periodReport(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result reporting.Report
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionReportsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			preferences, loadErr := application.reporting.Preferences(
				request.Context(), workspace, workspaceID, principal.UserID,
			)
			if loadErr != nil {
				return loadErr
			}
			period, periodErr := reportPeriod(request, preferences)
			if periodErr != nil {
				return periodErr
			}
			result, loadErr = application.reporting.PeriodReport(request.Context(), workspace, workspaceID, period)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	writer.Header().Set("Cache-Control", "private, max-age=15")
	httpx.WriteJSON(writer, http.StatusOK, reportResponse(result))
}

func (application *Application) getDashboardPreferences(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result reporting.Preferences
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionReportsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var loadErr error
			result, loadErr = application.reporting.Preferences(request.Context(), workspace, workspaceID, principal.UserID)
			return loadErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

type dashboardPreferencesRequest struct {
	Layout     json.RawMessage `json:"layout"`
	PeriodDays int16           `json:"periodDays"`
	Timezone   *string         `json:"timezone"`
}

func (application *Application) saveDashboardPreferences(writer http.ResponseWriter, request *http.Request) {
	workspaceID, err := application.workspaceID(request)
	if writeError(application, writer, request, err) {
		return
	}
	version, err := parseOptionalETag(request)
	if writeError(application, writer, request, err) {
		return
	}
	body, _, err := httpx.DecodeJSON[dashboardPreferencesRequest](writer, request, 64<<10)
	if writeError(application, writer, request, err) {
		return
	}
	principal, _ := httpx.Principal(request.Context())
	var result reporting.Preferences
	err = application.tenancy.WithWorkspace(request.Context(), principal, workspaceID,
		httpx.RequestID(request.Context()), tenancy.PermissionReportsRead,
		func(workspace *tenancy.WorkspaceTx) error {
			var updateErr error
			result, updateErr = application.reporting.SavePreferences(request.Context(), workspace,
				workspaceID, principal.UserID, version, reporting.PreferencesInput{
					Layout: body.Layout, PeriodDays: body.PeriodDays, Timezone: body.Timezone,
				})
			return updateErr
		})
	if writeError(application, writer, request, err) {
		return
	}
	setETag(writer, result.Version)
	httpx.WriteJSON(writer, http.StatusOK, result)
}

func collaborationIDs(application *Application, request *http.Request, resource string) (ids.UUID, ids.UUID, error) {
	workspaceID, err := application.workspaceID(request)
	if err != nil {
		return ids.UUID{}, ids.UUID{}, err
	}
	resourceID, err := parsePathID(request, resource)
	return workspaceID, resourceID, err
}

func parseQueryTime(request *http.Request, key string) (time.Time, error) {
	value := request.URL.Query().Get(key)
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/query/" + key, Code: "validation.datetime.invalid",
		}}}
	}
	return parsed.UTC(), nil
}

func parseQueryBool(request *http.Request, key string) (bool, error) {
	value := request.URL.Query().Get(key)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/query/" + key, Code: "validation.boolean.invalid",
		}}}
	}
	return parsed, nil
}

func parseOptionalETag(request *http.Request) (int64, error) {
	if strings.TrimSpace(request.Header.Get("If-Match")) == "" {
		return 0, nil
	}
	return parseETag(request)
}

func reportPeriod(request *http.Request, preferences reporting.Preferences) (reporting.Period, error) {
	startRaw := request.URL.Query().Get("start")
	endRaw := request.URL.Query().Get("end")
	timezone := request.URL.Query().Get("timezone")
	if timezone == "" {
		timezone = preferences.EffectiveTimezone
	}
	if startRaw == "" && endRaw == "" {
		preferences.EffectiveTimezone = timezone
		return reporting.PeriodFromPreferences(time.Now().UTC(), preferences)
	}
	if startRaw == "" || endRaw == "" {
		return reporting.Period{}, &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/query/end", Code: "validation.reference.incomplete",
		}}}
	}
	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return reporting.Period{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/query/start", Code: "validation.datetime.invalid"}}}
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return reporting.Period{}, &errx.ValidationError{Fields: []errx.FieldError{{Pointer: "/query/end", Code: "validation.datetime.invalid"}}}
	}
	return reporting.ValidatePeriod(start, end, timezone)
}

func activityResponse(value activities.Activity) map[string]any {
	result := map[string]any{
		"id": value.ID.String(), "type": value.Type, "title": value.Title,
		"status": value.Status, "priority": value.Priority, "occurredAt": value.OccurredAt,
		"createdBy": value.CreatedBy.String(), "version": value.Version,
		"visibilityScope": value.VisibilityScope,
		"createdAt":       value.CreatedAt, "updatedAt": value.UpdatedAt,
	}
	optionalResponse(result, "body", value.Body)
	optionalResponse(result, "relatedType", value.RelatedType)
	optionalIDResponse(result, "relatedId", value.RelatedID)
	optionalIDResponse(result, "assigneeId", value.AssigneeID)
	optionalResponse(result, "dueAt", value.DueAt)
	optionalResponse(result, "endsAt", value.EndsAt)
	optionalResponse(result, "location", value.Location)
	optionalResponse(result, "recurrenceRule", value.RecurrenceRule)
	optionalIDResponse(result, "scopeDepartmentId", value.ScopeDepartmentID)
	optionalIDResponse(result, "scopeUserId", value.ScopeUserID)
	optionalResponse(result, "completedAt", value.CompletedAt)
	return result
}

func activityPageResponse(page activities.ActivityPage) map[string]any {
	items := make([]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, activityResponse(item))
	}
	result := map[string]any{"items": items}
	if page.NextCursor != "" {
		result["nextCursor"] = page.NextCursor
	}
	return result
}

func commentResponse(value activities.Comment) map[string]any {
	mentions := make([]string, 0, len(value.MentionedUserIDs))
	for _, mention := range value.MentionedUserIDs {
		mentions = append(mentions, mention.String())
	}
	return map[string]any{
		"id": value.ID.String(), "activityId": value.ActivityID.String(),
		"authorUserId": value.AuthorUserID.String(), "body": value.Body,
		"mentionedUserIds": mentions, "version": value.Version,
		"createdAt": value.CreatedAt, "updatedAt": value.UpdatedAt,
	}
}

func commentPageResponse(page activities.CommentPage) map[string]any {
	items := make([]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, commentResponse(item))
	}
	result := map[string]any{"items": items}
	if page.NextCursor != "" {
		result["nextCursor"] = page.NextCursor
	}
	return result
}

func reminderResponse(value activities.Reminder) map[string]any {
	result := map[string]any{
		"id": value.ID.String(), "activityId": value.ActivityID.String(),
		"recipientUserId": value.RecipientUserID.String(), "remindAt": value.RemindAt,
		"channel": value.Channel, "version": value.Version, "createdAt": value.CreatedAt,
	}
	optionalResponse(result, "deliveredAt", value.DeliveredAt)
	optionalResponse(result, "cancelledAt", value.CancelledAt)
	return result
}

func notificationResponse(value notifications.Notification) map[string]any {
	params := map[string]any{}
	_ = json.Unmarshal(value.MessageParams, &params)
	result := map[string]any{
		"id": value.ID.String(), "recipientUserId": value.RecipientUserID.String(),
		"messageKey": value.MessageKey, "messageParams": params,
		"templateVersion": value.TemplateVersion, "version": value.Version,
		"emailState": value.EmailState, "createdAt": value.CreatedAt,
	}
	optionalResponse(result, "entityType", value.EntityType)
	optionalIDResponse(result, "entityId", value.EntityID)
	optionalResponse(result, "readAt", value.ReadAt)
	return result
}

func notificationPageResponse(page notifications.Page) map[string]any {
	items := make([]any, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, notificationResponse(item))
	}
	result := map[string]any{"items": items}
	if page.NextCursor != "" {
		result["nextCursor"] = page.NextCursor
	}
	return result
}

func reportResponse(value reporting.Report) map[string]any {
	stages := make([]any, 0, len(value.DealsByStage))
	for _, stage := range value.DealsByStage {
		stages = append(stages, map[string]any{
			"stageId": stage.StageID.String(), "stageName": stage.StageName,
			"position": stage.Position, "dealCount": stage.DealCount,
			"amountMinor": stage.AmountMinor, "weightedAmountMinor": stage.WeightedAmountMinor,
		})
	}
	owners := make([]any, 0, len(value.DealsByOwner))
	for _, owner := range value.DealsByOwner {
		item := map[string]any{
			"ownerName": owner.OwnerName, "dealCount": owner.DealCount,
			"wonCount": owner.WonCount, "lostCount": owner.LostCount, "amountMinor": owner.AmountMinor,
		}
		optionalIDResponse(item, "ownerId", owner.OwnerID)
		owners = append(owners, item)
	}
	return map[string]any{
		"period":   map[string]any{"start": value.Period.Start, "end": value.Period.End, "timezone": value.Period.Timezone},
		"overview": value.Overview, "dealsByStage": stages, "dealsByOwner": owners,
		"activities": value.Activities, "leadSources": value.LeadSources,
	}
}

func optionalResponse[T any](target map[string]any, key string, value *T) {
	if value != nil {
		target[key] = *value
	}
}

func optionalIDResponse(target map[string]any, key string, value *ids.UUID) {
	if value != nil {
		target[key] = value.String()
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (application *Application) requireNotificationEmail(channel string) error {
	if (channel == "email" || channel == "both") && application.cfg.SMTPAddress == "" {
		return &errx.ValidationError{Fields: []errx.FieldError{{
			Pointer: "/channel", Code: "validation.email.disabled",
		}}}
	}
	return nil
}
