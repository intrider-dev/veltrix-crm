import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';

import { ApiClient } from '../../core/api/api-client.service';
import { I18nService } from '../../core/i18n/i18n.service';
import { PasswordResetPage } from './password-reset.page';
import { RegistrationPage } from './registration.page';

const i18nStub = {
  locale: signal<'en' | 'ru'>('en').asReadonly(),
  supportedLocales: ['en', 'ru'] as const,
  languageName: (locale: string) => locale,
  problem: () => 'Translated error',
  t: (key: string) => key,
};

describe('RegistrationPage', () => {
  it('submits a valid development registration and exposes the created account', async () => {
    const registered = {
      id: '018f0000-0000-7000-8000-000000000001',
      email: 'ada@example.test',
      displayName: 'Ada Lovelace',
      preferredLocale: 'en' as const,
    };
    const api = { registerDevelopmentUser: vi.fn().mockResolvedValue(registered) };
    await TestBed.configureTestingModule({
      imports: [RegistrationPage],
      providers: [
        provideRouter([]),
        { provide: ApiClient, useValue: api },
        { provide: I18nService, useValue: i18nStub },
      ],
    }).compileComponents();
    const page = TestBed.createComponent(RegistrationPage).componentInstance;
    page.model.set({
      email: 'ada@example.test',
      displayName: ' Ada Lovelace ',
      password: 'correct-horse',
      confirmPassword: 'correct-horse',
      locale: 'en',
    });

    await page.submit(new SubmitEvent('submit'));

    expect(api.registerDevelopmentUser).toHaveBeenCalledWith({
      email: 'ada@example.test',
      displayName: 'Ada Lovelace',
      password: 'correct-horse',
      locale: 'en',
    });
    expect(page.registered()).toEqual(registered);
  });
});

describe('PasswordResetPage', () => {
  it('uses the query token when confirming a new password', async () => {
    const api = { confirmPasswordReset: vi.fn().mockResolvedValue(undefined) };
    await TestBed.configureTestingModule({
      imports: [PasswordResetPage],
      providers: [
        provideRouter([]),
        {
          provide: ActivatedRoute,
          useValue: { snapshot: { queryParamMap: convertToParamMap({ token: 'reset-token' }) } },
        },
        { provide: ApiClient, useValue: api },
        { provide: I18nService, useValue: i18nStub },
      ],
    }).compileComponents();
    const page = TestBed.createComponent(PasswordResetPage).componentInstance;
    page.confirmationModel.set({
      newPassword: 'correct-horse',
      confirmPassword: 'correct-horse',
    });

    await page.confirm(new SubmitEvent('submit'));

    expect(api.confirmPasswordReset).toHaveBeenCalledWith({
      token: 'reset-token',
      newPassword: 'correct-horse',
    });
    expect(page.complete()).toBe(true);
  });
});
