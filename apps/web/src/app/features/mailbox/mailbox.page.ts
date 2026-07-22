import type { OnInit } from '@angular/core';
import { ChangeDetectionStrategy, Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';

import type { MailboxAccount, MailboxAccountInput, MailboxMessage } from '../../core/api/api.types';
import { I18nService } from '../../core/i18n/i18n.service';
import { ErrorPanelComponent } from '../../shared/state/error-panel.component';
import { MailboxStore } from './mailbox.store';

@Component({
  selector: 'app-mailbox-page',
  imports: [
    ErrorPanelComponent,
    MatButtonModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
  ],
  providers: [MailboxStore],
  template: `
    <div class="page mailbox-page">
      <header class="page-header">
        <div>
          <h1>{{ i18n.t('mailbox.title') }}</h1>
          <p>{{ i18n.t('mailbox.subtitle') }}</p>
        </div>
        <div class="header-actions">
          @if (store.accounts().length > 0) {
            <button mat-stroked-button type="button" (click)="composeOpen.set(true)">
              {{ i18n.t('mailbox.compose') }}
            </button>
            <button
              mat-stroked-button
              type="button"
              [disabled]="store.syncing()"
              (click)="store.sync()"
            >
              {{ i18n.t(store.syncing() ? 'mailbox.syncing' : 'mailbox.sync') }}
            </button>
          }
          <button mat-flat-button type="button" (click)="startAccountCreate()">
            {{ i18n.t('mailbox.addAccount') }}
          </button>
        </div>
      </header>

      @if (store.error()) {
        <app-error-panel [error]="store.error()" (retry)="store.load()" />
      }

      @if (accountEditorOpen()) {
        <form
          class="panel account-editor"
          aria-labelledby="mailbox-account-title"
          (submit)="submitAccount($event)"
        >
          <header>
            <h2 id="mailbox-account-title">
              {{ i18n.t(editingAccount() ? 'mailbox.editAccount' : 'mailbox.newAccount') }}
            </h2>
            <button mat-button type="button" (click)="accountEditorOpen.set(false)">
              {{ i18n.t('common.action.cancel') }}
            </button>
          </header>
          <div class="account-fields">
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.displayName') }}</mat-label>
              <input
                matInput
                [value]="accountModel().displayName"
                (input)="setAccountField('displayName', textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('common.field.email') }}</mat-label>
              <input
                matInput
                type="email"
                autocomplete="email"
                [value]="accountModel().email"
                (input)="setAccountField('email', textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.username') }}</mat-label>
              <input
                matInput
                autocomplete="username"
                [value]="accountModel().username"
                (input)="setAccountField('username', textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.password') }}</mat-label>
              <input
                matInput
                type="password"
                autocomplete="new-password"
                [placeholder]="
                  i18n.t(editingAccount() ? 'mailbox.passwordKeep' : 'mailbox.passwordRequired')
                "
                [value]="accountModel().password"
                (input)="setAccountField('password', textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.imapHost') }}</mat-label>
              <input
                matInput
                [value]="accountModel().imapHost"
                (input)="setAccountField('imapHost', textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.imapPort') }}</mat-label>
              <mat-select
                [value]="accountModel().imapPort"
                (selectionChange)="setAccountField('imapPort', $event.value)"
              >
                <mat-option [value]="993">993</mat-option>
                <mat-option [value]="143">143</mat-option>
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.security') }}</mat-label>
              <mat-select
                [value]="accountModel().imapSecurity"
                (selectionChange)="setAccountField('imapSecurity', $event.value)"
              >
                <mat-option value="tls">TLS</mat-option>
                <mat-option value="starttls">STARTTLS</mat-option>
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.smtpHost') }}</mat-label>
              <input
                matInput
                [value]="accountModel().smtpHost"
                (input)="setAccountField('smtpHost', textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.smtpPort') }}</mat-label>
              <mat-select
                [value]="accountModel().smtpPort"
                (selectionChange)="setAccountField('smtpPort', $event.value)"
              >
                <mat-option [value]="465">465</mat-option>
                <mat-option [value]="587">587</mat-option>
              </mat-select>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.security') }}</mat-label>
              <mat-select
                [value]="accountModel().smtpSecurity"
                (selectionChange)="setAccountField('smtpSecurity', $event.value)"
              >
                <mat-option value="tls">TLS</mat-option>
                <mat-option value="starttls">STARTTLS</mat-option>
              </mat-select>
            </mat-form-field>
          </div>
          <label class="sync-toggle">
            <input
              type="checkbox"
              [checked]="accountModel().syncEnabled"
              (change)="setAccountField('syncEnabled', checkedValue($event))"
            />
            <span>{{ i18n.t('mailbox.syncEnabled') }}</span>
          </label>
          <footer>
            @if (editingAccount(); as account) {
              <button
                mat-button
                type="button"
                [disabled]="store.saving()"
                (click)="deleteAccount(account)"
              >
                {{ i18n.t('mailbox.deleteAccount') }}
              </button>
            }
            <span></span>
            <button mat-flat-button type="submit" [disabled]="store.saving() || !accountValid()">
              {{ i18n.t('common.action.save') }}
            </button>
          </footer>
        </form>
      }

      @if (composeOpen()) {
        <form
          class="panel compose"
          aria-labelledby="mailbox-compose-title"
          (submit)="submitMessage($event)"
        >
          <header>
            <h2 id="mailbox-compose-title">{{ i18n.t('mailbox.compose') }}</h2>
            <button mat-button type="button" (click)="composeOpen.set(false)">
              {{ i18n.t('common.action.close') }}
            </button>
          </header>
          <div class="compose-fields">
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.to') }}</mat-label>
              <input matInput [value]="composeTo()" (input)="composeTo.set(textValue($event))" />
              <mat-hint>{{ i18n.t('mailbox.addressHint') }}</mat-hint>
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.cc') }}</mat-label>
              <input matInput [value]="composeCc()" (input)="composeCc.set(textValue($event))" />
            </mat-form-field>
            <mat-form-field appearance="outline">
              <mat-label>{{ i18n.t('mailbox.subject') }}</mat-label>
              <input
                matInput
                maxlength="2000"
                [value]="composeSubject()"
                (input)="composeSubject.set(textValue($event))"
              />
            </mat-form-field>
            <mat-form-field appearance="outline" class="message-field">
              <mat-label>{{ i18n.t('mailbox.message') }}</mat-label>
              <textarea
                matInput
                rows="8"
                [value]="composeBody()"
                (input)="composeBody.set(textValue($event))"
              ></textarea>
            </mat-form-field>
          </div>
          <footer>
            <span></span><span></span>
            <button mat-flat-button type="submit" [disabled]="store.saving() || !composeValid()">
              {{ i18n.t('mailbox.send') }}
            </button>
          </footer>
        </form>
      }

      @if (store.accounts().length > 0) {
        <section class="panel mailbox-toolbar">
          <mat-form-field appearance="outline" subscriptSizing="dynamic">
            <mat-label>{{ i18n.t('mailbox.account') }}</mat-label>
            <mat-select
              [value]="store.selectedAccountId()"
              (selectionChange)="store.selectAccount($event.value)"
            >
              @for (account of store.accounts(); track account.id) {
                <mat-option [value]="account.id"
                  >{{ account.displayName }} · {{ account.email }}</mat-option
                >
              }
            </mat-select>
          </mat-form-field>
          @if (store.selectedAccount(); as account) {
            <span class="sync-state">{{ i18n.t(syncStateKey(account.syncState)) }}</span>
            <button mat-button type="button" (click)="startAccountEdit(account)">
              {{ i18n.t('mailbox.accountSettings') }}
            </button>
          }
        </section>

        <section class="panel mailbox-layout">
          <nav class="folders" [attr.aria-label]="i18n.t('mailbox.folders')">
            @for (folder of store.folders(); track folder.id) {
              <button
                type="button"
                [class.active]="folder.id === store.selectedFolderId()"
                (click)="store.selectFolder(folder.id)"
              >
                <span>{{ folder.displayName }}</span
                ><span class="count">{{ folder.unreadCount }}</span>
              </button>
            } @empty {
              <p class="empty-state">{{ i18n.t('mailbox.syncFirst') }}</p>
            }
          </nav>
          <div class="messages" [attr.aria-label]="i18n.t('mailbox.messages')">
            @for (message of store.messages(); track message.id) {
              <button
                type="button"
                [class.active]="message.id === store.selectedMessage()?.id"
                (click)="store.openMessage(message)"
              >
                <strong>{{ senderLabel(message) }}</strong>
                <span>{{ message.subject || i18n.t('mailbox.noSubject') }}</span>
                <small>{{
                  i18n.date(message.receivedAt, { dateStyle: 'short', timeStyle: 'short' })
                }}</small>
              </button>
            } @empty {
              <p class="empty-state">{{ i18n.t('mailbox.emptyFolder') }}</p>
            }
            @if (store.nextCursor()) {
              <button class="load-more" type="button" (click)="store.loadMore()">
                {{ i18n.t('mailbox.loadMore') }}
              </button>
            }
          </div>
          <article class="reader">
            @if (store.selectedMessage(); as message) {
              <header>
                <h2>{{ message.subject || i18n.t('mailbox.noSubject') }}</h2>
                <p>{{ senderLabel(message) }} · {{ message.sender.address }}</p>
              </header>
              @if (store.loading() && !store.messageBody()) {
                <p role="status">{{ i18n.t('common.app.loading') }}</p>
              } @else {
                <div class="plain-body">{{ store.messageBody() }}</div>
              }
            } @else {
              <div class="empty-state">{{ i18n.t('mailbox.selectMessage') }}</div>
            }
          </article>
        </section>
      } @else if (!store.loading()) {
        <section class="panel empty-state welcome">
          <h2>{{ i18n.t('mailbox.emptyTitle') }}</h2>
          <p>{{ i18n.t('mailbox.emptyDescription') }}</p>
          <button mat-flat-button type="button" (click)="startAccountCreate()">
            {{ i18n.t('mailbox.addAccount') }}
          </button>
        </section>
      }
    </div>
  `,
  styles: `
    .mailbox-page {
      max-width: 92rem;
    }
    .header-actions,
    .mailbox-toolbar,
    footer {
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }
    .account-editor,
    .compose {
      padding: 1rem;
    }
    .account-editor > header,
    .compose > header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 1rem;
    }
    h2 {
      margin: 0;
      font-size: 1rem;
    }
    .account-fields {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 0.75rem;
    }
    .compose-fields {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 0.75rem;
    }
    .message-field {
      grid-column: 1 / -1;
    }
    .sync-toggle {
      display: inline-flex;
      align-items: center;
      gap: 0.5rem;
      min-height: var(--control-height);
    }
    footer {
      margin-top: 0.75rem;
    }
    footer span {
      flex: 1;
    }
    .mailbox-toolbar {
      padding: 0.65rem 0.8rem;
    }
    .mailbox-toolbar mat-form-field {
      min-width: min(24rem, 60vw);
    }
    .sync-state {
      color: var(--text-muted);
      font-size: 0.8rem;
    }
    .mailbox-layout {
      display: grid;
      grid-template-columns: 13rem minmax(17rem, 22rem) minmax(20rem, 1fr);
      min-height: 34rem;
      overflow: hidden;
    }
    .folders,
    .messages {
      border-right: 1px solid var(--border);
      overflow: auto;
    }
    .folders button,
    .messages button {
      width: 100%;
      border: 0;
      border-bottom: 1px solid var(--border);
      background: transparent;
      color: inherit;
      text-align: left;
      cursor: pointer;
    }
    .folders button {
      display: flex;
      justify-content: space-between;
      gap: 0.5rem;
      padding: 0.75rem 0.9rem;
    }
    .messages button {
      display: grid;
      gap: 0.2rem;
      padding: 0.75rem 0.9rem;
    }
    .folders button:hover,
    .messages button:hover {
      background: var(--surface-subtle);
    }
    .folders button.active,
    .messages button.active {
      background: var(--brand-soft);
    }
    .messages span,
    .messages small {
      overflow: hidden;
      color: var(--text-muted);
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .count {
      min-width: 1.4rem;
      text-align: right;
    }
    .reader {
      min-width: 0;
      padding: 1rem 1.25rem;
      overflow: auto;
    }
    .reader header {
      padding-bottom: 0.85rem;
      border-bottom: 1px solid var(--border);
    }
    .reader header p {
      margin: 0.35rem 0 0;
      color: var(--text-muted);
    }
    .plain-body {
      padding-top: 1rem;
      white-space: pre-wrap;
      overflow-wrap: anywhere;
      line-height: 1.55;
    }
    .empty-state {
      padding: 1rem;
    }
    .welcome {
      display: grid;
      justify-items: start;
      gap: 0.75rem;
      padding: 2rem;
    }
    .welcome p {
      margin: 0;
      color: var(--text-muted);
    }
    @media (max-width: 960px) {
      .account-fields {
        grid-template-columns: repeat(2, minmax(0, 1fr));
      }
      .mailbox-layout {
        grid-template-columns: 11rem minmax(16rem, 1fr);
      }
      .reader {
        grid-column: 1 / -1;
        min-height: 20rem;
        border-top: 1px solid var(--border);
      }
    }
    @media (max-width: 640px) {
      .account-fields,
      .compose-fields,
      .mailbox-layout {
        grid-template-columns: 1fr;
      }
      .folders,
      .messages {
        border-right: 0;
        border-bottom: 1px solid var(--border);
        max-height: 18rem;
      }
      .reader {
        grid-column: auto;
      }
    }
  `,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class MailboxPage implements OnInit {
  readonly store = inject(MailboxStore);
  readonly i18n = inject(I18nService);
  readonly accountEditorOpen = signal(false);
  readonly editingAccount = signal<MailboxAccount | null>(null);
  readonly accountModel = signal<MailboxAccountInput>(emptyAccount());
  readonly composeOpen = signal(false);
  readonly composeTo = signal('');
  readonly composeCc = signal('');
  readonly composeSubject = signal('');
  readonly composeBody = signal('');

  ngOnInit(): void {
    void this.store.load();
  }

  startAccountCreate(): void {
    this.editingAccount.set(null);
    this.accountModel.set(emptyAccount());
    this.accountEditorOpen.set(true);
  }

  startAccountEdit(account: MailboxAccount): void {
    this.editingAccount.set(account);
    this.accountModel.set({
      displayName: account.displayName,
      email: account.email,
      username: account.username,
      imapHost: account.imapHost,
      imapPort: account.imapPort,
      imapSecurity: account.imapSecurity,
      smtpHost: account.smtpHost,
      smtpPort: account.smtpPort,
      smtpSecurity: account.smtpSecurity,
      password: '',
      syncEnabled: account.syncEnabled,
    });
    this.accountEditorOpen.set(true);
  }

  setAccountField<K extends keyof MailboxAccountInput>(
    key: K,
    value: MailboxAccountInput[K],
  ): void {
    this.accountModel.update((model) => ({ ...model, [key]: value }));
  }

  accountValid(): boolean {
    const model = this.accountModel();
    return Boolean(
      model.displayName.trim() &&
      model.email.trim() &&
      model.username.trim() &&
      model.imapHost.trim() &&
      model.smtpHost.trim() &&
      (this.editingAccount() || model.password),
    );
  }

  async saveAccount(): Promise<void> {
    if (await this.store.saveAccount(this.accountModel(), this.editingAccount())) {
      this.accountEditorOpen.set(false);
    }
  }

  submitAccount(event: SubmitEvent): void {
    event.preventDefault();
    void this.saveAccount();
  }

  async deleteAccount(account: MailboxAccount): Promise<void> {
    if (!globalThis.confirm(this.i18n.t('mailbox.deleteConfirm', { name: account.displayName })))
      return;
    if (await this.store.deleteAccount(account)) this.accountEditorOpen.set(false);
  }

  composeValid(): boolean {
    return parseAddresses(this.composeTo()).length > 0 && Boolean(this.composeBody().trim());
  }

  async send(): Promise<void> {
    const sent = await this.store.send({
      to: parseAddresses(this.composeTo()),
      cc: parseAddresses(this.composeCc()),
      subject: this.composeSubject().trim(),
      plainText: this.composeBody(),
    });
    if (sent) {
      this.composeOpen.set(false);
      this.composeTo.set('');
      this.composeCc.set('');
      this.composeSubject.set('');
      this.composeBody.set('');
    }
  }

  submitMessage(event: SubmitEvent): void {
    event.preventDefault();
    void this.send();
  }

  senderLabel(message: MailboxMessage): string {
    return message.sender.name || message.sender.address;
  }
  syncStateKey(state: MailboxAccount['syncState']): `mailbox.state.${MailboxAccount['syncState']}` {
    return `mailbox.state.${state}`;
  }
  textValue(event: Event): string {
    return (event.target as HTMLInputElement | HTMLTextAreaElement).value;
  }
  checkedValue(event: Event): boolean {
    return (event.target as HTMLInputElement).checked;
  }
}

function emptyAccount(): MailboxAccountInput {
  return {
    displayName: '',
    email: '',
    username: '',
    imapHost: '',
    imapPort: 993,
    imapSecurity: 'tls',
    smtpHost: '',
    smtpPort: 465,
    smtpSecurity: 'tls',
    password: '',
    syncEnabled: true,
  };
}

function parseAddresses(value: string): Array<{ address: string }> {
  return value
    .split(',')
    .map((address) => address.trim())
    .filter(Boolean)
    .map((address) => ({ address }));
}
