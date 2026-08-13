package app

import (
	"fmt"

	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/mailbox"
	"github.com/veltrixcrm/veltrix-crm/apps/api/internal/platform/config"
)

func buildMailboxService(cfg config.Config) (*mailbox.Service, error) {
	cipher, err := integrationCipher(cfg)
	if err != nil {
		return nil, fmt.Errorf("configure mailbox encryption: %w", err)
	}
	// Development can intentionally run without credential encryption. The API
	// then remains unavailable instead of ever persisting a plaintext secret.
	if cipher == nil {
		return nil, nil
	}
	policy := mailbox.DefaultEndpointPolicy()
	policy.AllowPrivate = cfg.MailboxAllowPrivate
	return mailbox.NewService(
		cipher,
		mailbox.EmersionIMAP{Policy: policy},
		mailbox.EmersionSMTP{Policy: policy},
		policy,
	)
}
