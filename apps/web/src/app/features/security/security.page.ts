import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject } from '@angular/core';

import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { ChangePasswordPanel } from './change-password.panel';
import { MFAPanel } from './mfa.panel';
import { SecurityStore } from './security.store';
import { SessionsPanel } from './sessions.panel';

@Component({
  selector: 'app-security-page',
  imports: [ChangePasswordPanel, ErrorPanelComponent, MFAPanel, SessionsPanel],
  providers: [SecurityStore],
  template: `
    <div class="page security-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('identity.security.title') }}</h1>
          <p>{{ i18n.t('identity.security.subtitle') }}</p>
        </div>
      </header>

      @if (store.loadError()) {
        <app-error-panel [error]="store.loadError()" (retry)="store.load()" />
      }

      <app-change-password-panel />

      @if (store.loading()) {
        <section class="panel loading-panel" aria-busy="true">
          <span class="skeleton"></span>
          <span class="skeleton short"></span>
        </section>
      } @else if (store.status()) {
        <app-mfa-panel />
      }

      <app-sessions-panel />
    </div>
  `,
  styles: `
    .security-page {
      max-width: 58rem;
    }
    .loading-panel {
      display: grid;
      gap: 0.75rem;
      padding: 1.25rem;
    }
    .loading-panel .skeleton {
      width: min(22rem, 75%);
    }
    .loading-panel .short {
      width: min(14rem, 50%);
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class SecurityPage implements OnInit {
  readonly i18n = inject(I18nService);
  readonly store = inject(SecurityStore);

  ngOnInit(): void {
    void this.store.load();
  }
}
