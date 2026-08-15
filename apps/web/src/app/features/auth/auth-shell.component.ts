import {
  ChangeDetectionStrategy,
  Component,
  HostListener,
  inject,
  input,
  signal,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { productConfig, type SupportedLocale } from '@veltrix-crm/product-config';

import { I18nService } from '../../core/i18n/i18n.service';
import { BrandLogoComponent } from '../../shared/brand/brand-logo.component';
import { IconComponent } from '../../shared/icon/icon.component';

@Component({
  selector: 'app-auth-shell',
  imports: [BrandLogoComponent, IconComponent, MatButtonModule],
  template: `
    <main class="auth-shell">
      <section class="brand-story" aria-labelledby="auth-product-story">
        <div class="aurora" aria-hidden="true"></div>
        <app-brand-logo size="large" />
        <div class="story-copy">
          <h1 id="auth-product-story">{{ i18n.t('auth.hero.title') }}</h1>
          <p>{{ i18n.t('auth.hero.subtitle') }}</p>
        </div>
        <ul class="benefits" aria-label="Product capabilities">
          <li>
            <span class="benefit-icon"><app-icon name="report" /></span>
            <span>
              <strong>{{ i18n.t('auth.hero.workspaceTitle') }}</strong>
              <small>{{ i18n.t('auth.hero.workspaceCopy') }}</small>
            </span>
          </li>
          <li>
            <span class="benefit-icon"><app-icon name="automation" /></span>
            <span>
              <strong>{{ i18n.t('auth.hero.automationTitle') }}</strong>
              <small>{{ i18n.t('auth.hero.automationCopy') }}</small>
            </span>
          </li>
          <li>
            <span class="benefit-icon"><app-icon name="contact" /></span>
            <span>
              <strong>{{ i18n.t('auth.hero.teamTitle') }}</strong>
              <small>{{ i18n.t('auth.hero.teamCopy') }}</small>
            </span>
          </li>
        </ul>
        <div class="mountains" aria-hidden="true"><span></span><span></span><span></span></div>
      </section>

      <section class="auth-pane">
        <div class="locale-wrap">
          <div class="locale-control">
            <app-icon name="language" />
            <button
              class="locale-trigger"
              type="button"
              [attr.aria-expanded]="localeOpen()"
              aria-controls="locale-options"
              (click)="toggleLocale($event)"
            >
              {{ i18n.languageName(i18n.locale()) }}
              <span class="locale-chevron" aria-hidden="true"></span>
            </button>
            @if (localeOpen()) {
              <div
                id="locale-options"
                class="locale-menu"
                role="group"
                [attr.aria-label]="i18n.t('settings.language.label')"
              >
                @for (locale of i18n.supportedLocales; track locale) {
                  <button
                    type="button"
                    [attr.aria-label]="i18n.languageName(locale)"
                    [attr.aria-pressed]="i18n.locale() === locale"
                    (click)="changeLocale(locale)"
                  >
                    {{ i18n.languageName(locale) }}
                    @if (i18n.locale() === locale) {
                      <app-icon name="check" />
                    }
                  </button>
                }
              </div>
            }
          </div>
        </div>

        <div class="auth-card">
          <header>
            <h2>{{ title() }}</h2>
            <p>{{ subtitle() }}</p>
          </header>
          <ng-content />
        </div>

        <footer>
          <span><app-icon name="shield" /> {{ i18n.t('auth.footer.secure') }}</span>
          <span>© 2026 {{ product.productName }}</span>
        </footer>
      </section>
    </main>
  `,
  styleUrl: './auth-shell.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AuthShellComponent {
  readonly title = input.required<string>();
  readonly subtitle = input.required<string>();
  readonly i18n = inject(I18nService);
  readonly product = productConfig;
  readonly localeOpen = signal(false);

  toggleLocale(event: MouseEvent): void {
    event.stopPropagation();
    this.localeOpen.update((open) => !open);
  }

  async changeLocale(locale: SupportedLocale): Promise<void> {
    this.localeOpen.set(false);
    await this.i18n.setLocale(locale);
  }

  @HostListener('document:click')
  @HostListener('document:keydown.escape')
  closeLocaleMenu(): void {
    this.localeOpen.set(false);
  }
}
