import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  Activity,
  Company,
  CompanyRecord,
  CreateActivity,
  DuplicateCandidate,
  UpdateCompany,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class CompanyDetailsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly company = signal<Company | CompanyRecord | null>(null);
  readonly activities = signal<readonly Activity[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly deleting = signal(false);
  readonly error = signal<unknown>(null);
  readonly conflict = signal(false);
  readonly activityError = signal<unknown>(null);
  readonly duplicates = signal<readonly DuplicateCandidate[]>([]);
  readonly duplicatesLoaded = signal(false);
  readonly duplicatesLoading = signal(false);
  readonly duplicatesError = signal<unknown>(null);
  readonly merging = signal<string | null>(null);

  async load(companyId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [company, activities] = await Promise.all([
        this.api.getCompany(workspaceId, companyId),
        this.api.listActivities(workspaceId, 'company', companyId),
      ]);
      this.company.set(company.body);
      this.activities.set(activities);
      this.conflict.set(false);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async save(companyId: string, body: UpdateCompany): Promise<CompanyRecord> {
    const workspaceId = this.requiredWorkspace();
    const current = this.requiredCompany();
    this.saving.set(true);
    this.conflict.set(false);
    try {
      const company = await this.api.updateCompany(workspaceId, companyId, current.version, body);
      this.company.set(company);
      return company;
    } catch (error) {
      if (isVersionConflict(error)) this.conflict.set(true);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async addActivity(body: CreateActivity): Promise<void> {
    const workspaceId = this.requiredWorkspace();
    this.activityError.set(null);
    try {
      const activity = await this.api.createActivity(workspaceId, body);
      this.activities.update((activities) => [activity, ...activities]);
    } catch (error) {
      this.activityError.set(error);
      throw error;
    }
  }

  async deleteCompany(companyId: string): Promise<void> {
    const workspaceId = this.requiredWorkspace();
    const current = this.requiredCompany();
    this.deleting.set(true);
    try {
      await this.api.deleteCompany(workspaceId, companyId, current.version);
    } catch (error) {
      if (isVersionConflict(error)) this.conflict.set(true);
      throw error;
    } finally {
      this.deleting.set(false);
    }
  }

  async loadDuplicates(companyId: string): Promise<void> {
    const workspaceId = this.requiredWorkspace();
    if (this.duplicatesLoading()) return;
    this.duplicatesLoading.set(true);
    this.duplicatesError.set(null);
    try {
      this.duplicates.set(await this.api.companyDuplicates(workspaceId, companyId));
      this.duplicatesLoaded.set(true);
    } catch (error) {
      this.duplicatesError.set(error);
    } finally {
      this.duplicatesLoading.set(false);
    }
  }

  async mergeDuplicate(companyId: string, candidateId: string): Promise<void> {
    const workspaceId = this.requiredWorkspace();
    const current = this.requiredCompany();
    this.merging.set(candidateId);
    this.duplicatesError.set(null);
    try {
      const source = await this.api.getCompany(workspaceId, candidateId);
      await this.api.mergeCompanies(
        workspaceId,
        companyId,
        candidateId,
        source.body.version,
        current.version,
      );
      await this.load(companyId);
      this.duplicates.update((items) => items.filter((item) => item.id !== candidateId));
    } catch (error) {
      if (isVersionConflict(error)) this.conflict.set(true);
      this.duplicatesError.set(error);
      throw error;
    } finally {
      this.merging.set(null);
    }
  }

  private requiredWorkspace(): string {
    const workspaceId = this.workspace.id();
    if (!workspaceId) throw new Error('workspace-required');
    return workspaceId;
  }

  private requiredCompany(): Company | CompanyRecord {
    const company = this.company();
    if (!company) throw new Error('company.notFound');
    return company;
  }
}

function isVersionConflict(error: unknown): boolean {
  return typeof error === 'object' && error !== null && 'status' in error && error.status === 412;
}
