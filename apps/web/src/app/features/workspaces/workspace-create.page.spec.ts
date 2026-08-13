import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';

import { ApiClient } from '../../core/api/api-client.service';
import { AuthStore } from '../../core/auth/auth.store';
import { I18nService } from '../../core/i18n/i18n.service';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { InvitationAcceptPage } from './invitation-accept.page';
import { WorkspaceCreatePage } from './workspace-create.page';

describe('WorkspaceCreatePage', () => {
  it('refreshes the session and selects the newly created workspace', async () => {
    const created = {
      id: '018f0000-0000-7000-8000-000000000010',
      name: 'Research',
      slug: 'research',
      role: 'owner' as const,
      defaultLocale: 'en' as const,
      supportedLocales: ['en', 'ru'] as const,
      timezone: 'UTC',
      defaultCurrency: 'USD',
      version: 1,
    };
    const api = { createWorkspace: vi.fn().mockResolvedValue(created) };
    const auth = { refreshSession: vi.fn().mockResolvedValue(true) };
    const workspace = {
      id: signal<string | null>(null),
      select: vi.fn().mockResolvedValue(undefined),
    };
    const router = { navigateByUrl: vi.fn().mockResolvedValue(true) };
    const i18n = {
      locale: signal<'en' | 'ru'>('en').asReadonly(),
      supportedLocales: ['en', 'ru'] as const,
      languageName: (locale: string) => locale,
      problem: () => 'Translated error',
      t: (key: string) => key,
    };
    await TestBed.configureTestingModule({
      imports: [WorkspaceCreatePage],
      providers: [
        { provide: ApiClient, useValue: api },
        { provide: AuthStore, useValue: auth },
        { provide: WorkspaceStore, useValue: workspace },
        { provide: Router, useValue: router },
        { provide: I18nService, useValue: i18n },
      ],
    }).compileComponents();
    const page = TestBed.createComponent(WorkspaceCreatePage).componentInstance;
    page.model.set({
      name: ' Research ',
      slug: 'research',
      defaultLocale: 'en',
      timezone: 'UTC',
      defaultCurrency: 'USD',
    });

    await page.submit(new SubmitEvent('submit'));

    expect(api.createWorkspace).toHaveBeenCalledWith({
      name: 'Research',
      slug: 'research',
      defaultLocale: 'en',
      timezone: 'UTC',
      defaultCurrency: 'USD',
    });
    expect(auth.refreshSession).toHaveBeenCalledOnce();
    expect(workspace.select).toHaveBeenCalledWith(created.id);
    expect(router.navigateByUrl).toHaveBeenCalledWith('/dashboard');
  });
});

describe('InvitationAcceptPage', () => {
  it('accepts the query token and refreshes workspace memberships', async () => {
    const api = { acceptInvitation: vi.fn().mockResolvedValue({ id: 'membership-1' }) };
    const auth = { refreshSession: vi.fn().mockResolvedValue(true) };
    const router = { navigateByUrl: vi.fn().mockResolvedValue(true), navigate: vi.fn() };
    const i18n = {
      problem: () => 'Translated error',
      t: (key: string) => key,
    };
    await TestBed.configureTestingModule({
      imports: [InvitationAcceptPage],
      providers: [
        {
          provide: ActivatedRoute,
          useValue: { snapshot: { queryParamMap: convertToParamMap({ token: 'invite-token' }) } },
        },
        { provide: ApiClient, useValue: api },
        { provide: AuthStore, useValue: auth },
        { provide: Router, useValue: router },
        { provide: I18nService, useValue: i18n },
      ],
    }).compileComponents();
    const page = TestBed.createComponent(InvitationAcceptPage).componentInstance;

    await page.accept();

    expect(api.acceptInvitation).toHaveBeenCalledWith({ token: 'invite-token' });
    expect(auth.refreshSession).toHaveBeenCalledOnce();
    expect(page.accepted()).toBe(true);
  });
});
