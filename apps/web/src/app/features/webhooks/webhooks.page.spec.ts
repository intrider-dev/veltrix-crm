import { signal } from '@angular/core';
import type { ComponentFixture } from '@angular/core/testing';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import { Permissions } from '../../core/auth/permissions';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { WebhooksPage } from './webhooks.page';
import { WebhooksStore } from './webhooks.store';

describe('WebhooksPage', () => {
  it('does not request integration data without settings permission', async () => {
    const api = { listWebhooks: vi.fn() };
    const fixture = await render(api, false);
    const element = fixture.nativeElement as HTMLElement;

    expect(api.listWebhooks).not.toHaveBeenCalled();
    expect(element.querySelector('[role="alert"]')?.textContent).toContain(
      'integrations.permission',
    );
  });

  it('renders a persistent retryable error when loading fails', async () => {
    const api = { listWebhooks: vi.fn().mockRejectedValue(new Error('offline')) };
    const fixture = await render(api, true);
    const element = fixture.nativeElement as HTMLElement;

    const alert = element.querySelector('[role="alert"]');
    expect(alert?.textContent).toContain('web.status.error');
    expect(alert?.querySelector('button')).not.toBeNull();
  });

  it('removes a one-time signing secret from the DOM when dismissed', async () => {
    const api = {
      listWebhooks: vi.fn().mockResolvedValue([]),
      listWebhookDeliveries: vi.fn().mockResolvedValue({ items: [] }),
    };
    const fixture = await render(api, true);
    const element = fixture.nativeElement as HTMLElement;
    const store = fixture.debugElement.injector.get(WebhooksStore);
    store.revealedSecret.set({
      value: 'whsec_ephemeral',
      subscriptionUrl: 'https://hooks.example.test/crm',
      reason: 'rotated',
    });
    fixture.detectChanges();
    expect(element.querySelector('.secret-panel code')?.textContent).toContain('whsec_ephemeral');

    store.dismissSecret();
    fixture.detectChanges();
    expect(element.querySelector('.secret-panel')).toBeNull();
  });
});

async function render(api: object, allowed: boolean): Promise<ComponentFixture<WebhooksPage>> {
  await TestBed.configureTestingModule({
    imports: [WebhooksPage],
    providers: [
      { provide: ApiClient, useValue: api },
      { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
      { provide: Permissions, useValue: { allows: () => allowed } },
      {
        provide: I18nService,
        useValue: {
          t: (key: string) => key,
          problem: (code: string) => code,
          date: (value: string) => value,
        },
      },
    ],
  }).compileComponents();
  const fixture = TestBed.createComponent(WebhooksPage);
  fixture.detectChanges();
  await fixture.whenStable();
  fixture.detectChanges();
  return fixture;
}
