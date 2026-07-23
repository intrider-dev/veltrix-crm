import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Lead, LeadConversion, LeadInput, LeadStage } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';

@Injectable()
export class LeadDetailsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly toasts = inject(ToastService);
  readonly lead = signal<Lead | null>(null);
  readonly stages = signal<readonly LeadStage[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly loadError = signal<unknown>(null);
  readonly mutationError = signal<unknown>(null);
  private loadSequence = 0;

  async load(leadId: string): Promise<Lead | null> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return null;
    const sequence = ++this.loadSequence;
    this.saving.set(false);
    this.loading.set(true);
    this.loadError.set(null);
    try {
      const [lead, stages] = await Promise.all([
        this.api.getLead(workspaceId, leadId),
        this.api.listLeadStages(workspaceId),
      ]);
      if (sequence !== this.loadSequence || this.workspace.id() !== workspaceId) return null;
      this.lead.set(lead.body);
      this.stages.set(stages);
      return lead.body;
    } catch (error) {
      if (sequence === this.loadSequence && this.workspace.id() === workspaceId) {
        this.loadError.set(error);
      }
      return null;
    } finally {
      if (sequence === this.loadSequence && this.workspace.id() === workspaceId) {
        this.loading.set(false);
      }
    }
  }

  async save(body: LeadInput): Promise<boolean> {
    const workspaceId = this.workspace.id();
    const lead = this.lead();
    if (!workspaceId || !lead) return false;
    const sequence = this.loadSequence;
    const updated = await this.mutate(workspaceId, lead.id, sequence, () =>
      this.api.updateLead(workspaceId, lead, body),
    );
    const saved = updated !== null;
    if (updated) this.lead.set(updated);
    if (saved) this.toasts.show({ messageKey: 'leads.saved', messageParams: {}, tone: 'success' });
    return saved;
  }

  async move(stage: LeadStage): Promise<boolean> {
    const workspaceId = this.workspace.id();
    const lead = this.lead();
    if (!workspaceId || !lead) return false;
    const sequence = this.loadSequence;
    const updated = await this.mutate(workspaceId, lead.id, sequence, () =>
      this.api.moveLeadStage(workspaceId, lead, stage.id),
    );
    const moved = updated !== null;
    if (updated) this.lead.set(updated);
    if (moved)
      this.toasts.show({ messageKey: 'leads.stageChanged', messageParams: {}, tone: 'success' });
    return moved;
  }

  async win(): Promise<LeadConversion | null> {
    const workspaceId = this.workspace.id();
    const lead = this.lead();
    if (!workspaceId || !lead) return null;
    const sequence = this.loadSequence;
    const result = await this.mutate(workspaceId, lead.id, sequence, () =>
      this.api.convertLead(workspaceId, lead),
    );
    const converted = result !== null;
    if (result) this.lead.set(result.lead);
    if (converted)
      this.toasts.show({ messageKey: 'leads.converted', messageParams: {}, tone: 'success' });
    return result;
  }

  setVersion(version: number): void {
    this.lead.update((lead) => (lead ? { ...lead, version } : lead));
  }
  stageName(stageId: string): string {
    return this.stages().find((stage) => stage.id === stageId)?.displayName ?? stageId;
  }

  private async mutate<T>(
    workspaceId: string,
    leadId: string,
    sequence: number,
    action: () => Promise<T>,
  ): Promise<T | null> {
    this.saving.set(true);
    this.mutationError.set(null);
    try {
      const result = await action();
      return this.isCurrent(workspaceId, leadId, sequence) ? result : null;
    } catch (error) {
      if (this.isCurrent(workspaceId, leadId, sequence)) this.mutationError.set(error);
      return null;
    } finally {
      if (this.isCurrent(workspaceId, leadId, sequence)) this.saving.set(false);
    }
  }

  private isCurrent(workspaceId: string, leadId: string, sequence: number): boolean {
    return (
      this.workspace.id() === workspaceId &&
      this.loadSequence === sequence &&
      this.lead()?.id === leadId
    );
  }
}
