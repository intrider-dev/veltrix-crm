package ai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/database/dbgen"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/events"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/ids"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/tenancy"
)

// RecordAudit stores only operation metadata. Prompt context, model output,
// provider credentials, and the configured model name are deliberately absent.
func RecordAudit(
	ctx context.Context,
	workspace *tenancy.WorkspaceTx,
	metadata events.Metadata,
	capability Capability,
	provider ProviderInfo,
	externalConsent bool,
	outputBytes int,
) error {
	eventID, err := ids.NewV7()
	if err != nil {
		return fmt.Errorf("generate AI audit ID: %w", err)
	}
	operationID, err := ids.NewV7()
	if err != nil {
		return fmt.Errorf("generate AI operation ID: %w", err)
	}
	summary, err := auditSummary(capability, provider, externalConsent, outputBytes)
	if err != nil {
		return err
	}
	if len(metadata.UserAgent) > 512 {
		metadata.UserAgent = metadata.UserAgent[:512]
	}
	if err := workspace.Queries.InsertAuditEvent(ctx, dbgen.InsertAuditEventParams{
		WorkspaceID: metadata.WorkspaceID.PG(),
		ID:          eventID.PG(),
		ActorUserID: metadata.ActorID.PG(),
		Action:      "ai." + string(capability) + ".generated",
		EntityType:  "ai_assistance",
		EntityID:    operationID.PG(),
		RequestID:   metadata.RequestID,
		Summary:     summary,
		IpAddress:   metadata.IPAddress,
		UserAgent:   metadata.UserAgent,
	}); err != nil {
		return fmt.Errorf("insert AI audit event: %w", err)
	}
	return nil
}

func auditSummary(
	capability Capability,
	provider ProviderInfo,
	externalConsent bool,
	outputBytes int,
) ([]byte, error) {
	summary, err := json.Marshal(struct {
		Capability              Capability    `json:"capability"`
		Provider                string        `json:"provider"`
		ProviderClass           ProviderClass `json:"providerClass"`
		ExternalTransferConsent bool          `json:"externalTransferConsent"`
		OutputBytes             int           `json:"outputBytes"`
	}{
		Capability: capability, Provider: provider.Name, ProviderClass: provider.Class,
		ExternalTransferConsent: externalConsent, OutputBytes: outputBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("encode AI audit summary: %w", err)
	}
	return summary, nil
}
