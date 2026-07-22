import { inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type { Attachment, AttachmentEntityType } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';

@Injectable()
export class AttachmentStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);

  readonly items = signal<readonly Attachment[]>([]);
  readonly loading = signal(false);
  readonly uploading = signal(false);
  readonly deleting = signal<string | null>(null);
  readonly error = signal<unknown>(null);

  async load(entityType: AttachmentEntityType, entityId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      this.items.set(await this.api.listAttachments(workspaceId, entityType, entityId));
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async upload(
    entityType: AttachmentEntityType,
    entityId: string,
    file: File,
  ): Promise<Attachment | null> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return null;
    this.uploading.set(true);
    this.error.set(null);
    try {
      const attachment = await this.api.uploadAttachment(workspaceId, entityType, entityId, file);
      this.items.update((items) => [attachment, ...items]);
      return attachment;
    } catch (error) {
      this.error.set(error);
      return null;
    } finally {
      this.uploading.set(false);
    }
  }

  async download(attachmentId: string): Promise<Blob | null> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return null;
    this.error.set(null);
    try {
      return await this.api.downloadAttachment(workspaceId, attachmentId);
    } catch (error) {
      this.error.set(error);
      return null;
    }
  }

  async remove(attachmentId: string): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.deleting.set(attachmentId);
    this.error.set(null);
    try {
      await this.api.deleteAttachment(workspaceId, attachmentId);
      this.items.update((items) => items.filter((item) => item.id !== attachmentId));
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.deleting.set(null);
    }
  }
}
