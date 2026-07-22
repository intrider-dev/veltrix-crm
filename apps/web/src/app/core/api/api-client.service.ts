import type { HttpErrorResponse, HttpResponse } from '@angular/common/http';
import { HttpClient, HttpParams } from '@angular/common/http';
import { inject, Injectable } from '@angular/core';
import { catchError, firstValueFrom, throwError, type Observable } from 'rxjs';

import { ApiError } from './api-error';
import type {
  AcceptInvitationRequest,
  Activity,
  Attachment,
  AttachmentEntityType,
  AssignmentSubjectOption,
  ApiKeyItem,
  ApiKeyScope,
  AuditEvent,
  AutomationRule,
  AutomationRuleInput,
  CalendarActivity,
  CalendarActivityInput,
  ChatConversation,
  ChatAttachment,
  ChatMessage,
  ChatMessagePage,
  Call,
  CallConfig,
  CallJoin,
  Company,
  CompanyPage,
  CompanyRecord,
  ContactImportMapping,
  ContactImportPreview,
  ContactImportStatus,
  ContactDetails,
  ContactPage,
  ContentTranslation,
  ContentTranslationPage,
  CreateActivity,
  CreateChatConversation,
  CreateChatMessage,
  CreateCompany,
  CreateContact,
  CreateDeal,
  CreateWorkspaceRequest,
  CustomFieldDefinition,
  CustomFieldDefinitionInput,
  Dashboard,
  DashboardPreferences,
  Deal,
  DealLineItem,
  DealLineItemInput,
  DealPage,
  DealParticipant,
  DealParticipantInput,
  DealRecord,
  DealUpdateInput,
  DealOutcomeInput,
  DeletedRecordPage,
  Department,
  DevelopmentRegistrationRequest,
  DuplicateCandidate,
  GeneratedApiKey,
  GeneratedWebhook,
  Lead,
  LeadConversion,
  LeadInput,
  LeadPage,
  LeadStage,
  LeadStageInput,
  LeadStageOrderRequest,
  MailboxAccount,
  MailboxAccountInput,
  MailboxAccountUpdate,
  MailboxFolder,
  MailboxMessageBody,
  MailboxMessagePage,
  MailboxSendInput,
  MailboxSendResult,
  LineItemMutation,
  MergeResult,
  MembershipMutation,
  MFAChallenge,
  MFACodeRequest,
  MFALoginRequest,
  MFAProtectedRequest,
  MFASetup,
  MFASetupRequest,
  MFAStatus,
  NotificationItem,
  NotificationPage,
  PasswordChangeRequest,
  PasswordResetConfirmation,
  PasswordResetRequest,
  PeriodReport,
  PipelineInput,
  PipelineRecord,
  PipelineStageInput,
  PipelineStageOrderRequest,
  PipelineStageRecord,
  StageAccessRequest,
  StageAccessRule,
  ParticipantMutation,
  PutContentTranslation,
  Project,
  ProjectAssignmentInput,
  ProjectAssignmentSet,
  ProjectInput,
  ProjectPage,
  RecordAssignmentInput,
  RecordAssignmentSet,
  RecoveryCodes,
  RegisteredUser,
  SavedView,
  SavedViewInput,
  SearchResult,
  SessionProbe,
  SessionView,
  Team,
  TaggedResource,
  TranslationCoverage,
  TranslationStatus,
  UpdateContact,
  UpdateCompany,
  UpdateWorkspaceLocaleSettings,
  User,
  VersionedResource,
  VersionedResponse,
  WebhookDeliveryPage,
  WebhookSubscription,
  WorkspaceInvitation,
  WorkspaceLocaleSettings,
  WorkspaceDetails,
  WorkspaceMember,
  WorkspaceRole,
  WorkspaceRoleDefinition,
  WorkspaceRoleInput,
  BulkResult,
  StageHistoryPage,
  Tag,
} from './api.types';

@Injectable({ providedIn: 'root' })
export class ApiClient {
  private readonly baseUrl = '/api/v1';
  private readonly http = inject(HttpClient);

  login(email: string, password: string): Promise<SessionView | MFAChallenge> {
    return this.request(
      this.http.post<SessionView | MFAChallenge>(`${this.baseUrl}/auth/login`, { email, password }),
    );
  }

  registerDevelopmentUser(body: DevelopmentRegistrationRequest): Promise<RegisteredUser> {
    return this.request(this.http.post<RegisteredUser>(`${this.baseUrl}/auth/register`, body));
  }

  verifyMFALogin(body: MFALoginRequest): Promise<SessionView> {
    return this.request(this.http.post<SessionView>(`${this.baseUrl}/auth/mfa/verify`, body));
  }

  requestPasswordReset(body: PasswordResetRequest): Promise<void> {
    return this.request(this.http.post<void>(`${this.baseUrl}/auth/password-reset/request`, body));
  }

  confirmPasswordReset(body: PasswordResetConfirmation): Promise<void> {
    return this.request(this.http.post<void>(`${this.baseUrl}/auth/password-reset/confirm`, body));
  }

  logout(): Promise<void> {
    return this.request(this.http.post<void>(`${this.baseUrl}/auth/logout`, null));
  }

  logoutAllSessions(): Promise<void> {
    return this.request(this.http.delete<void>(`${this.baseUrl}/me/sessions`));
  }

  probeSession(): Promise<SessionProbe> {
    return this.request(this.http.get<SessionProbe>(`${this.baseUrl}/auth/session`));
  }

  me(): Promise<SessionView> {
    return this.request(this.http.get<SessionView>(`${this.baseUrl}/me`));
  }

  updateLocale(preferredLocale: 'en' | 'ru'): Promise<User> {
    return this.request(this.http.patch<User>(`${this.baseUrl}/me`, { preferredLocale }));
  }

  changePassword(body: PasswordChangeRequest): Promise<void> {
    return this.request(this.http.put<void>(`${this.baseUrl}/me/password`, body));
  }

  mfaStatus(): Promise<MFAStatus> {
    return this.request(this.http.get<MFAStatus>(`${this.baseUrl}/me/mfa`));
  }

  beginMFASetup(body: MFASetupRequest): Promise<MFASetup> {
    return this.request(this.http.post<MFASetup>(`${this.baseUrl}/me/mfa`, body));
  }

  confirmMFASetup(body: MFACodeRequest): Promise<RecoveryCodes> {
    return this.request(this.http.post<RecoveryCodes>(`${this.baseUrl}/me/mfa/confirm`, body));
  }

  regenerateRecoveryCodes(body: MFAProtectedRequest): Promise<RecoveryCodes> {
    return this.request(
      this.http.post<RecoveryCodes>(`${this.baseUrl}/me/mfa/recovery-codes`, body),
    );
  }

  disableMFA(body: MFAProtectedRequest): Promise<void> {
    return this.request(this.http.delete<void>(`${this.baseUrl}/me/mfa`, { body }));
  }

  createWorkspace(body: CreateWorkspaceRequest): Promise<WorkspaceDetails> {
    return this.request(this.http.post<WorkspaceDetails>(`${this.baseUrl}/workspaces`, body));
  }

  acceptInvitation(body: AcceptInvitationRequest): Promise<MembershipMutation> {
    return this.request(
      this.http.post<MembershipMutation>(`${this.baseUrl}/invitations/accept`, body),
    );
  }

