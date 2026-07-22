import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  Activity,
  ContactDetails,
  CreateActivity,
  DuplicateCandidate,
  UpdateContact,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class ContactDetailsStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly contact = signal<ContactDetails | null>(null);
  readonly activities = signal<readonly Activity[]>([]);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly error = signal<unknown>(null);
  readonly conflict = signal(false);
  readonly duplicates = signal<readonly DuplicateCandidate[]>([]);
  readonly duplicatesLoaded = signal(false);
  readonly duplicatesLoading = signal(false);
  readonly duplicatesError = signal<unknown>(null);
  readonly merging = signal<string | null>(null);
  readonly deleting = signal(false);

  async load(contactId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const [contact, activities] = await Promise.all([
        this.api.getContact(workspaceId, contactId),
        this.api.listActivities(workspaceId, 'contact', contactId),
      ]);
      this.contact.set(contact.body);
      this.activities.set(activities);
      this.conflict.set(false);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async save(contactId: string, body: UpdateContact): Promise<ContactDetails> {
    const workspaceId = this.workspace.id();
    const current = this.contact();
    if (!workspaceId || !current) throw new Error('contact.notFound');
    this.saving.set(true);
    this.conflict.set(false);
    try {
      const contact = await this.api.updateContact(workspaceId, contactId, current.version, body);
      this.contact.set(contact);
      return contact;
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'status' in error && error.status === 412)
        this.conflict.set(true);
      throw error;
    } finally {
      this.saving.set(false);
    }
  }

  async addActivity(contactId: string, body: CreateActivity): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    const activity = await this.api.createActivity(workspaceId, body);
    this.activities.update((activities) => [activity, ...activities]);
  }

  async loadDuplicates(contactId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId || this.duplicatesLoading()) return;
    this.duplicatesLoading.set(true);
    this.duplicatesError.set(null);
    try {
      this.duplicates.set(await this.api.contactDuplicates(workspaceId, contactId));
      this.duplicatesLoaded.set(true);
    } catch (error) {
      this.duplicatesError.set(error);
    } finally {
      this.duplicatesLoading.set(false);
    }
  }

  async mergeDuplicate(contactId: string, candidateId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    const current = this.contact();
    if (!workspaceId || !current) throw new Error('contact.notFound');
    this.merging.set(candidateId);
    this.duplicatesError.set(null);
    try {
      const source = await this.api.getContact(workspaceId, candidateId);
      await this.api.mergeContacts(
        workspaceId,
        contactId,
        candidateId,
        source.body.version,
        current.version,
      );
      await this.load(contactId);
      this.duplicates.update((candidates) =>
        candidates.filter((candidate) => candidate.id !== candidateId),
      );
    } catch (error) {
      this.duplicatesError.set(error);
      throw error;
    } finally {
      this.merging.set(null);
    }
  }

  async deleteContact(contactId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    const current = this.contact();
    if (!workspaceId || !current) throw new Error('contact.notFound');
    this.deleting.set(true);
    try {
      await this.api.deleteContact(workspaceId, contactId, current.version);
    } finally {
      this.deleting.set(false);
    }
  }
}
