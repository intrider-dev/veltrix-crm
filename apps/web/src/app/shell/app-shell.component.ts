import { CdkTrapFocus } from '@angular/cdk/a11y';
import { OverlayContainer } from '@angular/cdk/overlay';
import type { ElementRef } from '@angular/core';
import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  HostListener,
  Injector,
  computed,
  effect,
  inject,
  signal,
  viewChild,
} from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatMenuModule } from '@angular/material/menu';
import { MatSelectModule } from '@angular/material/select';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { productConfig } from '@veltrix-crm/product-config';
import {
  Subject,
  catchError,
  debounceTime,
  distinctUntilChanged,
  filter,
  of,
  switchMap,
  tap,
} from 'rxjs';

import { ApiClient } from '../core/api/api-client.service';
import type { SearchResult } from '../core/api/api.types';
import { AuthStore } from '../core/auth/auth.store';
import { Permissions, type Permission } from '../core/auth/permissions';
import type { AppMessageKey } from '../core/i18n/app-message-key';
import { I18nService } from '../core/i18n/i18n.service';
import { NetworkStatusService } from '../core/network/network-status.service';
import { NotificationRealtimeService } from '../core/notifications/notification-realtime.service';
import { AppearanceStore } from '../core/preferences/appearance.store';
import { WorkspaceStore } from '../core/workspace/workspace.store';
import { ChatDockComponent } from '../features/chat/chat-dock.component';
import { focusAfterNextRender } from '../shared/a11y/focus-after-render';
import { BrandLogoComponent } from '../shared/brand/brand-logo.component';
import { ToastViewportComponent } from '../shared/feedback/toast-viewport.component';
import { IconComponent, type IconName } from '../shared/icon/icon.component';
import { usesDarkWorkspace } from './workspace-appearance';

interface NavItem {
  readonly path: string;
  readonly key: AppMessageKey;
  readonly icon: IconName;
  readonly exact?: boolean;
  readonly permission?: Permission;
}

