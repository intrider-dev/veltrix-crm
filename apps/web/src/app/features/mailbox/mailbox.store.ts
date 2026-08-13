import { computed, inject, Injectable, signal } from '@angular/core';

import { ApiClient } from '../../core/api/api-client.service';
import type {
  MailboxAccount,
  MailboxAccountInput,
  MailboxFolder,
  MailboxMessage,
  MailboxSendInput,
} from '../../core/api/api.types';
import { WorkspaceStore } from '../../core/workspace/workspace.store';
import { ToastService } from '../../shared/feedback/toast.service';

@Injectable()
export class MailboxStore {
  private readonly api = inject(ApiClient);
  private readonly workspace = inject(WorkspaceStore);
  private readonly toasts = inject(ToastService);

  readonly accounts = signal<readonly MailboxAccount[]>([]);
  readonly folders = signal<readonly MailboxFolder[]>([]);
  readonly messages = signal<readonly MailboxMessage[]>([]);
  readonly selectedAccountId = signal<string | null>(null);
  readonly selectedFolderId = signal<string | null>(null);
  readonly selectedMessage = signal<MailboxMessage | null>(null);
  readonly messageBody = signal('');
  readonly nextCursor = signal<string | null>(null);
  readonly loading = signal(false);
  readonly saving = signal(false);
  readonly syncing = signal(false);
  readonly error = signal<unknown>(null);
  readonly selectedAccount = computed(
    () => this.accounts().find((account) => account.id === this.selectedAccountId()) ?? null,
  );

  async load(): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.loading.set(true);
    this.error.set(null);
    try {
      const accounts = await this.api.listMailboxAccounts(workspaceId);
      this.accounts.set(accounts);
      const accountId = this.selectedAccountId() ?? accounts[0]?.id ?? null;
      this.selectedAccountId.set(accountId);
      if (accountId) await this.loadFolders(accountId);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async saveAccount(input: MailboxAccountInput, editing: MailboxAccount | null): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      const saved = editing
        ? await this.api.updateMailboxAccount(workspaceId, editing, input)
        : await this.api.createMailboxAccount(workspaceId, input);
      this.accounts.update((accounts) =>
        editing
          ? accounts.map((account) => (account.id === saved.id ? saved : account))
          : [...accounts, saved],
      );
      this.selectedAccountId.set(saved.id);
      await this.loadFolders(saved.id);
      this.toasts.show({ messageKey: 'mailbox.accountSaved', messageParams: {} });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async deleteAccount(account: MailboxAccount): Promise<boolean> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      await this.api.deleteMailboxAccount(workspaceId, account);
      this.accounts.update((accounts) =>
        accounts.filter((candidate) => candidate.id !== account.id),
      );
      this.selectedAccountId.set(null);
      this.folders.set([]);
      this.messages.set([]);
      this.selectedMessage.set(null);
      this.toasts.show({ messageKey: 'mailbox.accountDeleted', messageParams: {} });
      await this.load();
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }

  async selectAccount(accountId: string): Promise<void> {
    this.selectedAccountId.set(accountId);
    this.selectedMessage.set(null);
    this.messageBody.set('');
    await this.loadFolders(accountId);
  }

  async sync(): Promise<void> {
    const workspaceId = this.workspace.id();
    const accountId = this.selectedAccountId();
    if (!workspaceId || !accountId) return;
    this.syncing.set(true);
    this.error.set(null);
    try {
      await this.api.syncMailboxAccount(workspaceId, accountId);
      await this.load();
      this.toasts.show({ messageKey: 'mailbox.synced', messageParams: {} });
    } catch (error) {
      this.error.set(error);
    } finally {
      this.syncing.set(false);
    }
  }

  async loadFolders(accountId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.error.set(null);
    try {
      const folders = await this.api.listMailboxFolders(workspaceId, accountId);
      this.folders.set(folders);
      const folderId = folders.some((folder) => folder.id === this.selectedFolderId())
        ? this.selectedFolderId()
        : (folders.find((folder) => folder.specialUse === 'inbox')?.id ?? folders[0]?.id ?? null);
      this.selectedFolderId.set(folderId);
      if (folderId) await this.selectFolder(folderId);
      else this.messages.set([]);
    } catch (error) {
      this.error.set(error);
    }
  }

  async selectFolder(folderId: string): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.selectedFolderId.set(folderId);
    this.selectedMessage.set(null);
    this.messageBody.set('');
    this.loading.set(true);
    this.error.set(null);
    try {
      const page = await this.api.listMailboxMessages(workspaceId, folderId);
      this.messages.set(page.items);
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async loadMore(): Promise<void> {
    const workspaceId = this.workspace.id();
    const folderId = this.selectedFolderId();
    const cursor = this.nextCursor();
    if (!workspaceId || !folderId || !cursor || this.loading()) return;
    this.loading.set(true);
    try {
      const page = await this.api.listMailboxMessages(workspaceId, folderId, cursor);
      this.messages.update((messages) => [...messages, ...page.items]);
      this.nextCursor.set(page.nextCursor ?? null);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async openMessage(message: MailboxMessage): Promise<void> {
    const workspaceId = this.workspace.id();
    if (!workspaceId) return;
    this.selectedMessage.set(message);
    this.messageBody.set('');
    this.loading.set(true);
    this.error.set(null);
    try {
      const body = await this.api.readMailboxMessageBody(workspaceId, message.id);
      this.messageBody.set(body.plainText);
    } catch (error) {
      this.error.set(error);
    } finally {
      this.loading.set(false);
    }
  }

  async send(input: MailboxSendInput): Promise<boolean> {
    const workspaceId = this.workspace.id();
    const accountId = this.selectedAccountId();
    if (!workspaceId || !accountId) return false;
    this.saving.set(true);
    this.error.set(null);
    try {
      const result = await this.api.sendMailboxMessage(workspaceId, accountId, input);
      if (result.queued) {
        this.toasts.show({ messageKey: 'mailbox.queued', messageParams: {} });
        return true;
      }
      if (!result.sent) {
        this.error.set(new Error(result.errorCode ?? 'mail_delivery_failed'));
        return false;
      }
      this.toasts.show({ messageKey: 'mailbox.sent', messageParams: {} });
      return true;
    } catch (error) {
      this.error.set(error);
      return false;
    } finally {
      this.saving.set(false);
    }
  }
}
