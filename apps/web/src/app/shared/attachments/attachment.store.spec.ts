import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type { Attachment } from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { AttachmentStore } from './attachment.store';

describe('AttachmentStore', () => {
  const attachment: Attachment = {
    id: '01900000-0000-7000-8000-000000000001',
    entityType: 'contact',
    entityId: '01900000-0000-7000-8000-000000000002',
    displayName: 'brief.pdf',
    mediaType: 'application/pdf',
    sizeBytes: 128,
    scanState: 'clean',
    createdAt: '2026-07-22T00:00:00Z',
  };
  let api: {
    listAttachments: ReturnType<typeof vi.fn>;
    uploadAttachment: ReturnType<typeof vi.fn>;
    downloadAttachment: ReturnType<typeof vi.fn>;
    deleteAttachment: ReturnType<typeof vi.fn>;
  };
  let store: AttachmentStore;

  beforeEach(() => {
    api = {
      listAttachments: vi.fn().mockResolvedValue([attachment]),
      uploadAttachment: vi.fn().mockResolvedValue(attachment),
      downloadAttachment: vi.fn().mockResolvedValue(new Blob(['test'])),
      deleteAttachment: vi.fn().mockResolvedValue(undefined),
    };
    TestBed.configureTestingModule({
      providers: [
        AttachmentStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: () => 'workspace-1' } },
      ],
    });
    store = TestBed.inject(AttachmentStore);
  });

  it('loads attachments through the active workspace guard', async () => {
    await store.load('contact', attachment.entityId);
    expect(api.listAttachments).toHaveBeenCalledWith('workspace-1', 'contact', attachment.entityId);
    expect(store.items()).toEqual([attachment]);
  });

  it('adds a streamed upload to the bounded entity list', async () => {
    const file = new File(['pdf'], 'brief.pdf', { type: 'application/pdf' });
    await store.upload('contact', attachment.entityId, file);
    expect(api.uploadAttachment).toHaveBeenCalledWith(
      'workspace-1',
      'contact',
      attachment.entityId,
      file,
    );
    expect(store.items()).toEqual([attachment]);
  });

  it('removes metadata only after the API confirms deletion', async () => {
    store.items.set([attachment]);
    await expect(store.remove(attachment.id)).resolves.toBe(true);
    expect(store.items()).toEqual([]);
  });
});
