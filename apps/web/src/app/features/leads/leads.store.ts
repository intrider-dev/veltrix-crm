import { computed, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Lead, LeadInput, LeadStage, LeadStatus } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';

@Injectable()
export class LeadsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly toasts = inject(ToastService);
  private requestSequence = 0;

  readonly leads = signal<readonly Lead[]>([]);
  readonly stages = signal<readonly LeadStage[]>([]);
  readonly query = signal('');
  readonly status = signal<LeadStatus | ''>('');
  readonly stageId = signal('');
  readonly nextCursor = signal<string | null>(null);
  readonly nextCursorByStage = signal<Readonly<Record<string, string | null>>>({});
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly movingLeadIds = signal<ReadonlySet<string>>(new Set());
  readonly loadError = signal<unknown>(null);
  readonly stageError = signal<unknown>(null);
  readonly formError = signal<unknown>(null);
  readonly viewMode = signal<LeadViewMode>(readInitialViewMode());
  readonly scheduledLeads = computed(() =>
    this.leads().filter((lead) => lead.plannedStartDate || lead.expectedCloseDate),
  );

  async loadStages(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.stageError.set(null);
    try {
      this.stages.set(await this.api.listLeadStages(workspaceId));
    } catch (error) {
      this.stageError.set(error);
    }
  }

  async load(reset = true): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || (!reset && !this.nextCursor())) return;
    const request = ++this.requestSequence;
    this.loading.set(true);
    this.loadError.set(null);
    try {
      if (this.viewMode() === 'kanban') {
        const stages = this.boardStages();
        const pages = await Promise.all(
          stages.map((stage) =>
            this.api.listLeads(workspaceId, {
              query: this.query() || undefined,
              status: this.status() || undefined,
              stageId: stage.id,
              limit: 25,
            }),
          ),
        );
        if (request === this.requestSequence) {
          this.leads.set(pages.flatMap((page) => page.items));
          this.nextCursor.set(null);
          this.nextCursorByStage.set(
            Object.fromEntries(
              stages.map((stage, index) => [stage.id, pages[index]?.nextCursor ?? null]),
            ),
          );
        }
        return;
      }
      const page = await this.api.listLeads(workspaceId, {
        query: this.query() || undefined,
        status: this.status() || undefined,
        stageId: this.stageId() || undefined,
        cursor: reset ? undefined : (this.nextCursor() ?? undefined),
        limit: 50,
      });
      if (request !== this.requestSequence) return;
      this.leads.update((current) => (reset ? page.items : [...current, ...page.items]));
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      if (request === this.requestSequence) this.loadError.set(error);
    } finally {
      if (request === this.requestSequence) this.loading.set(false);
    }
  }

  async setViewMode(mode: LeadViewMode): Promise<void> {
    if (this.viewMode() === mode) return;
    this.viewMode.set(mode);
    try {
      localStorage.setItem(viewPreferenceKey, mode);
    } catch {
      // Browsers can disable preference storage without disabling the CRM.
    }
    await this.load(true);
  }

  leadsFor(stageId: string): readonly Lead[] {
    return this.leads().filter((lead) => lead.stageId === stageId);
  }

  boardStages(): readonly LeadStage[] {
    const status = this.status();
    return this.stages().filter((stage) => !status || stage.category === status);
  }

  async loadMoreStage(stageId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    const cursor = this.nextCursorByStage()[stageId];
    if (!workspaceId || !cursor || this.loading()) return;
    this.loading.set(true);
    try {
      const page = await this.api.listLeads(workspaceId, {
        query: this.query() || undefined,
        status: this.status() || undefined,
        stageId,
        cursor,
        limit: 25,
      });
      const existing = new Set(this.leads().map((lead) => lead.id));
      this.leads.update((leads) => [
        ...leads,
        ...page.items.filter((lead) => !existing.has(lead.id)),
      ]);
      this.nextCursorByStage.update((cursors) => ({
        ...cursors,
        [stageId]: page.nextCursor ?? null,
      }));
    } catch (error) {
      this.toasts.showError(error);
    } finally {
      this.loading.set(false);
    }
  }

  async create(body: LeadInput): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.saving.set(true);
    this.formError.set(null);
    try {
      const created = await this.api.createLead(workspaceId, body);
      this.leads.update((items) => [created, ...items]);
      this.toasts.show({ messageKey: 'leads.created', messageParams: {}, tone: 'success' });
      return true;
    } catch (error) {
      this.formError.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async changeStage(lead: Lead, stage: LeadStage): Promise<void> {
    const workspaceId = this.workspace.id();
    if (
      !workspaceId ||
      lead.stageId === stage.id ||
      stage.category === 'converted' ||
      this.movingLeadIds().has(lead.id)
    )
      return;
    const previous = lead;
    this.movingLeadIds.update((ids) => new Set(ids).add(lead.id));
    this.replace({ ...lead, stageId: stage.id, status: stage.category });
    try {
      this.replace(await this.api.moveLeadStage(workspaceId, lead, stage.id));
      this.toasts.show({ messageKey: 'leads.stageChanged', messageParams: {}, tone: 'success' });
    } catch (error) {
      const current = this.leads().find((candidate) => candidate.id === lead.id);
      if (current?.version === previous.version) this.replace(previous);
      this.toasts.showError(error);
    } finally {
      this.movingLeadIds.update((ids) => {
        const next = new Set(ids);
        next.delete(lead.id);
        return next;
      });
    }
  }

  isMoving(leadId: string): boolean {
    return this.movingLeadIds().has(leadId);
  }

  setVersion(leadId: string, version: number): void {
    this.leads.update((items) =>
      items.map((item) => (item.id === leadId ? { ...item, version } : item)),
    );
  }

  async convert(lead: Lead): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || lead.status === 'converted') return;
    this.saving.set(true);
    try {
      const result = await this.api.convertLead(workspaceId, lead);
      this.replace(result.lead);
      this.toasts.show({ messageKey: 'leads.converted', messageParams: {}, tone: 'success' });
    } catch (error) {
      this.toasts.showError(error);
    } finally {
      this.saving.set(false);
    }
  }

  private replace(lead: Lead): void {
    this.leads.update((items) => items.map((item) => (item.id === lead.id ? lead : item)));
  }
}

export type LeadViewMode = 'list' | 'kanban' | 'gantt';

const viewPreferenceKey = 'veltrix.leads.view';

function readInitialViewMode(): LeadViewMode {
  try {
    const value = localStorage.getItem(viewPreferenceKey);
    if (value === 'list' || value === 'kanban' || value === 'gantt') return value;
  } catch {
    // List remains usable when browser storage is unavailable.
  }
  return 'list';
}
