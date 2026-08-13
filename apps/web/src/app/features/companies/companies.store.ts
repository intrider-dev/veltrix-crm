import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  Company,
  CreateCompany,
  DeletedRecordPage,
  SavedView,
  SavedViewInput,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

export type CompanyListMode = 'companies' | 'trash';
export type CompanyStatusFilter = 'all' | 'active' | 'inactive';
export type DeletedCompany = DeletedRecordPage['items'][number];

@Injectable()
export class CompaniesStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly companies = signal<readonly Company[]>([]);
  readonly nextCursor = signal<string | null>(null);
  readonly trash = signal<readonly DeletedCompany[]>([]);
  readonly trashNextCursor = signal<string | null>(null);
  readonly savedViews = signal<readonly SavedView[]>([]);
  readonly query = signal('');
  readonly status = signal<CompanyStatusFilter>('all');
  readonly mode = signal<CompanyListMode>('companies');
  readonly loading = signal(false);
  readonly referencesLoading = signal(false);
  readonly creating = signal(false);
  readonly operationPending = signal(false);
  readonly error = signal<unknown>(null);
  readonly operationError = signal<unknown>(null);

  async initialize(): Promise<void> {
    await Promise.all([this.load(true), this.loadSavedViews()]);
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
      const page = await this.api.listCompanies(workspaceId, {
        cursor: reset ? undefined : (this.nextCursor() ?? undefined),
        query: this.query().trim() || undefined,
        status: this.status() === 'all' ? undefined : this.status(),
      });
      this.companies.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async setMode(mode: CompanyListMode): Promise<void> {
    if (this.mode() === mode) return;
    this.mode.set(mode);
    await this.load(true);
  }

  async setStatus(status: CompanyStatusFilter): Promise<void> {
    if (this.status() === status) return;
    this.status.set(status);
    await this.load(true);
  }

  async create(body: CreateCompany): Promise<Company> {
    const workspaceId = this.requiredWorkspace();
    this.creating.set(true);
    try {
      const company = await this.api.createCompany(workspaceId, body);
      if (this.mode() === 'companies' && this.matchesCurrentStatus(company)) {
        this.companies.update((companies) => [company, ...companies]);
      }
      return company;
    } finally {
      this.creating.set(false);
    }
  }

  async restore(company: DeletedCompany): Promise<void> {
    await this.runOperation(async () => {
      await this.api.restoreCompany(this.requiredWorkspace(), company.id, company.version);
      this.trash.update((items) => items.filter((item) => item.id !== company.id));
    });
  }

  async saveCurrentView(name: string): Promise<SavedView> {
    const filters: SavedViewInput['definition']['filters'] = [];
    if (this.query().trim()) {
      filters.push({ field: 'name', operator: 'contains', value: this.query().trim() });
    }
    if (this.status() !== 'all') {
      filters.push({ field: 'status', operator: 'eq', value: this.status() });
    }
    const view = await this.runOperation(() =>
      this.api.createSavedView(this.requiredWorkspace(), {
        entityType: 'company',
        name: name.trim(),
        definition: {
          filters,
          sort: [{ field: 'updatedAt', direction: 'desc' }],
          columns: ['name', 'domain', 'industry', 'status'],
        },
        isShared: false,
      }),
    );
    this.savedViews.update((views) => [...views, view]);
    return view;
  }

  async applySavedView(view: SavedView): Promise<void> {
    const query = view.definition.filters.find((filter) => filter.field === 'name')?.value;
    const status = view.definition.filters.find((filter) => filter.field === 'status')?.value;
    this.query.set(typeof query === 'string' ? query : '');
    this.status.set(status === 'active' || status === 'inactive' ? status : 'all');
    this.mode.set('companies');
    await this.load(true);
  }

  async deleteSavedView(view: SavedView): Promise<void> {
    await this.runOperation(() => this.api.deleteSavedView(this.requiredWorkspace(), view));
    this.savedViews.update((views) => views.filter((item) => item.id !== view.id));
  }

  private async loadSavedViews(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.referencesLoading()) return;
    this.referencesLoading.set(true);
    try {
      this.savedViews.set(await this.api.listSavedViews(workspaceId, 'company'));
    } catch (error) {
      this.operationError.set(error);
    } finally {
      this.referencesLoading.set(false);
    }
  }

  private async loadTrash(reset: boolean): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.loading()) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listCompanyTrash(
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

  private requiredWorkspace(): string {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace-required');
    return workspaceId;
  }

  private matchesCurrentStatus(company: Company): boolean {
    return this.status() === 'all' || company.status === this.status();
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