@Component({
  selector: 'app-shell',
  imports: [
    CdkTrapFocus,
    ChatDockComponent,
    BrandLogoComponent,
    IconComponent,
    MatButtonModule,
    MatMenuModule,
    MatSelectModule,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    ToastViewportComponent,
  ],
  template: `
    <a class="skip-link" href="#main-content">{{ i18n.t('common.app.skipToContent') }}</a>
    <div
      class="shell"
      [class.nav-collapsed]="collapsed()"
      [class.mobile-open]="mobileOpen()"
      [class.dark-workspace-route]="darkWorkspaceRoute()"
    >
      <header class="topbar">
        <button
          #mobileMenu
          mat-icon-button
          type="button"
          class="mobile-menu"
          aria-controls="primary-navigation"
          [attr.aria-expanded]="mobileOpen()"
          (click)="openMobileNavigation()"
          [attr.aria-label]="i18n.t('web.shell.openNavigation')"
        >
          <app-icon name="menu" />
        </button>
        <a routerLink="/dashboard" class="mobile-wordmark" [attr.aria-label]="product.productName">
          <app-brand-logo size="small" />
        </a>

        <button
          type="button"
          class="global-search"
          (click)="openCommandPalette()"
          [attr.aria-label]="i18n.t('web.search.label')"
        >
          <app-icon name="search" />
          <span>{{ i18n.t('web.shell.searchPlaceholder') }}</span>
          <kbd>{{ i18n.t('web.shell.commandShortcut') }}</kbd>
        </button>

        <button
          mat-button
          type="button"
          [matMenuTriggerFor]="accountMenu"
          class="account-button"
          [attr.aria-label]="
            i18n.t('web.shell.signedInAs', { name: auth.user()?.displayName ?? '' })
          "
        >
          <span class="account-content">
            <span class="avatar" aria-hidden="true">{{ initials() }}</span>
            <span class="account-name">{{ auth.user()?.displayName }}</span>
          </span>
        </button>
        <mat-menu #accountMenu="matMenu">
          <button mat-menu-item type="button" routerLink="/settings/security">
            {{ i18n.t('settings.settings.security') }}
          </button>
          <button mat-menu-item type="button" routerLink="/workspace/new">
            {{ i18n.t('settings.settings.newWorkspace') }}
          </button>
          <button mat-menu-item type="button" routerLink="/settings">
            {{ i18n.t('common.nav.settings') }}
          </button>
          <button mat-menu-item type="button" (click)="logout()">
            {{ i18n.t('auth.logout.submit') }}
          </button>
        </mat-menu>
      </header>

      @if (!mobileViewport() || mobileOpen()) {
        <aside
          id="primary-navigation"
          class="sidebar"
          [cdkTrapFocus]="mobileViewport() && mobileOpen()"
          [cdkTrapFocusAutoCapture]="mobileViewport() && mobileOpen()"
          [attr.aria-label]="i18n.t('web.shell.navigation')"
        >
          <div class="sidebar-head">
            <a routerLink="/dashboard" class="wordmark" [attr.aria-label]="product.productName">
              <app-brand-logo />
            </a>
            <button
              #mobileNavClose
              cdkFocusInitial
              mat-icon-button
              type="button"
              class="mobile-sidebar-close"
              (click)="closeMobileNavigation()"
              [attr.aria-label]="i18n.t('web.shell.closeNavigation')"
            >
              <app-icon name="close" />
            </button>
          </div>
          <div class="workspace-control">
            @if (workspace.active(); as activeWorkspace) {
              <mat-select
                class="workspace-select"
                panelClass="workspace-select-panel"
                disableOptionCentering
                [value]="activeWorkspace.id"
                [disabled]="workspaceSwitching()"
                (selectionChange)="switchWorkspace($event.value)"
                [aria-label]="i18n.t('common.nav.workspace')"
              >
                @for (item of workspace.workspaces(); track item.id) {
                  <mat-option [value]="item.id" [attr.title]="item.name">{{
                    item.name
                  }}</mat-option>
                }
              </mat-select>
            } @else {
              <a mat-stroked-button routerLink="/workspace/new" class="workspace-empty">
                {{ i18n.t('settings.settings.newWorkspace') }}
              </a>
            }
          </div>
          <nav>
            @for (item of visibleNavItems(); track item.path) {
              <a
                [routerLink]="item.path"
                routerLinkActive="active"
                [routerLinkActiveOptions]="{ exact: item.exact ?? false }"
                [attr.aria-label]="i18n.t(item.key)"
              >
                <app-icon [name]="item.icon" />
                <span>{{ i18n.t(item.key) }}</span>
              </a>
            }
          </nav>
          <button
            type="button"
            class="collapse-button"
            (click)="collapsed.set(!collapsed())"
            [attr.aria-label]="
              i18n.t(collapsed() ? 'web.shell.expandNavigation' : 'web.shell.collapseNavigation')
            "
          >
            <app-icon name="chevron" /><span>{{ i18n.t('web.shell.collapseNavigation') }}</span>
          </button>
        </aside>
      }

      @if (mobileOpen()) {
        <button
          class="scrim"
          type="button"
          (click)="closeMobileNavigation()"
          [attr.aria-label]="i18n.t('web.shell.closeNavigation')"
        ></button>
      }

      <main #mainContent id="main-content" tabindex="-1" [attr.aria-busy]="workspaceSwitching()">
        <router-outlet />
      </main>
      @if (!network.online()) {
        <div class="offline-indicator" role="status" aria-live="polite">
          {{ i18n.t('pwa.offline') }}
        </div>
      }
    </div>

    <app-toast-viewport />
    <app-chat-dock />

    @if (commandOpen()) {
      <section class="palette-backdrop" role="presentation" (mousedown)="closeFromBackdrop($event)">
        <div
          class="palette"
          role="dialog"
          aria-modal="true"
          cdkTrapFocus
          [cdkTrapFocusAutoCapture]="true"
          [attr.aria-label]="i18n.t('web.shell.commandPalette')"
        >
          <div class="palette-search">
            <app-icon name="search" />
            <input
              #searchInput
              type="search"
              autocomplete="off"
              [attr.aria-label]="i18n.t('web.search.label')"
              [placeholder]="i18n.t('web.shell.searchPlaceholder')"
              (input)="search($event)"
            />
            <button
              mat-icon-button
              type="button"
              (click)="closeCommandPalette()"
              [attr.aria-label]="i18n.t('common.action.close')"
            >
              <app-icon name="close" />
            </button>
          </div>
          <div class="palette-results" aria-live="polite">
            @if (searchPending()) {
              <p>{{ i18n.t('common.result.loading') }}</p>
            } @else if (searchQuery().length < 2) {
              <p>{{ i18n.t('web.search.minimum') }}</p>
            } @else if (searchResults().length === 0) {
              <p>{{ i18n.t('web.search.empty') }}</p>
            } @else {
              @for (result of searchResults(); track result.entityType + result.entityId) {
                <a [routerLink]="searchLink(result)" (click)="closeCommandPalette()">
                  <span class="result-type">{{ i18n.t(entityTypeKey(result.entityType)) }}</span>
                  <strong>{{ result.title }}</strong>
                  @if (result.subtitle) {
                    <small>{{ result.subtitle }}</small>
                  }
                </a>
              }
            }
          </div>
        </div>
      </section>
    }
  `,
  styleUrl: './app-shell.component.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class AppShellComponent {
  readonly auth = inject(AuthStore);
  readonly i18n = inject(I18nService);
  readonly network = inject(NetworkStatusService);
  readonly notifications = inject(NotificationRealtimeService);
  readonly appearance = inject(AppearanceStore);
  readonly workspace = inject(WorkspaceStore);
  readonly permissions = inject(Permissions);
  readonly product = productConfig;
  readonly collapsed = signal(false);
  readonly mobileOpen = signal(false);
  readonly mobileViewport = signal(false);
  readonly commandOpen = signal(false);
  readonly darkWorkspaceRoute = computed(
    () => this.appearance.dark() || this.routeUsesDarkWorkspace(),
  );
  readonly searchPending = signal(false);
  readonly searchQuery = signal('');
  readonly searchResults = signal<readonly SearchResult[]>([]);
  readonly searchInput = viewChild<ElementRef<HTMLInputElement>>('searchInput');
  readonly mainContent = viewChild<ElementRef<HTMLElement>>('mainContent');
  readonly mobileMenu = viewChild<ElementRef<HTMLButtonElement>>('mobileMenu');
  readonly mobileNavClose = viewChild.required<ElementRef<HTMLButtonElement>>('mobileNavClose');
  readonly workspaceSwitching = signal(false);
  readonly navItems: readonly NavItem[] = [
    { path: '/dashboard', key: 'common.nav.dashboard', icon: 'dashboard', exact: true },
    { path: '/contacts', key: 'common.nav.contacts', icon: 'contact' },
    { path: '/companies', key: 'common.nav.companies', icon: 'building' },
    { path: '/leads', key: 'common.nav.leads', icon: 'lead', permission: 'leads.read' },
    { path: '/deals', key: 'common.nav.deals', icon: 'deal', permission: 'deals.read' },
    { path: '/projects', key: 'common.nav.projects', icon: 'project' },
    { path: '/activities', key: 'common.nav.activities', icon: 'activity' },
    { path: '/calendar', key: 'common.nav.calendar', icon: 'calendar' },
    { path: '/mail', key: 'common.nav.mailbox', icon: 'mail' },
    {
      path: '/automations',
      key: 'common.nav.automations',
      icon: 'automation',
      permission: 'settings.write',
    },
    { path: '/reports', key: 'common.nav.reports', icon: 'report', permission: 'reports.read' },
    { path: '/notifications', key: 'common.nav.notifications', icon: 'notification' },
    { path: '/settings', key: 'common.nav.settings', icon: 'settings', exact: true },
  ];
  readonly visibleNavItems = computed(() =>
    this.navItems.filter((item) => !item.permission || this.permissions.allows(item.permission)),
  );

  private readonly api = inject(ApiClient);
  private readonly router = inject(Router);
  private readonly destroyRef = inject(DestroyRef);
  private readonly injector = inject(Injector);
  private readonly overlayContainer = inject(OverlayContainer);
  private readonly searchTerms = new Subject<string>();
  private readonly routeUsesDarkWorkspace = signal(false);

  constructor() {
    this.syncWorkspaceAppearance();
    effect((onCleanup) => {
      const container = this.overlayContainer.getContainerElement();
      container.classList.toggle('dark-workspace-overlay', this.darkWorkspaceRoute());
      onCleanup(() => container.classList.remove('dark-workspace-overlay'));
    });
    if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      const mobileQuery = window.matchMedia('(max-width: 820px)');
      const syncMobileViewport = (matches: boolean) => {
        this.mobileViewport.set(matches);
        if (!matches) this.mobileOpen.set(false);
      };
      syncMobileViewport(mobileQuery.matches);
      const listener = (event: MediaQueryListEvent) => syncMobileViewport(event.matches);
      mobileQuery.addEventListener('change', listener);
      this.destroyRef.onDestroy(() => mobileQuery.removeEventListener('change', listener));
    }
    this.router.events
      .pipe(
        filter((event): event is NavigationEnd => event instanceof NavigationEnd),
        takeUntilDestroyed(),
      )
      .subscribe(() => {
        this.syncWorkspaceAppearance();
        this.mobileOpen.set(false);
        // The shell main element is stable across child-route navigation, so
        // it does not need an Angular after-render registration per route.
        queueMicrotask(() => this.mainContent()?.nativeElement.focus());
      });
    this.searchTerms
      .pipe(
        debounceTime(150),
        distinctUntilChanged(),
        tap((query) => {
          this.searchQuery.set(query);
          this.searchPending.set(query.length >= 2);
        }),
        switchMap((query) => {
          const workspaceId = this.workspace.id();
          if (query.length < 2 || !workspaceId) return of([]);
          return this.api.searchStream(workspaceId, query).pipe(catchError(() => of([])));
        }),
        takeUntilDestroyed(this.destroyRef),
      )
      .subscribe((results) => {
        this.searchResults.set(results);
        this.searchPending.set(false);
      });
  }

  private syncWorkspaceAppearance(): void {
    this.routeUsesDarkWorkspace.set(usesDarkWorkspace(this.router.url));
  }

  readonly initials = () => {
    const name = this.auth.user()?.displayName ?? '';
    return name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase())
      .join('');
  };

  @HostListener('document:keydown', ['$event'])
  onKeydown(event: KeyboardEvent): void {
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      if (this.commandOpen()) this.closeCommandPalette();
      else this.openCommandPalette();
    } else if (event.key === 'Escape') {
      const target = event.target instanceof Element ? event.target : null;
      if (target?.closest('[role="combobox"][aria-expanded="true"]')) return;
      if (this.commandOpen()) this.closeCommandPalette();
      else if (this.mobileOpen()) this.closeMobileNavigation();
    }
  }

  openMobileNavigation(): void {
    this.mobileOpen.set(true);
    focusAfterNextRender(this.injector, () => this.mobileNavClose().nativeElement);
  }

  closeMobileNavigation(): void {
    this.mobileOpen.set(false);
    focusAfterNextRender(this.injector, () => this.mobileMenu()?.nativeElement);
  }

  openCommandPalette(): void {
    this.commandOpen.set(true);
    focusAfterNextRender(this.injector, () => this.searchInput()?.nativeElement);
  }

  closeCommandPalette(): void {
    this.commandOpen.set(false);
    this.searchQuery.set('');
    this.searchResults.set([]);
  }

  closeFromBackdrop(event: MouseEvent): void {
    if (event.target === event.currentTarget) this.closeCommandPalette();
  }

  search(event: Event): void {
    this.searchTerms.next((event.target as HTMLInputElement).value.trim());
  }

  async switchWorkspace(workspaceId: string): Promise<void> {
    if (this.workspaceSwitching() || workspaceId === this.workspace.id()) return;
    this.workspaceSwitching.set(true);
    try {
      // Destroy the current feature before changing tenant context so no old
      // feature cache or SSE subscription can remain visible under the new label.
      await this.router.navigateByUrl('/settings', { skipLocationChange: true });
      await this.workspace.select(workspaceId);
      this.searchQuery.set('');
      this.searchResults.set([]);
      await this.router.navigateByUrl('/dashboard');
    } finally {
      this.workspaceSwitching.set(false);
    }
  }

  searchLink(result: SearchResult): string {
    if (result.entityType === 'contact') return `/contacts/${result.entityId}`;
    if (result.entityType === 'company') return `/companies/${result.entityId}`;
    if (result.entityType === 'deal') return `/deals/${result.entityId}`;
    if (result.entityType === 'lead') return '/leads';
    return '/activities';
  }

  entityTypeKey(entityType: SearchResult['entityType']): AppMessageKey {
    return `web.entity.${entityType}` as AppMessageKey;
  }

  async logout(): Promise<void> {
    await this.auth.logout();
    await this.router.navigateByUrl('/login');
  }
}
