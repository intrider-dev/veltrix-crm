package app

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/errx"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
)

func (application *Application) routes() http.Handler {
	router := chi.NewRouter()
	router.Use(httpx.RequestContext)
	router.Use(func(next http.Handler) http.Handler { return httpx.Recover(application.logger, next) })
	router.Use(func(next http.Handler) http.Handler {
		callsOrigin := ""
		if application.cfg.CallsProvider == "livekit" {
			callsOrigin = application.cfg.LiveKitPublicURL
		}
		return httpx.SecurityHeaders(application.cfg.Environment == "production" && application.cfg.CookieSecure, callsOrigin, next)
	})
	router.Use(func(next http.Handler) http.Handler {
		return httpx.SameOrigin(application.cfg.PublicURL, application.logger, next)
	})
	router.Use(func(next http.Handler) http.Handler { return httpx.AccessLog(application.logger, next) })

	router.Route("/api/v1", func(api chi.Router) {
		api.Get("/health/live", application.healthLive)
		api.Get("/health/ready", application.healthReady)
		api.Get("/config", application.publicConfig)
		api.Post("/auth/login", application.login)
		api.Get("/auth/session", application.sessionProbe)
		api.Post("/auth/register", application.registerDevelopmentUser)
		api.Post("/auth/mfa/verify", application.completeMFALogin)
		api.Post("/auth/password-reset/request", application.requestPasswordReset)
		api.Post("/auth/password-reset/confirm", application.confirmPasswordReset)

		api.Group(func(protected chi.Router) {
			protected.Use(application.authenticateCredential)
			protected.Use(application.csrf)
			protected.Post("/auth/logout", application.logout)
			protected.Get("/me", application.me)
			protected.Patch("/me", application.updatePreferences)
			protected.Put("/me/password", application.changePassword)
			protected.Delete("/me/sessions", application.logoutAllSessions)
			protected.Get("/me/mfa", application.mfaStatus)
			protected.Post("/me/mfa", application.beginMFASetup)
			protected.Post("/me/mfa/confirm", application.confirmMFASetup)
			protected.Post("/me/mfa/recovery-codes", application.regenerateRecoveryCodes)
			protected.Delete("/me/mfa", application.disableMFA)
			protected.Post("/workspaces", application.createWorkspace)
			protected.Post("/invitations/accept", application.acceptInvitation)
			protected.Route("/workspaces/{workspaceId}", func(workspace chi.Router) {
				RegisterAdvancedWorkspaceRoutes(workspace, application.advanced)
				application.registerRoleRoutes(workspace)
				application.registerAIRoutes(workspace)
				application.registerAdvancedCustomerRoutes(workspace)
				application.registerAdvancedSalesRoutes(workspace)
				application.registerCollaborationReportingRoutes(workspace)
				application.registerAttachmentRoutes(workspace)
				application.registerProjectRoutes(workspace)
				application.registerChatRoutes(workspace)
				application.registerCallRoutes(workspace)
				application.registerMailboxRoutes(workspace)
				workspace.Get("/dashboard", application.dashboard)
				workspace.Get("/contacts", application.listContacts)
				workspace.Post("/contacts", application.createContact)
				workspace.Get("/contacts/{contactId}", application.getContact)
				workspace.Patch("/contacts/{contactId}", application.updateContact)
				workspace.Delete("/contacts/{contactId}", application.deleteContact)
				workspace.Get("/companies", application.listCompanies)
				workspace.Post("/companies", application.createCompany)
				workspace.Get("/companies/{companyId}", application.getCompany)
				workspace.Get("/activities", application.listActivities)
				workspace.Post("/activities", application.createAdvancedActivity)
				workspace.Patch("/activities/{activityId}/complete", application.completeActivity)
				workspace.Get("/search", application.globalSearch)
				workspace.Get("/audit", application.listAudit)
				workspace.Get("/events", application.events)
				workspace.Get("/localization-settings", application.localizationSettings)
				workspace.Patch("/localization-settings", application.updateLocalizationSettings)
				workspace.Get("/translations", application.listTranslations)
				workspace.Get("/translation-coverage", application.translationCoverage)
				workspace.Put("/translations/{locale}/{namespace}/{translationKey}", application.putTranslation)
				workspace.Post("/invitations", application.inviteMember)
				workspace.Get("/members", application.listMembers)
				workspace.Patch("/members/{membershipId}/role", application.updateMemberRole)
				workspace.Patch("/members/{membershipId}/status", application.updateMemberStatus)
				workspace.Patch("/defaults", application.updateWorkspaceDefaults)
				workspace.Patch("/me/locale", application.setMyWorkspaceLocale)
				workspace.Get("/teams", application.listTeams)
				workspace.Post("/teams", application.createTeam)
				workspace.Get("/teams/{teamId}/members", application.listTeamMembers)
				workspace.Put("/teams/{teamId}/members/{membershipId}", application.addTeamMember)
				workspace.Delete("/teams/{teamId}/members/{membershipId}", application.removeTeamMember)
				workspace.Get("/departments", application.listTeams)
				workspace.Post("/departments", application.createTeam)
				workspace.Get("/departments/{teamId}/members", application.listTeamMembers)
				workspace.Put("/departments/{teamId}/members/{membershipId}", application.addTeamMember)
				workspace.Delete("/departments/{teamId}/members/{membershipId}", application.removeTeamMember)
			})
		})

		api.NotFound(func(writer http.ResponseWriter, request *http.Request) {
			httpx.WriteProblem(writer, request, application.logger, errx.ErrNotFound)
		})
	})
	router.NotFound(application.spa.ServeHTTP)
	return router
}

func ignoreNotFound(err error) error {
	if errors.Is(err, errx.ErrNotFound) {
		return nil
	}
	return err
}
