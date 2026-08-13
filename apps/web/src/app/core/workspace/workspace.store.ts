import { computed, effect, inject, Injectable, signal } from '@angular/core';

import { AuthStore } from '../auth/auth.store';
import { I18nService } from '../i18n/i18n.service';
import { openAppDatabase } from '../storage/app-database';

interface WorkspacePreference {
  readonly selectedId: string;
  readonly recent: ReadonlyArray<{
    readonly id: string;
    readonly name: string;
    readonly accessedAt: number;
  }>;
}

@Injectable({ providedIn: 'root' })
export class WorkspaceStore {
  private readonly auth = inject(AuthStore);
  private readonly i18n = inject(I18nService);
  private readonly selectedId = signal<string | null>(null);
  private readonly restorePromise: Promise<void>;

  readonly workspaces = computed(() => this.auth.session()?.workspaces ?? []);
  readonly active = computed(() => {
    const workspaces = this.workspaces();
    return (
      workspaces.find((workspace) => workspace.id === this.selectedId()) ?? workspaces[0] ?? null
    );
  });
  readonly id = computed(() => this.active()?.id ?? null);

  constructor() {
    effect(() => this.i18n.setTimeZone(this.active()?.timezone));
    this.restorePromise = this.restore();
  }

  /**
   * Resolves after the persisted tenant preference has been restored.
   * Route guards await this before constructing tenant-scoped feature stores,
   * preventing their first request from using the fallback workspace while
   * IndexedDB is still being read.
   */
  whenReady(): Promise<void> {
    return this.restorePromise;
  }

  async select(workspaceId: string): Promise<void> {
    // A user action can race the asynchronous IndexedDB restore during the
    // first render. Serialize them so the old preference cannot overwrite a
    // newer explicit choice after this method resolves.
    await this.restorePromise;
    const workspace = this.workspaces().find((item) => item.id === workspaceId);
    if (!workspace) return;
    this.selectedId.set(workspaceId);
    await this.i18n.applyPreference(this.auth.user()?.preferredLocale, workspace.defaultLocale);
    // Callers navigate and recreate tenant-scoped feature stores after this
    // promise settles. Await the durable preference so a concurrent reload
    // cannot restore the workspace that was active before the switch.
    await this.persist(workspaceId);
  }

  private async restore(): Promise<void> {
    const preference = await this.readPreference();
    if (preference?.selectedId) this.selectedId.set(preference.selectedId);
  }

  private async persist(workspaceId: string): Promise<void> {
    if (!('indexedDB' in globalThis)) return;
    const active = this.workspaces().find((workspace) => workspace.id === workspaceId);
    if (!active) return;
    const previous = await this.readPreference();
    const recent = [
      { id: active.id, name: active.name, accessedAt: Date.now() },
      ...(previous?.recent ?? []).filter((workspace) => workspace.id !== active.id),
    ].slice(0, 8);
    const database = await openAppDatabase().catch(() => null);
    if (!database) return;
    await new Promise<void>((resolve) => {
      const transaction = database.transaction('preferences', 'readwrite');
      transaction.objectStore('preferences').put({ selectedId: workspaceId, recent }, 'workspace');
      transaction.oncomplete = () => resolve();
      transaction.onerror = () => resolve();
      transaction.onabort = () => resolve();
    });
    database.close();
  }

  private async readPreference(): Promise<WorkspacePreference | null> {
    if (!('indexedDB' in globalThis)) return null;
    const database = await openAppDatabase().catch(() => null);
    if (!database) return null;
    const value = await new Promise<WorkspacePreference | null>((resolve) => {
      const request = database
        .transaction('preferences', 'readonly')
        .objectStore('preferences')
        .get('workspace');
      request.onsuccess = () =>
        resolve(isWorkspacePreference(request.result) ? request.result : null);
      request.onerror = () => resolve(null);
    });
    database.close();
    return value;
  }
}

function isWorkspacePreference(value: unknown): value is WorkspacePreference {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Partial<WorkspacePreference>;
  return typeof candidate.selectedId === 'string' && Array.isArray(candidate.recent);
}
