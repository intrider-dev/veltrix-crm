package app

import (
	"net/http"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/api/apigen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/brand"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/httpx"
)

func (application *Application) healthLive(writer http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(writer, http.StatusOK, apigen.Health{Status: apigen.Ok})
}

func (application *Application) healthReady(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := application.readinessContext(request.Context())
	defer cancel()
	if err := application.pool.Ping(ctx); err != nil {
		httpx.WriteProblem(writer, request, application.logger, err)
		return
	}
	httpx.WriteJSON(writer, http.StatusOK, apigen.Health{Status: apigen.Ok})
}

func (application *Application) publicConfig(writer http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(writer, http.StatusOK, apigen.PublicConfig{
		ProductName: brand.Config.ProductName, ShortName: brand.Config.ShortName,
		Description: brand.Config.Description, DefaultLocale: application.cfg.DefaultLocale,
		SupportedLocales: append([]string(nil), application.cfg.SupportedLocales...),
	})
}