  dashboard(workspaceId: string): Promise<Dashboard> {
    return this.request(this.http.get<Dashboard>(this.workspaceUrl(workspaceId, 'dashboard')));
  }

  listContacts(
    workspaceId: string,
    options: { cursor?: string; query?: string; status?: string } = {},
  ): Promise<ContactPage> {
    let params = new HttpParams();
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.query) params = params.set('q', options.query);
    if (options.status) params = params.set('status', options.status);
    return this.request(
      this.http.get<ContactPage>(this.workspaceUrl(workspaceId, 'contacts'), { params }),
    );
  }

  getContact(workspaceId: string, contactId: string): Promise<VersionedResponse<ContactDetails>> {
    return this.response(
      this.http.get<ContactDetails>(this.workspaceUrl(workspaceId, `contacts/${contactId}`), {
        observe: 'response',
      }),
    );
  }

  createContact(workspaceId: string, body: CreateContact): Promise<ContactDetails> {
    return this.request(
      this.http.post<ContactDetails>(this.workspaceUrl(workspaceId, 'contacts'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  updateContact(
    workspaceId: string,
    contactId: string,
    version: number,
    body: UpdateContact,
  ): Promise<ContactDetails> {
    return this.request(
      this.http.patch<ContactDetails>(
        this.workspaceUrl(workspaceId, `contacts/${contactId}`),
        body,
        { headers: { 'If-Match': `"${version}"` } },
      ),
    );
  }

  deleteContact(workspaceId: string, contactId: string, version: number): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `contacts/${contactId}`), {
        headers: { 'If-Match': `"${version}"` },
      }),
    );
  }

  listContactTrash(workspaceId: string, cursor?: string): Promise<DeletedRecordPage> {
    let params = new HttpParams().set('limit', 50);
    if (cursor) params = params.set('cursor', cursor);
    return this.request(
      this.http.get<DeletedRecordPage>(this.workspaceUrl(workspaceId, 'contacts/trash'), {
        params,
      }),
    );
  }

  restoreContact(
    workspaceId: string,
    contactId: string,
    version: number,
  ): Promise<VersionedResponse<VersionedResource>> {
    return this.response(
      this.http.post<VersionedResource>(
        this.workspaceUrl(workspaceId, `contacts/${contactId}/restore`),
        null,
        { headers: { 'If-Match': `"${version}"` }, observe: 'response' },
      ),
    );
  }

  replaceContactTags(
    workspaceId: string,
    contactId: string,
    version: number,
    tagIds: readonly string[],
  ): Promise<VersionedResponse<TaggedResource>> {
    return this.response(
      this.http.put<TaggedResource>(
        this.workspaceUrl(workspaceId, `contacts/${contactId}/tags`),
        { tagIds },
        { headers: { 'If-Match': `"${version}"` }, observe: 'response' },
      ),
    );
  }

  contactDuplicates(workspaceId: string, contactId: string): Promise<DuplicateCandidate[]> {
    return this.request(
      this.http.get<DuplicateCandidate[]>(
        this.workspaceUrl(workspaceId, `contacts/${contactId}/duplicates`),
      ),
    );
  }

  mergeContacts(
    workspaceId: string,
    targetId: string,
    sourceId: string,
    sourceVersion: number,
    targetVersion: number,
  ): Promise<VersionedResponse<MergeResult>> {
    return this.response(
      this.http.post<MergeResult>(
        this.workspaceUrl(workspaceId, `contacts/${targetId}/merge`),
        { sourceId, sourceVersion, targetVersion },
        { observe: 'response' },
      ),
    );
  }

  bulkAssignContacts(
    workspaceId: string,
    records: ReadonlyArray<{ readonly id: string; readonly version: number }>,
    ownerId: string | null,
  ): Promise<BulkResult> {
    return this.request(
      this.http.post<BulkResult>(this.workspaceUrl(workspaceId, 'contacts/bulk/assign'), {
        records,
        ownerId,
      }),
    );
  }

  bulkTagContacts(
    workspaceId: string,
    records: ReadonlyArray<{ readonly id: string; readonly version: number }>,
    tagIds: readonly string[],
    mode: 'add' | 'remove' | 'replace',
  ): Promise<BulkResult> {
    return this.request(
      this.http.post<BulkResult>(this.workspaceUrl(workspaceId, 'contacts/bulk/tags'), {
        records,
        tagIds,
        mode,
      }),
    );
  }

  bulkDeleteContacts(
    workspaceId: string,
    records: ReadonlyArray<{ readonly id: string; readonly version: number }>,
  ): Promise<BulkResult> {
    return this.request(
      this.http.post<BulkResult>(this.workspaceUrl(workspaceId, 'contacts/bulk/delete'), {
        records,
      }),
    );
  }

  exportContacts(
    workspaceId: string,
    options: { query?: string; status?: string } = {},
  ): Promise<Blob> {
    let params = new HttpParams();
    if (options.query) params = params.set('q', options.query);
    if (options.status) params = params.set('status', options.status);
    return this.request(
      this.http.get(this.workspaceUrl(workspaceId, 'contacts/export'), {
        params,
        responseType: 'blob',
      }),
    );
  }

  previewContactImport(workspaceId: string, file: File): Promise<ContactImportPreview> {
    const formData = new FormData();
    formData.set('file', file, file.name);
    return this.request(
      this.http.post<ContactImportPreview>(
        this.workspaceUrl(workspaceId, 'contacts/imports/preview'),
        formData,
      ),
    );
  }

  getContactImport(workspaceId: string, importId: string): Promise<ContactImportStatus> {
    return this.request(
      this.http.get<ContactImportStatus>(
        this.workspaceUrl(workspaceId, `contacts/imports/${importId}`),
      ),
    );
  }

  queueContactImport(
    workspaceId: string,
    importId: string,
    mapping: ContactImportMapping,
  ): Promise<ContactImportStatus> {
    return this.request(
      this.http.post<ContactImportStatus>(
        this.workspaceUrl(workspaceId, `contacts/imports/${importId}/queue`),
        mapping,
      ),
    );
  }

  contactImportErrorsUrl(workspaceId: string, importId: string): string {
    return this.workspaceUrl(workspaceId, `contacts/imports/${importId}/errors`);
  }

  listSavedViews(workspaceId: string, entityType: 'contact' | 'company'): Promise<SavedView[]> {
    return this.request(
      this.http.get<SavedView[]>(this.workspaceUrl(workspaceId, 'saved-views'), {
        params: { entityType },
      }),
    );
  }

  listTags(workspaceId: string): Promise<Tag[]> {
    return this.request(this.http.get<Tag[]>(this.workspaceUrl(workspaceId, 'tags')));
  }

  createSavedView(workspaceId: string, body: SavedViewInput): Promise<SavedView> {
    return this.request(
      this.http.post<SavedView>(this.workspaceUrl(workspaceId, 'saved-views'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  deleteSavedView(workspaceId: string, view: SavedView): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `saved-views/${view.id}`), {
        headers: { 'If-Match': `"${view.version}"` },
      }),
    );
  }

  listCompanies(
    workspaceId: string,
    options: { cursor?: string; query?: string; status?: string; limit?: number } = {},
  ): Promise<CompanyPage> {
    let params = new HttpParams().set('limit', options.limit ?? 50);
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.query) params = params.set('q', options.query);
    if (options.status) params = params.set('status', options.status);
    return this.request(
      this.http.get<CompanyPage>(this.workspaceUrl(workspaceId, 'companies'), { params }),
    );
  }

  getCompany(workspaceId: string, companyId: string): Promise<VersionedResponse<Company>> {
    return this.response(
      this.http.get<Company>(this.workspaceUrl(workspaceId, `companies/${companyId}`), {
        observe: 'response',
      }),
    );
  }

  createCompany(workspaceId: string, body: CreateCompany): Promise<Company> {
    return this.request(
      this.http.post<Company>(this.workspaceUrl(workspaceId, 'companies'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  updateCompany(
    workspaceId: string,
    companyId: string,
    version: number,
    body: UpdateCompany,
  ): Promise<CompanyRecord> {
    return this.request(
      this.http.patch<CompanyRecord>(
        this.workspaceUrl(workspaceId, `companies/${companyId}`),
        body,
        { headers: { 'If-Match': `"${version}"` } },
      ),
    );
  }

  deleteCompany(workspaceId: string, companyId: string, version: number): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `companies/${companyId}`), {
        headers: { 'If-Match': `"${version}"` },
      }),
    );
  }

  listCompanyTrash(workspaceId: string, cursor?: string): Promise<DeletedRecordPage> {
    let params = new HttpParams().set('limit', 50);
    if (cursor) params = params.set('cursor', cursor);
    return this.request(
      this.http.get<DeletedRecordPage>(this.workspaceUrl(workspaceId, 'companies/trash'), {
        params,
      }),
    );
  }

  restoreCompany(
    workspaceId: string,
    companyId: string,
    version: number,
  ): Promise<VersionedResponse<CompanyRecord>> {
    return this.response(
      this.http.post<CompanyRecord>(
        this.workspaceUrl(workspaceId, `companies/${companyId}/restore`),
        null,
        { headers: { 'If-Match': `"${version}"` }, observe: 'response' },
      ),
    );
  }

  companyDuplicates(workspaceId: string, companyId: string): Promise<DuplicateCandidate[]> {
    return this.request(
      this.http.get<DuplicateCandidate[]>(
        this.workspaceUrl(workspaceId, `companies/${companyId}/duplicates`),
      ),
    );
  }

  mergeCompanies(
    workspaceId: string,
    targetId: string,
    sourceId: string,
    sourceVersion: number,
    targetVersion: number,
  ): Promise<VersionedResponse<MergeResult>> {
    return this.response(
      this.http.post<MergeResult>(
        this.workspaceUrl(workspaceId, `companies/${targetId}/merge`),
        { sourceId, sourceVersion, targetVersion },
        { observe: 'response' },
      ),
    );
  }

  listPipelines(workspaceId: string): Promise<PipelineRecord[]> {
    return this.request(
      this.http.get<PipelineRecord[]>(this.workspaceUrl(workspaceId, 'pipelines')),
    );
  }

  createPipeline(workspaceId: string, body: PipelineInput): Promise<PipelineRecord> {
    return this.request(
      this.http.post<PipelineRecord>(this.workspaceUrl(workspaceId, 'pipelines'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  updatePipeline(
    workspaceId: string,
    pipeline: PipelineRecord,
    body: PipelineInput,
  ): Promise<PipelineRecord> {
    return this.request(
      this.http.put<PipelineRecord>(
        this.workspaceUrl(workspaceId, `pipelines/${pipeline.id}`),
        body,
        { headers: { 'If-Match': `"${pipeline.version}"` } },
      ),
    );
  }

  deletePipeline(workspaceId: string, pipeline: PipelineRecord): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `pipelines/${pipeline.id}`), {
        headers: { 'If-Match': `"${pipeline.version}"` },
      }),
    );
  }

  createPipelineStage(
    workspaceId: string,
    pipelineId: string,
    body: PipelineStageInput,
  ): Promise<PipelineStageRecord> {
    return this.request(
      this.http.post<PipelineStageRecord>(
        this.workspaceUrl(workspaceId, `pipelines/${pipelineId}/stages`),
        body,
        { headers: { 'Idempotency-Key': crypto.randomUUID() } },
      ),
    );
  }

  updatePipelineStage(
    workspaceId: string,
    stage: PipelineStageRecord,
    body: PipelineStageInput,
  ): Promise<PipelineStageRecord> {
    return this.request(
      this.http.put<PipelineStageRecord>(
        this.workspaceUrl(workspaceId, `pipeline-stages/${stage.id}`),
        body,
        { headers: { 'If-Match': `"${stage.version}"` } },
      ),
    );
  }

  deletePipelineStage(workspaceId: string, stage: PipelineStageRecord): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `pipeline-stages/${stage.id}`), {
        headers: { 'If-Match': `"${stage.version}"` },
      }),
    );
  }

  reorderPipelineStages(
    workspaceId: string,
    pipelineId: string,
    body: PipelineStageOrderRequest,
  ): Promise<PipelineStageRecord[]> {
    return this.request(
      this.http.put<PipelineStageRecord[]>(
        this.workspaceUrl(workspaceId, `pipelines/${pipelineId}/stages/order`),
        body,
      ),
    );
  }

  listPipelineStageAccess(workspaceId: string, stageId: string): Promise<StageAccessRule[]> {
    return this.request(
      this.http.get<StageAccessRule[]>(
        this.workspaceUrl(workspaceId, `pipeline-stages/${stageId}/access`),
      ),
    );
  }

  replacePipelineStageAccess(
    workspaceId: string,
    stageId: string,
    body: StageAccessRequest,
  ): Promise<StageAccessRule[]> {
    return this.request(
      this.http.put<StageAccessRule[]>(
        this.workspaceUrl(workspaceId, `pipeline-stages/${stageId}/access`),
        body,
      ),
    );
  }

  listDeals(
    workspaceId: string,
    pipelineId?: string,
    stageId?: string,
    cursor?: string,
    limit = 25,
  ): Promise<DealPage> {
    let params = new HttpParams();
    if (pipelineId) params = params.set('pipelineId', pipelineId);
    if (stageId) params = params.set('stageId', stageId);
    if (cursor) params = params.set('cursor', cursor);
    params = params.set('limit', limit);
    return this.request(
      this.http.get<DealPage>(this.workspaceUrl(workspaceId, 'deals'), { params }),
    );
  }

  createDeal(workspaceId: string, body: CreateDeal): Promise<Deal> {
    return this.request(
      this.http.post<Deal>(this.workspaceUrl(workspaceId, 'deals'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  moveDeal(
    workspaceId: string,
    dealId: string,
    version: number,
    stageId: string,
    position: number,
  ): Promise<Deal> {
    return this.request(
      this.http.patch<Deal>(
        this.workspaceUrl(workspaceId, `deals/${dealId}/stage`),
        { stageId, position },
        { headers: { 'If-Match': `"${version}"` } },
      ),
    );
  }

  getDeal(workspaceId: string, dealId: string): Promise<VersionedResponse<DealRecord>> {
    return this.response(
      this.http.get<DealRecord>(this.workspaceUrl(workspaceId, `deals/${dealId}`), {
        observe: 'response',
      }),
    );
  }

  updateDeal(
    workspaceId: string,
    dealId: string,
    version: number,
    body: DealUpdateInput,
  ): Promise<VersionedResponse<DealRecord>> {
    return this.response(
      this.http.put<DealRecord>(this.workspaceUrl(workspaceId, `deals/${dealId}`), body, {
        headers: { 'If-Match': `"${version}"` },
        observe: 'response',
      }),
    );
  }

  setDealOutcome(
    workspaceId: string,
    dealId: string,
    version: number,
    body: DealOutcomeInput,
  ): Promise<VersionedResponse<DealRecord>> {
    return this.response(
      this.http.patch<DealRecord>(this.workspaceUrl(workspaceId, `deals/${dealId}/outcome`), body, {
        headers: { 'If-Match': `"${version}"` },
        observe: 'response',
      }),
    );
  }

  listDealHistory(workspaceId: string, dealId: string): Promise<StageHistoryPage> {
    return this.request(
      this.http.get<StageHistoryPage>(this.workspaceUrl(workspaceId, `deals/${dealId}/history`)),
    );
  }

  listDealLineItems(workspaceId: string, dealId: string): Promise<DealLineItem[]> {
    return this.request(
      this.http.get<DealLineItem[]>(this.workspaceUrl(workspaceId, `deals/${dealId}/line-items`)),
    );
  }

  createDealLineItem(
    workspaceId: string,
    dealId: string,
    dealVersion: number,
    body: DealLineItemInput,
  ): Promise<VersionedResponse<LineItemMutation>> {
    return this.response(
      this.http.post<LineItemMutation>(
        this.workspaceUrl(workspaceId, `deals/${dealId}/line-items`),
        body,
        {
          headers: {
            'If-Match': `"${dealVersion}"`,
            'Idempotency-Key': crypto.randomUUID(),
          },
          observe: 'response',
        },
      ),
    );
  }

  listDealParticipants(workspaceId: string, dealId: string): Promise<DealParticipant[]> {
    return this.request(
      this.http.get<DealParticipant[]>(
        this.workspaceUrl(workspaceId, `deals/${dealId}/participants`),
      ),
    );
  }

  upsertDealParticipant(
    workspaceId: string,
    dealId: string,
    dealVersion: number,
    body: DealParticipantInput,
  ): Promise<VersionedResponse<ParticipantMutation>> {
    return this.response(
      this.http.put<ParticipantMutation>(
        this.workspaceUrl(workspaceId, `deals/${dealId}/participants`),
        body,
        {
          headers: {
            'If-Match': `"${dealVersion}"`,
            'Idempotency-Key': crypto.randomUUID(),
          },
          observe: 'response',
        },
      ),
    );
  }

  listActivities(
    workspaceId: string,
    entityType?: 'contact' | 'company' | 'deal' | 'project',
    entityId?: string,
  ): Promise<Activity[]> {
    let params = new HttpParams();
    if (entityType) params = params.set('entityType', entityType);
    if (entityId) params = params.set('entityId', entityId);
    return this.request(
      this.http.get<Activity[]>(this.workspaceUrl(workspaceId, 'activities'), { params }),
    );
  }

  createActivity(workspaceId: string, body: CreateActivity): Promise<Activity> {
    return this.request(
      this.http.post<Activity>(this.workspaceUrl(workspaceId, 'activities'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  completeActivity(workspaceId: string, activity: Activity): Promise<Activity> {
    return this.request(
      this.http.patch<Activity>(
        this.workspaceUrl(workspaceId, `activities/${activity.id}/complete`),
        null,
        { headers: { 'If-Match': `"${activity.version}"` } },
      ),
    );
  }

  listProjects(
    workspaceId: string,
    options: { status?: string; cursor?: string; limit?: number } = {},
  ): Promise<ProjectPage> {
    let params = new HttpParams();
    if (options.status) params = params.set('status', options.status);
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.limit) params = params.set('limit', options.limit);
    return this.request(
      this.http.get<ProjectPage>(this.workspaceUrl(workspaceId, 'projects'), { params }),
    );
  }

  listConversations(workspaceId: string): Promise<ChatConversation[]> {
    return this.request(
      this.http.get<ChatConversation[]>(this.workspaceUrl(workspaceId, 'conversations')),
    );
  }

  createConversation(workspaceId: string, body: CreateChatConversation): Promise<ChatConversation> {
    return this.request(
      this.http.post<ChatConversation>(this.workspaceUrl(workspaceId, 'conversations'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  callConfig(workspaceId: string): Promise<CallConfig> {
    return this.request(this.http.get<CallConfig>(this.workspaceUrl(workspaceId, 'calls/config')));
  }

  createCall(workspaceId: string, conversationId: string, kind: 'audio' | 'video'): Promise<Call> {
    return this.request(
      this.http.post<Call>(
        this.workspaceUrl(workspaceId, `conversations/${encodeURIComponent(conversationId)}/calls`),
        { kind },
        { headers: { 'Idempotency-Key': crypto.randomUUID() } },
      ),
    );
  }

  getCall(workspaceId: string, callId: string): Promise<Call> {
    return this.request(
      this.http.get<Call>(this.workspaceUrl(workspaceId, `calls/${encodeURIComponent(callId)}`)),
    );
  }

  joinCall(workspaceId: string, callId: string): Promise<CallJoin> {
    return this.request(
      this.http.post<CallJoin>(
        this.workspaceUrl(workspaceId, `calls/${encodeURIComponent(callId)}/join-token`),
        null,
      ),
    );
  }

  declineCall(workspaceId: string, callId: string): Promise<void> {
    return this.callAction(workspaceId, callId, 'decline');
  }

  leaveCall(workspaceId: string, callId: string): Promise<void> {
    return this.callAction(workspaceId, callId, 'leave');
  }

  endCall(workspaceId: string, callId: string): Promise<Call> {
    return this.request(
      this.http.post<Call>(
        this.workspaceUrl(workspaceId, `calls/${encodeURIComponent(callId)}/end`),
        null,
      ),
    );
  }

  listChatMessages(
    workspaceId: string,
    conversationId: string,
    cursor?: string,
  ): Promise<ChatMessagePage> {
    let params = new HttpParams().set('limit', 50);
    if (cursor) params = params.set('cursor', cursor);
    return this.request(
      this.http.get<ChatMessagePage>(
        this.workspaceUrl(
          workspaceId,
          `conversations/${encodeURIComponent(conversationId)}/messages`,
        ),
        { params },
      ),
    );
  }

  listChatAttachments(workspaceId: string, conversationId: string): Promise<ChatAttachment[]> {
    return this.request(
      this.http.get<ChatAttachment[]>(
        this.workspaceUrl(
          workspaceId,
          `conversations/${encodeURIComponent(conversationId)}/attachments`,
        ),
      ),
    );
  }

  sendChatMessage(
    workspaceId: string,
    conversationId: string,
    body: CreateChatMessage,
  ): Promise<ChatMessage> {
    return this.request(
      this.http.post<ChatMessage>(
        this.workspaceUrl(
          workspaceId,
          `conversations/${encodeURIComponent(conversationId)}/messages`,
        ),
        body,
        { headers: { 'Idempotency-Key': crypto.randomUUID() } },
      ),
    );
  }

  markConversationRead(workspaceId: string, conversationId: string): Promise<void> {
    return this.request(
      this.http.post<void>(
        this.workspaceUrl(workspaceId, `conversations/${encodeURIComponent(conversationId)}/read`),
        null,
      ),
    );
  }

  addChatReaction(workspaceId: string, messageId: string, emoji: string): Promise<void> {
    return this.request(
      this.http.put<void>(
        this.workspaceUrl(workspaceId, `chat/messages/${encodeURIComponent(messageId)}/reactions`),
        { emoji },
      ),
    );
  }

  removeChatReaction(workspaceId: string, messageId: string, emoji: string): Promise<void> {
    return this.request(
      this.http.delete<void>(
        this.workspaceUrl(workspaceId, `chat/messages/${encodeURIComponent(messageId)}/reactions`),
        { params: { emoji } },
      ),
    );
  }

  setChatMessagePinned(workspaceId: string, messageId: string, pinned: boolean): Promise<void> {
    const url = this.workspaceUrl(
      workspaceId,
      `chat/messages/${encodeURIComponent(messageId)}/pin`,
    );
    return this.request(pinned ? this.http.put<void>(url, null) : this.http.delete<void>(url));
  }

  createProject(workspaceId: string, body: ProjectInput): Promise<Project> {
    return this.request(
      this.http.post<Project>(this.workspaceUrl(workspaceId, 'projects'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  getProject(workspaceId: string, projectId: string): Promise<VersionedResponse<Project>> {
    return this.response(
      this.http.get<Project>(this.workspaceUrl(workspaceId, `projects/${projectId}`), {
        observe: 'response',
      }),
    );
  }

  updateProject(
    workspaceId: string,
    projectId: string,
    version: number,
    body: ProjectInput,
  ): Promise<VersionedResponse<Project>> {
    return this.response(
      this.http.put<Project>(this.workspaceUrl(workspaceId, `projects/${projectId}`), body, {
        headers: { 'If-Match': `"${version}"` },
        observe: 'response',
      }),
    );
  }

  deleteProject(workspaceId: string, projectId: string, version: number): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `projects/${projectId}`), {
        headers: { 'If-Match': `"${version}"` },
      }),
    );
  }

  listProjectAssignments(workspaceId: string, projectId: string): Promise<ProjectAssignmentSet> {
    return this.request(
      this.http.get<ProjectAssignmentSet>(
        this.workspaceUrl(workspaceId, `projects/${projectId}/assignments`),
      ),
    );
  }

  replaceProjectAssignments(
    workspaceId: string,
    projectId: string,
    version: number,
    assignments: readonly ProjectAssignmentInput[],
  ): Promise<ProjectAssignmentSet> {
    return this.request(
      this.http.put<ProjectAssignmentSet>(
        this.workspaceUrl(workspaceId, `projects/${projectId}/assignments`),
        { assignments },
        { headers: { 'If-Match': `"${version}"` } },
      ),
    );
  }

  listLeadAssignments(workspaceId: string, leadId: string): Promise<RecordAssignmentSet> {
    return this.request(
      this.http.get<RecordAssignmentSet>(
        this.workspaceUrl(workspaceId, `leads/${leadId}/assignments`),
      ),
    );
  }

  listAssignmentSubjects(workspaceId: string): Promise<AssignmentSubjectOption[]> {
    return this.request(
      this.http.get<AssignmentSubjectOption[]>(
        this.workspaceUrl(workspaceId, 'assignment-subjects'),
      ),
    );
  }

  replaceLeadAssignments(
    workspaceId: string,
    leadId: string,
    version: number,
    assignments: readonly RecordAssignmentInput[],
  ): Promise<RecordAssignmentSet> {
    return this.replaceRecordAssignments(
      this.workspaceUrl(workspaceId, `leads/${leadId}/assignments`),
      version,
      assignments,
    );
  }

  listDealAssignments(workspaceId: string, dealId: string): Promise<RecordAssignmentSet> {
    return this.request(
      this.http.get<RecordAssignmentSet>(
        this.workspaceUrl(workspaceId, `deals/${dealId}/assignments`),
      ),
    );
  }

  replaceDealAssignments(
    workspaceId: string,
    dealId: string,
    version: number,
    assignments: readonly RecordAssignmentInput[],
  ): Promise<RecordAssignmentSet> {
    return this.replaceRecordAssignments(
      this.workspaceUrl(workspaceId, `deals/${dealId}/assignments`),
      version,
      assignments,
    );
  }

  listTaskAssignments(workspaceId: string, activityId: string): Promise<RecordAssignmentSet> {
    return this.request(
      this.http.get<RecordAssignmentSet>(
        this.workspaceUrl(workspaceId, `activities/${activityId}/assignments`),
      ),
    );
  }

  replaceTaskAssignments(
    workspaceId: string,
    activityId: string,
    version: number,
    assignments: readonly RecordAssignmentInput[],
  ): Promise<RecordAssignmentSet> {
    return this.replaceRecordAssignments(
      this.workspaceUrl(workspaceId, `activities/${activityId}/assignments`),
      version,
      assignments,
    );
  }

  listLeads(
    workspaceId: string,
    options: { query?: string; status?: string; cursor?: string; limit?: number } = {},
  ): Promise<LeadPage> {
    let params = new HttpParams();
    if (options.query) params = params.set('query', options.query);
    if (options.status) params = params.set('status', options.status);
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.limit) params = params.set('limit', options.limit);
    return this.request(
      this.http.get<LeadPage>(this.workspaceUrl(workspaceId, 'leads'), { params }),
    );
  }

  listLeadStages(workspaceId: string): Promise<LeadStage[]> {
    return this.request(this.http.get<LeadStage[]>(this.workspaceUrl(workspaceId, 'lead-stages')));
  }

  createLeadStage(workspaceId: string, body: LeadStageInput): Promise<LeadStage> {
    return this.request(
      this.http.post<LeadStage>(this.workspaceUrl(workspaceId, 'lead-stages'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  updateLeadStage(workspaceId: string, stage: LeadStage, body: LeadStageInput): Promise<LeadStage> {
    return this.request(
      this.http.put<LeadStage>(this.workspaceUrl(workspaceId, `lead-stages/${stage.id}`), body, {
        headers: { 'If-Match': `"${stage.version}"` },
      }),
    );
  }

  deleteLeadStage(workspaceId: string, stage: LeadStage): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `lead-stages/${stage.id}`), {
        headers: { 'If-Match': `"${stage.version}"` },
      }),
    );
  }

  reorderLeadStages(workspaceId: string, body: LeadStageOrderRequest): Promise<LeadStage[]> {
    return this.request(
      this.http.put<LeadStage[]>(this.workspaceUrl(workspaceId, 'lead-stages/order'), body),
    );
  }

  listLeadStageAccess(workspaceId: string, stageId: string): Promise<StageAccessRule[]> {
    return this.request(
      this.http.get<StageAccessRule[]>(
        this.workspaceUrl(workspaceId, `lead-stages/${stageId}/access`),
      ),
    );
  }

  replaceLeadStageAccess(
    workspaceId: string,
    stageId: string,
    body: StageAccessRequest,
  ): Promise<StageAccessRule[]> {
    return this.request(
      this.http.put<StageAccessRule[]>(
        this.workspaceUrl(workspaceId, `lead-stages/${stageId}/access`),
        body,
      ),
    );
  }

  createLead(workspaceId: string, body: LeadInput): Promise<Lead> {
    return this.request(
      this.http.post<Lead>(this.workspaceUrl(workspaceId, 'leads'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  updateLead(workspaceId: string, lead: Lead, body: LeadInput): Promise<Lead> {
    return this.request(
      this.http.put<Lead>(this.workspaceUrl(workspaceId, `leads/${lead.id}`), body, {
        headers: { 'If-Match': `"${lead.version}"` },
      }),
    );
  }

  moveLeadStage(workspaceId: string, lead: Lead, stageId: string): Promise<Lead> {
    return this.request(
      this.http.patch<Lead>(
        this.workspaceUrl(workspaceId, `leads/${lead.id}/stage`),
        { stageId },
        { headers: { 'If-Match': `"${lead.version}"` } },
      ),
    );
  }

  convertLead(workspaceId: string, lead: Lead): Promise<LeadConversion> {
    const name = lead.name.trim().split(/\s+/);
    return this.request(
      this.http.post<LeadConversion>(
        this.workspaceUrl(workspaceId, `leads/${lead.id}/convert`),
        {
          createContact: true,
          contact: {
            firstName: name[0] ?? '',
            lastName: name.slice(1).join(' '),
            email: lead.email,
            phone: lead.phone,
            jobTitle: lead.jobTitle,
          },
          createCompany: Boolean(lead.companyName),
        },
        {
          headers: {
            'If-Match': `"${lead.version}"`,
            'Idempotency-Key': crypto.randomUUID(),
          },
        },
      ),
    );
  }

  listCalendar(workspaceId: string, start: Date, end: Date): Promise<CalendarActivity[]> {
    return this.request(
      this.http.get<CalendarActivity[]>(this.workspaceUrl(workspaceId, 'calendar'), {
        params: { start: start.toISOString(), end: end.toISOString() },
      }),
    );
  }

  createCalendarActivity(
    workspaceId: string,
    body: CalendarActivityInput,
  ): Promise<CalendarActivity> {
    return this.request(
      this.http.post<CalendarActivity>(this.workspaceUrl(workspaceId, 'activities'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  calendarIcsUrl(workspaceId: string, start: Date, end: Date): string {
    const params = new URLSearchParams({ start: start.toISOString(), end: end.toISOString() });
    return `${this.workspaceUrl(workspaceId, 'calendar.ics')}?${params.toString()}`;
  }

  listNotifications(
    workspaceId: string,
    options: { unread?: boolean; cursor?: string; limit?: number } = {},
  ): Promise<NotificationPage> {
    let params = new HttpParams();
    if (options.unread) params = params.set('unread', true);
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.limit) params = params.set('limit', options.limit);
    return this.request(
      this.http.get<NotificationPage>(this.workspaceUrl(workspaceId, 'notifications'), { params }),
    );
  }

  markNotificationRead(workspaceId: string, item: NotificationItem): Promise<NotificationItem> {
    return this.request(
      this.http.put<NotificationItem>(
        this.workspaceUrl(workspaceId, `notifications/${item.id}/read`),
        null,
        { headers: { 'If-Match': `"${item.version}"` } },
      ),
    );
  }

  markAllNotificationsRead(workspaceId: string): Promise<{ readonly updated: number }> {
    return this.request(
      this.http.post<{ readonly updated: number }>(
        this.workspaceUrl(workspaceId, 'notifications/read-all'),
        null,
      ),
    );
  }

  eventsUrl(workspaceId: string): string {
    return this.workspaceUrl(workspaceId, 'events');
  }

  periodReport(
    workspaceId: string,
    start: Date,
    end: Date,
    timezone: string,
  ): Promise<PeriodReport> {
    return this.request(
      this.http.get<PeriodReport>(this.workspaceUrl(workspaceId, 'reports/period'), {
        params: { start: start.toISOString(), end: end.toISOString(), timezone },
      }),
    );
  }

  dashboardPreferences(workspaceId: string): Promise<DashboardPreferences> {
    return this.request(
      this.http.get<DashboardPreferences>(this.workspaceUrl(workspaceId, 'dashboard/preferences')),
    );
  }

  saveDashboardPreferences(
    workspaceId: string,
    body: Pick<DashboardPreferences, 'layout' | 'periodDays' | 'timezone'>,
    version?: number,
  ): Promise<DashboardPreferences> {
    return this.request(
      this.http.put<DashboardPreferences>(
        this.workspaceUrl(workspaceId, 'dashboard/preferences'),
        body,
        {
          headers: version ? { 'If-Match': `"${version}"` } : {},
        },
      ),
    );
  }

  listAutomations(workspaceId: string): Promise<AutomationRule[]> {
    return this.request(
      this.http.get<AutomationRule[]>(this.workspaceUrl(workspaceId, 'automations')),
    );
  }

  createAutomation(workspaceId: string, body: AutomationRuleInput): Promise<AutomationRule> {
    return this.request(
      this.http.post<AutomationRule>(this.workspaceUrl(workspaceId, 'automations'), body),
    );
  }

  setAutomationEnabled(
    workspaceId: string,
    rule: AutomationRule,
    enabled: boolean,
  ): Promise<AutomationRule> {
    return this.request(
      this.http.patch<AutomationRule>(
        this.workspaceUrl(workspaceId, `automations/${rule.id}/enabled`),
        { enabled },
        { headers: { 'If-Match': `"${rule.version}"` } },
      ),
    );
  }

  listMembers(workspaceId: string): Promise<WorkspaceMember[]> {
    return this.request(
      this.http.get<WorkspaceMember[]>(this.workspaceUrl(workspaceId, 'members')),
    );
  }

  inviteMember(
    workspaceId: string,
    email: string,
    role: WorkspaceRole,
  ): Promise<WorkspaceInvitation> {
    return this.request(
      this.http.post<WorkspaceInvitation>(this.workspaceUrl(workspaceId, 'invitations'), {
        email,
        role,
        expiresInHours: 72,
      }),
    );
  }

  updateMemberRole(
    workspaceId: string,
    membershipId: string,
    role: WorkspaceRole,
  ): Promise<MembershipMutation> {
    return this.request(
      this.http.patch<MembershipMutation>(
        this.workspaceUrl(workspaceId, `members/${membershipId}/role`),
        { role },
      ),
    );
  }

  listMailboxAccounts(workspaceId: string): Promise<MailboxAccount[]> {
    return this.request(
      this.http.get<MailboxAccount[]>(this.workspaceUrl(workspaceId, 'mail/accounts')),
    );
  }

  createMailboxAccount(workspaceId: string, body: MailboxAccountInput): Promise<MailboxAccount> {
    return this.request(
      this.http.post<MailboxAccount>(this.workspaceUrl(workspaceId, 'mail/accounts'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  updateMailboxAccount(
    workspaceId: string,
    account: MailboxAccount,
    body: MailboxAccountUpdate,
  ): Promise<MailboxAccount> {
    return this.request(
      this.http.put<MailboxAccount>(
        this.workspaceUrl(workspaceId, `mail/accounts/${account.id}`),
        body,
        { headers: { 'If-Match': `"${account.version}"` } },
      ),
    );
  }

  deleteMailboxAccount(workspaceId: string, account: MailboxAccount): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `mail/accounts/${account.id}`), {
        headers: { 'If-Match': `"${account.version}"` },
      }),
    );
  }

  syncMailboxAccount(workspaceId: string, accountId: string): Promise<{ synced: true }> {
    return this.request(
      this.http.post<{ synced: true }>(
        this.workspaceUrl(workspaceId, `mail/accounts/${accountId}/sync`),
        null,
      ),
    );
  }

  listMailboxFolders(workspaceId: string, accountId: string): Promise<MailboxFolder[]> {
    return this.request(
      this.http.get<MailboxFolder[]>(
        this.workspaceUrl(workspaceId, `mail/accounts/${accountId}/folders`),
      ),
    );
  }

  listMailboxMessages(
    workspaceId: string,
    folderId: string,
    cursor?: string,
  ): Promise<MailboxMessagePage> {
    let params = new HttpParams().set('limit', 50);
    if (cursor) params = params.set('cursor', cursor);
    return this.request(
      this.http.get<MailboxMessagePage>(
        this.workspaceUrl(workspaceId, `mail/folders/${folderId}/messages`),
        { params },
      ),
    );
  }

  readMailboxMessageBody(workspaceId: string, messageId: string): Promise<MailboxMessageBody> {
    return this.request(
      this.http.get<MailboxMessageBody>(
        this.workspaceUrl(workspaceId, `mail/messages/${messageId}/body`),
      ),
    );
  }

  sendMailboxMessage(
    workspaceId: string,
    accountId: string,
    body: MailboxSendInput,
  ): Promise<MailboxSendResult> {
    return this.request(
      this.http.post<MailboxSendResult>(
        this.workspaceUrl(workspaceId, `mail/accounts/${accountId}/send`),
        body,
        { headers: { 'Idempotency-Key': crypto.randomUUID() } },
      ),
    );
  }

  listWorkspaceRoles(workspaceId: string): Promise<WorkspaceRoleDefinition[]> {
    return this.request(
      this.http.get<WorkspaceRoleDefinition[]>(this.workspaceUrl(workspaceId, 'roles')),
    );
  }

  createWorkspaceRole(
    workspaceId: string,
    body: WorkspaceRoleInput,
  ): Promise<WorkspaceRoleDefinition> {
    return this.request(
      this.http.post<WorkspaceRoleDefinition>(this.workspaceUrl(workspaceId, 'roles'), body),
    );
  }

  updateWorkspaceRole(
    workspaceId: string,
    role: WorkspaceRoleDefinition,
    body: WorkspaceRoleInput,
  ): Promise<WorkspaceRoleDefinition> {
    return this.request(
      this.http.patch<WorkspaceRoleDefinition>(
        this.workspaceUrl(workspaceId, `roles/${role.id}`),
        body,
        { headers: { 'If-Match': `"${role.version}"` } },
      ),
    );
  }

  deleteWorkspaceRole(workspaceId: string, role: WorkspaceRoleDefinition): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `roles/${role.id}`), {
        headers: { 'If-Match': `"${role.version}"` },
      }),
    );
  }

  assignWorkspaceRole(
    workspaceId: string,
    membershipId: string,
    roleId: string,
  ): Promise<MembershipMutation> {
    return this.request(
      this.http.patch<MembershipMutation>(
        this.workspaceUrl(workspaceId, `members/${membershipId}/role-assignment`),
        { roleId },
      ),
    );
  }

  updateMemberStatus(
    workspaceId: string,
    membershipId: string,
    status: 'active' | 'disabled',
  ): Promise<MembershipMutation> {
    return this.request(
      this.http.patch<MembershipMutation>(
        this.workspaceUrl(workspaceId, `members/${membershipId}/status`),
        { status },
      ),
    );
  }

  listTeams(workspaceId: string): Promise<Team[]> {
    return this.request(this.http.get<Team[]>(this.workspaceUrl(workspaceId, 'teams')));
  }

  listDepartments(workspaceId: string): Promise<Department[]> {
    return this.request(this.http.get<Department[]>(this.workspaceUrl(workspaceId, 'departments')));
  }

  createDepartment(workspaceId: string, name: string): Promise<Department> {
    return this.request(
      this.http.post<Department>(this.workspaceUrl(workspaceId, 'departments'), { name }),
    );
  }

  addDepartmentMember(
    workspaceId: string,
    departmentId: string,
    membershipId: string,
  ): Promise<void> {
    return this.request(
      this.http.put<void>(
        this.workspaceUrl(workspaceId, `departments/${departmentId}/members/${membershipId}`),
        null,
      ),
    );
  }

  createTeam(workspaceId: string, name: string): Promise<Team> {
    return this.request(this.http.post<Team>(this.workspaceUrl(workspaceId, 'teams'), { name }));
  }

  addTeamMember(workspaceId: string, teamId: string, membershipId: string): Promise<void> {
    return this.request(
      this.http.put<void>(
        this.workspaceUrl(workspaceId, `teams/${teamId}/members/${membershipId}`),
        null,
      ),
    );
  }

  listCustomFields(workspaceId: string, entityType?: string): Promise<CustomFieldDefinition[]> {
    let params = new HttpParams();
    if (entityType) params = params.set('entityType', entityType);
    return this.request(
      this.http.get<CustomFieldDefinition[]>(this.workspaceUrl(workspaceId, 'custom-fields'), {
        params,
      }),
    );
  }

  createCustomField(
    workspaceId: string,
    body: CustomFieldDefinitionInput,
  ): Promise<CustomFieldDefinition> {
    return this.request(
      this.http.post<CustomFieldDefinition>(this.workspaceUrl(workspaceId, 'custom-fields'), body, {
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    );
  }

  deleteCustomField(workspaceId: string, definition: CustomFieldDefinition): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `custom-fields/${definition.id}`), {
        headers: { 'If-Match': `"${definition.version}"` },
      }),
    );
  }

  listApiKeys(workspaceId: string): Promise<ApiKeyItem[]> {
    return this.request(this.http.get<ApiKeyItem[]>(this.workspaceUrl(workspaceId, 'api-keys')));
  }

  createApiKey(
    workspaceId: string,
    name: string,
    scopes: readonly ApiKeyScope[],
  ): Promise<GeneratedApiKey> {
    return this.request(
      this.http.post<GeneratedApiKey>(this.workspaceUrl(workspaceId, 'api-keys'), { name, scopes }),
    );
  }

  revokeApiKey(workspaceId: string, keyId: string): Promise<void> {
    return this.request(
      this.http.delete<void>(this.workspaceUrl(workspaceId, `api-keys/${keyId}`)),
    );
  }

  listWebhooks(workspaceId: string): Promise<WebhookSubscription[]> {
    return this.request(
      this.http.get<WebhookSubscription[]>(this.workspaceUrl(workspaceId, 'webhooks')),
    );
  }

  createWebhook(
    workspaceId: string,
    body: {
      readonly url: string;
      readonly eventTypes: readonly string[];
      readonly enabled: boolean;
      readonly timeoutSeconds: number;
      readonly maxAttempts: number;
    },
  ): Promise<GeneratedWebhook> {
    return this.request(
      this.http.post<GeneratedWebhook>(this.workspaceUrl(workspaceId, 'webhooks'), body),
    );
  }

  setWebhookEnabled(
    workspaceId: string,
    subscription: WebhookSubscription,
    enabled: boolean,
  ): Promise<WebhookSubscription> {
    return this.request(
      this.http.patch<WebhookSubscription>(
        this.workspaceUrl(workspaceId, `webhooks/${subscription.id}/enabled`),
        { enabled },
        { headers: { 'If-Match': `"${subscription.version}"` } },
      ),
    );
  }

  rotateWebhookSecret(
    workspaceId: string,
    subscription: WebhookSubscription,
  ): Promise<GeneratedWebhook> {
    return this.request(
      this.http.post<GeneratedWebhook>(
        this.workspaceUrl(workspaceId, `webhooks/${subscription.id}/rotate-secret`),
        { overlapSeconds: 3600 },
        { headers: { 'If-Match': `"${subscription.version}"` } },
      ),
    );
  }

  listWebhookDeliveries(
    workspaceId: string,
    options: {
      readonly subscriptionId?: string;
      readonly cursor?: string;
      readonly limit?: number;
    } = {},
  ): Promise<WebhookDeliveryPage> {
    let params = new HttpParams();
    if (options.subscriptionId) params = params.set('subscriptionId', options.subscriptionId);
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.limit) params = params.set('limit', options.limit);
    return this.request(
      this.http.get<WebhookDeliveryPage>(this.workspaceUrl(workspaceId, 'webhook-deliveries'), {
        params,
      }),
    );
  }

  retryWebhookDelivery(workspaceId: string, deliveryId: string): Promise<void> {
    return this.request(
      this.http.post<void>(
        this.workspaceUrl(workspaceId, `webhook-deliveries/${deliveryId}/retry`),
        null,
      ),
    );
  }

  search(workspaceId: string, query: string): Promise<SearchResult[]> {
    return this.request(
      this.http.get<SearchResult[]>(this.workspaceUrl(workspaceId, 'search'), {
        params: { q: query },
      }),
    );
  }

  searchStream(workspaceId: string, query: string): Observable<SearchResult[]> {
    return this.http
      .get<SearchResult[]>(this.workspaceUrl(workspaceId, 'search'), { params: { q: query } })
      .pipe(catchError((error: HttpErrorResponse) => throwError(() => ApiError.from(error))));
  }

  listAudit(workspaceId: string): Promise<AuditEvent[]> {
    return this.request(this.http.get<AuditEvent[]>(this.workspaceUrl(workspaceId, 'audit')));
  }

  listTranslations(
    workspaceId: string,
    options: {
      locale?: string;
      namespace?: string;
      status?: TranslationStatus;
      query?: string;
      cursor?: string;
      limit?: number;
    } = {},
  ): Promise<ContentTranslationPage> {
    let params = new HttpParams();
    if (options.locale) params = params.set('locale', options.locale);
    if (options.namespace) params = params.set('namespace', options.namespace);
    if (options.status) params = params.set('status', options.status);
    if (options.query) params = params.set('q', options.query);
    if (options.cursor) params = params.set('cursor', options.cursor);
    if (options.limit) params = params.set('limit', options.limit);
    return this.request(
      this.http.get<ContentTranslationPage>(this.workspaceUrl(workspaceId, 'translations'), {
        params,
      }),
    );
  }

  translationCoverage(workspaceId: string, locale: string): Promise<TranslationCoverage[]> {
    return this.request(
      this.http.get<TranslationCoverage[]>(this.workspaceUrl(workspaceId, 'translation-coverage'), {
        params: { locale },
      }),
    );
  }

  putTranslation(
    workspaceId: string,
    locale: string,
    namespace: string,
    key: string,
    version: number,
    body: PutContentTranslation,
  ): Promise<VersionedResponse<ContentTranslation>> {
    const path = [
      'translations',
      encodeURIComponent(locale),
      encodeURIComponent(namespace),
      encodeURIComponent(key),
    ].join('/');
    return this.response(
      this.http.put<ContentTranslation>(this.workspaceUrl(workspaceId, path), body, {
        headers: version > 0 ? { 'If-Match': `"${version}"` } : {},
        observe: 'response',
      }),
    );
  }

  localizationSettings(workspaceId: string): Promise<VersionedResponse<WorkspaceLocaleSettings>> {
    return this.response(
      this.http.get<WorkspaceLocaleSettings>(
        this.workspaceUrl(workspaceId, 'localization-settings'),
        { observe: 'response' },
      ),
    );
  }

  updateLocalizationSettings(
    workspaceId: string,
    version: number,
    body: UpdateWorkspaceLocaleSettings,
  ): Promise<VersionedResponse<WorkspaceLocaleSettings>> {
    return this.response(
      this.http.patch<WorkspaceLocaleSettings>(
        this.workspaceUrl(workspaceId, 'localization-settings'),
        body,
        { headers: { 'If-Match': `"${version}"` }, observe: 'response' },
      ),
    );
  }

  listAttachments(
    workspaceId: string,
    entityType: AttachmentEntityType,
    entityId: string,
  ): Promise<Attachment[]> {
    return this.request(
      this.http.get<Attachment[]>(this.workspaceUrl(workspaceId, 'attachments'), {
        params: { entityType, entityId, limit: 50 },
      }),
    );
  }

  uploadAttachment(
    workspaceId: string,
    entityType: AttachmentEntityType,
    entityId: string,
    file: File,
  ): Promise<Attachment> {
    const body = new FormData();
    body.append('file', file, file.name);
    return this.request(
      this.http.post<Attachment>(this.workspaceUrl(workspaceId, 'attachments'), body, {
        params: { entityType, entityId },
      }),
    );
  }

  downloadAttachment(workspaceId: string, attachmentId: string): Promise<Blob> {
    return this.request(
      this.http.get(
        this.workspaceUrl(workspaceId, `attachments/${encodeURIComponent(attachmentId)}`),
        {
          responseType: 'blob',
        },
      ),
    );
  }

  deleteAttachment(workspaceId: string, attachmentId: string): Promise<void> {
    return this.request(
      this.http.delete<void>(
        this.workspaceUrl(workspaceId, `attachments/${encodeURIComponent(attachmentId)}`),
      ),
    );
  }

  private replaceRecordAssignments(
    url: string,
    version: number,
    assignments: readonly RecordAssignmentInput[],
  ): Promise<RecordAssignmentSet> {
    return this.request(
      this.http.put<RecordAssignmentSet>(
        url,
        { assignments },
        {
          headers: { 'If-Match': `"${version}"` },
        },
      ),
    );
  }

  private workspaceUrl(workspaceId: string, resource: string): string {
    return `${this.baseUrl}/workspaces/${encodeURIComponent(workspaceId)}/${resource}`;
  }

  private callAction(
    workspaceId: string,
    callId: string,
    action: 'decline' | 'leave',
  ): Promise<void> {
    return this.request(
      this.http.post<void>(
        this.workspaceUrl(workspaceId, `calls/${encodeURIComponent(callId)}/${action}`),
        null,
      ),
    );
  }

  private request<T>(observable: Observable<T>): Promise<T> {
    return firstValueFrom(
      observable.pipe(
        catchError((error: HttpErrorResponse) => throwError(() => ApiError.from(error))),
      ),
    );
  }

  private async response<T>(
    observable: Observable<HttpResponse<T>>,
  ): Promise<VersionedResponse<T>> {
    const response = await this.request(observable);
    if (response.body === null) throw new ApiError(500, null);
    return { body: response.body, etag: response.headers.get('ETag') };
  }
}
