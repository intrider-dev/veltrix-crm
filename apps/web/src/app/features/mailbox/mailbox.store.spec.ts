import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  MailboxAccount,
  MailboxAccountInput,
  MailboxFolder,
  MailboxMessage,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';
import { MailboxStore } from './mailbox.store';

const account: MailboxAccount = {
  id: 'account-1',
  displayName: 'Work mail',
  email: 'alice@example.test',
  username: 'alice@example.test',
  imapHost: 'imap.example.test',
  imapPort: 993,
  imapSecurity: 'tls',
  smtpHost: 'smtp.example.test',
  smtpPort: 465,
  smtpSecurity: 'tls',
  syncEnabled: true,
  syncState: 'ready',
  version: 1,
};

const folder: MailboxFolder = {
  id: 'folder-1',
  accountId: account.id,
  remoteName: 'INBOX',
  displayName: 'Inbox',
  specialUse: 'inbox',
  highestUid: 12,
  totalCount: 1,
  unreadCount: 1,
};

const message: MailboxMessage = {
  id: 'message-1',
  accountId: account.id,
  folderId: folder.id,
  remoteUid: 12,
  subject: 'Quarterly plan',
  sender: { name: 'Bob', address: 'bob@example.test' },
  recipients: [{ name: 'Alice', address: account.email }],
  receivedAt: '2026-07-22T00:00:00Z',
  flags: [],
  sizeBytes: 1024,
  snippet: 'Hello Alice',
  hasAttachments: false,
  bodyState: 'metadata',
};

describe('MailboxStore', () => {
  it('selects the inbox and loads one bounded message page', async () => {
    const api = {
      listMailboxAccounts: vi.fn().mockResolvedValue([account]),
      listMailboxFolders: vi.fn().mockResolvedValue([folder]),
      listMailboxMessages: vi.fn().mockResolvedValue({ items: [message], nextCursor: 'older' }),
    };
    TestBed.configureTestingModule({
      providers: [
        MailboxStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: { show: vi.fn() } },
      ],
    });
    const store = TestBed.inject(MailboxStore);

    await store.load();

    expect(store.selectedAccountId()).toBe(account.id);
    expect(store.selectedFolderId()).toBe(folder.id);
    expect(store.messages()).toEqual([message]);
    expect(store.nextCursor()).toBe('older');
    expect(api.listMailboxMessages).toHaveBeenCalledWith('workspace-1', folder.id);
  });

  it('keeps credentials write-only when adding an account', async () => {
    const input: MailboxAccountInput = {
      displayName: account.displayName,
      email: account.email,
      username: account.username,
      imapHost: account.imapHost,
      imapPort: account.imapPort,
      imapSecurity: account.imapSecurity,
      smtpHost: account.smtpHost,
      smtpPort: account.smtpPort,
      smtpSecurity: account.smtpSecurity,
      password: 'app-password',
      syncEnabled: true,
    };
    const api = {
      createMailboxAccount: vi.fn().mockResolvedValue(account),
      listMailboxFolders: vi.fn().mockResolvedValue([]),
    };
    const toasts = { show: vi.fn() };
    TestBed.configureTestingModule({
      providers: [
        MailboxStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: toasts },
      ],
    });
    const store = TestBed.inject(MailboxStore);

    await expect(store.saveAccount(input, null)).resolves.toBe(true);

    expect(api.createMailboxAccount).toHaveBeenCalledWith('workspace-1', input);
    expect(store.accounts()).toEqual([account]);
    expect('password' in store.accounts()[0]).toBe(false);
    expect(toasts.show).toHaveBeenCalledWith({
      messageKey: 'mailbox.accountSaved',
      messageParams: {},
    });
  });

  it('treats durable outbound queue acceptance as a successful compose action', async () => {
    const api = {
      sendMailboxMessage: vi.fn().mockResolvedValue({
        outgoingId: 'outgoing-1',
        sent: false,
        queued: true,
      }),
    };
    const toasts = { show: vi.fn() };
    TestBed.configureTestingModule({
      providers: [
        MailboxStore,
        { provide: ApiClient, useValue: api },
        { provide: WorkspaceStore, useValue: { id: signal('workspace-1') } },
        { provide: ToastService, useValue: toasts },
      ],
    });
    const store = TestBed.inject(MailboxStore);
    store.selectedAccountId.set(account.id);

    await expect(
      store.send({
        to: [{ address: 'client@example.test' }],
        cc: [],
        bcc: [],
        subject: 'Follow up',
        plainText: 'Hello',
      }),
    ).resolves.toBe(true);

    expect(toasts.show).toHaveBeenCalledWith({
      messageKey: 'mailbox.queued',
      messageParams: {},
    });
  });
});
