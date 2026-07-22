import { DestroyRef, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  BulkResult,
  Contact,
  ContactImportMapping,
  ContactImportPreview,
  ContactImportStatus,
  CreateContact,
  CustomFieldDefinition,
  DeletedRecordPage,
  SavedView,
  Tag,
  WorkspaceMember,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

export type ContactListMode = 'contacts' | 'trash';
export type ContactStatusFilter = 'all' | 'active' | 'inactive';
export type DeletedContact = DeletedRecordPage['items'][number];

@Injectable()
export class ContactsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly destroyRef = inject(DestroyRef);
  private importTimer: ReturnType<typeof setTimeout> | null = null;
  private destroyed = false;

  readonly contacts = signal<readonly Contact[]>([]);
  readonly nextCursor = signal<string | null>(null);
  readonly trash = signal<readonly DeletedContact[]>([]);
  readonly trashNextCursor = signal<string | null>(null);
  readonly savedViews = signal<readonly SavedView[]>([]);
  readonly members = signal<readonly WorkspaceMember[]>([]);
  readonly tags = signal<readonly Tag[]>([]);
  readonly customFields = signal<readonly CustomFieldDefinition[]>([]);
  readonly loading = signal(false);
  readonly creating = signal(false);
  readonly referencesLoading = signal(false);
  readonly operationPending = signal(false);
  readonly operationError = signal<unknown>(null);
  readonly operationResult = signal<BulkResult | null>(null);
  readonly error = signal<unknown>(null);
  readonly query = signal('');
  readonly status = signal<ContactStatusFilter>('all');
  readonly mode = signal<ContactListMode>('contacts');
  readonly importPreview = signal<ContactImportPreview | null>(null);
  readonly importStatus = signal<ContactImportStatus | null>(null);
  readonly importBusy = signal(false);
  readonly importError = signal<unknown>(null);
  readonly importErrorsUrl = signal<string | null>(null);

  constructor() {
    this.destroyRef.onDestroy(() => {
      this.destroyed = true;
      this.clearImportTimer();
    });
  }

  async load(reset = true): Promise<void> {
    if (this.mode() === 'trash') {
      await this.loadTrash(reset);
      return;
    }
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.loading()) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listContacts(workspaceId, {
        cursor: reset ? undefined : (this.nextCursor() ?? undefined),
        query: this.query().trim() || undefined,
        status: this.status() === 'all' ? undefined : this.status(),
      });
      this.contacts.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async loadReferences(canReadMembers: boolean): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.referencesLoading()) return;
    this.referencesLoading.set(true);
    this.operationError.set(null);
    try {
      const [views, tags, customFields, members] = await Promise.all([
        this.api.listSavedViews(workspaceId, 'contact'),
        this.api.listTags(workspaceId),
        this.api.listCustomFields(workspaceId, 'contact'),
        canReadMembers ? this.api.listMembers(workspaceId) : Promise.resolve([]),
      ]);
      this.savedViews.set(views);
      this.tags.set(tags);
      this.customFields.set(customFields);
      this.members.set(members.filter((member) => member.status === 'active'));
    } catch (error) {
      this.operationError.set(error);
    } finally {
      this.referencesLoading.set(false);
    }
  }

  async setMode(mode: ContactListMode): Promise<void> {
    if (mode === this.mode()) return;
    this.mode.set(mode);
    this.operationResult.set(null);
    await this.load(true);
  }

  async applySavedView(view: SavedView | null): Promise<void> {
    if (!view) {
      this.query.set('');
      this.status.set('all');
    } else {
      const text = view.definition.filters.find(
        (filter) => filter.field === 'displayName' && filter.operator === 'contains',
      )?.value;
      const status = view.definition.filters.find(
        (filter) => filter.field === 'status' && filter.operator === 'eq',
      )?.value;
      this.query.set(typeof text === 'string' ? text : '');
      this.status.set(status === 'active' || status === 'inactive' ? status : 'all');
    }
    this.mode.set('contacts');
    await this.load(true);
  }

  async createSavedView(name: string): Promise<SavedView> {
    const workspaceId = this.requiredWorkspace();
    return this.runOperation(async () => {
      const filters: SavedView['definition']['filters'] = [];
      const query = this.query().trim();
      if (query) filters.push({ field: 'displayName', operator: 'contains', value: query });
      if (this.status() !== 'all') {
        filters.push({ field: 'status', operator: 'eq', value: this.status() });
      }
      const created = await this.api.createSavedView(workspaceId, {
        entityType: 'contact',
        name: name.trim(),
        definition: {
          filters,
          sort: [{ field: 'updatedAt', direction: 'desc' }],
          columns: ['displayName', 'email', 'companyId', 'status'],
        },
        isShared: false,
      });
      this.savedViews.update((views) => [...views, created]);
      return created;
    });
  }

  async deleteSavedView(view: SavedView): Promise<void> {
    const workspaceId = this.requiredWorkspace();
    await this.runOperation(async () => {
      await this.api.deleteSavedView(workspaceId, view);
      this.savedViews.update((views) => views.filter((item) => item.id !== view.id));
    });
  }

  async bulkAssign(records: readonly Contact[], ownerId: string | null): Promise<BulkResult> {
    return this.runOperation(async () => {
      const result = await this.api.bulkAssignContacts(
        this.requiredWorkspace(),
        this.versioned(records),
        ownerId,
      );
      await this.load(true);
      this.operationResult.set(result);
      return result;
    });
  }

  async bulkTag(
    records: readonly Contact[],
    tagIds: readonly string[],
    mode: 'add' | 'remove' | 'replace',
  ): Promise<BulkResult> {
    return this.runOperation(async () => {
      const result = await this.api.bulkTagContacts(
        this.requiredWorkspace(),
        this.versioned(records),
        tagIds,
        mode,
      );
      await this.load(true);
      this.operationResult.set(result);
      return result;
    });
  }

  async bulkDelete(records: readonly Contact[]): Promise<BulkResult> {
    return this.runOperation(async () => {
      const result = await this.api.bulkDeleteContacts(
        this.requiredWorkspace(),
        this.versioned(records),
      );
      await this.load(true);
      this.operationResult.set(result);
      return result;
    });
  }

  async restore(record: DeletedContact): Promise<void> {
    await this.runOperation(() =>
      this.api.restoreContact(this.requiredWorkspace(), record.id, record.version),
    );
    this.trash.update((records) => records.filter((item) => item.id !== record.id));
  }

  async exportCurrentView(): Promise<Blob> {
    return this.runOperation(() =>
      this.api.exportContacts(this.requiredWorkspace(), {
        query: this.query().trim() || undefined,
        status: this.status() === 'all' ? undefined : this.status(),
      }),
    );
  }

  async previewImport(file: File): Promise<ContactImportPreview> {
    this.importBusy.set(true);
    this.importError.set(null);
    this.importStatus.set(null);
    this.importErrorsUrl.set(null);
    this.clearImportTimer();
    try {
      const preview = await this.api.previewContactImport(this.requiredWorkspace(), file);
      this.importPreview.set(preview);
      return preview;
    } catch (error) {
      this.importError.set(error);
      throw error;
    } finally {
      this.importBusy.set(false);
    }
  }

  async queueImport(mapping: ContactImportMapping): Promise<ContactImportStatus> {
    const preview = this.importPreview();
    if (!preview) throw new Error('contact.import.previewRequired');
    this.importBusy.set(true);
    this.importError.set(null);
    try {
      const status = await this.api.queueContactImport(
        this.requiredWorkspace(),
        preview.id,
        mapping,
      );
      this.importStatus.set(status);
      this.importErrorsUrl.set(
        this.api.contactImportErrorsUrl(this.requiredWorkspace(), preview.id),
      );
      if (this.isImportTerminal(status)) {
        await this.finishImport(status);
      } else {
        this.scheduleImportPoll(preview.id);
      }
      return status;
    } catch (error) {
      this.importError.set(error);
      throw error;
    } finally {
      this.importBusy.set(false);
    }
  }

  resetImport(): void {
    this.clearImportTimer();
    this.importPreview.set(null);
    this.importStatus.set(null);
    this.importError.set(null);
    this.importErrorsUrl.set(null);
  }

  retryImport(): void {
    const preview = this.importPreview();
    const status = this.importStatus();
    if (!preview || (status && this.isImportTerminal(status))) return;
    this.importError.set(null);
    void this.pollImport(preview.id);
  }

  clearOperationResult(): void {
    this.operationResult.set(null);
    this.operationError.set(null);
  }

  async create(body: CreateContact): Promise<Contact> {
    const workspaceId = this.requiredWorkspace();
    this.creating.set(true);
    try {
      const contact = await this.api.createContact(workspaceId, body);
      this.contacts.update((contacts) => [contact, ...contacts]);
      return contact;
    } finally {
      this.creating.set(false);
    }
  }

  private async loadTrash(reset: boolean): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.loading()) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listContactTrash(
        workspaceId,
        reset ? undefined : (this.trashNextCursor() ?? undefined),
      );
      this.trash.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.trashNextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  private scheduleImportPoll(importId: string): void {
    this.clearImportTimer();
    this.importTimer = setTimeout(() => {
      this.importTimer = null;
      void this.pollImport(importId);
    }, 1500);
  }

  private async pollImport(importId: string): Promise<void> {
    if (this.destroyed) return;
    try {
      const status = await this.api.getContactImport(this.requiredWorkspace(), importId);
      this.importStatus.set(status);
      if (this.isImportTerminal(status)) await this.finishImport(status);
      else this.scheduleImportPoll(importId);
    } catch (error) {
      this.importError.set(error);
    }
  }

  private async finishImport(status: ContactImportStatus): Promise<void> {
    this.clearImportTimer();
    if (status.status === 'completed') {
      this.mode.set('contacts');
      await this.load(true);
    }
  }

  private isImportTerminal(status: ContactImportStatus): boolean {
    return status.status === 'completed' || status.status === 'failed';
  }

  private clearImportTimer(): void {
    if (this.importTimer === null) return;
    clearTimeout(this.importTimer);
    this.importTimer = null;
  }

  private versioned(records: readonly Contact[]): ReadonlyArray<{ id: string; version: number }> {
    return records.map(({ id, version }) => ({ id, version }));
  }

  private requiredWorkspace(): string {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace-required');
    return workspaceId;
  }

  private async runOperation<T>(operation: () => Promise<T>): Promise<T> {
    this.operationPending.set(true);
    this.operationError.set(null);
    try {
      return await operation();
    } catch (error) {
      this.operationError.set(error);
      throw error;
    } finally {
      this.operationPending.set(false);
    }
  }
}
