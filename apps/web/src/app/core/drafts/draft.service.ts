import { InjectionToken, Injectable, inject } from '@angular/core';

import { openAppDatabase } from '../storage/app-database';

export type JsonValue =
  null | boolean | number | string | JsonValue[] | { [key: string]: JsonValue };

export interface DraftKey {
  readonly workspaceId: string;
  readonly feature: string;
  readonly recordId: string;
}

export interface DraftRecord<T extends JsonValue = JsonValue> {
  readonly id: string;
  readonly formatVersion: 1;
  readonly workspaceId: string;
  readonly feature: string;
  readonly recordId: string;
  readonly value: T;
  readonly bytes: number;
  readonly updatedAt: number;
  readonly expiresAt: number;
}

export interface DraftLimits {
  readonly maxEntries: number;
  readonly maxEntryBytes: number;
  readonly maxTotalBytes: number;
  readonly ttlMs: number;
}

export interface DraftBackend {
  list(): Promise<DraftRecord[]>;
  put(record: DraftRecord): Promise<void>;
  delete(id: string): Promise<void>;
}

export interface DraftClock {
  now(): number;
}

export const DRAFT_LIMITS = new InjectionToken<DraftLimits>('DRAFT_LIMITS', {
  providedIn: 'root',
  factory: () => ({
    maxEntries: 32,
    maxEntryBytes: 64 * 1024,
    maxTotalBytes: 512 * 1024,
    ttlMs: 7 * 24 * 60 * 60 * 1000,
  }),
});

export const DRAFT_CLOCK = new InjectionToken<DraftClock>('DRAFT_CLOCK', {
  providedIn: 'root',
  factory: () => ({ now: () => Date.now() }),
});

export const DRAFT_BACKEND = new InjectionToken<DraftBackend>('DRAFT_BACKEND', {
  providedIn: 'root',
  factory: () => new IndexedDbDraftBackend(),
});

export class DraftQuotaError extends Error {
  constructor(readonly reason: 'entry-too-large' | 'storage-full') {
    super(reason);
  }
}

@Injectable({ providedIn: 'root' })
export class DraftService {
  private readonly backend = inject(DRAFT_BACKEND);
  private readonly limits = inject(DRAFT_LIMITS);
  private readonly clock = inject(DRAFT_CLOCK);

  async save<T extends JsonValue>(key: DraftKey, value: T): Promise<DraftRecord<T>> {
    const now = this.clock.now();
    const bytes = new TextEncoder().encode(JSON.stringify(value)).byteLength;
    if (bytes > this.limits.maxEntryBytes) throw new DraftQuotaError('entry-too-large');
    if (bytes > this.limits.maxTotalBytes) throw new DraftQuotaError('storage-full');

    const id = draftId(key);
    const records = await this.prune(now);
    const candidates = records
      .filter((record) => record.id !== id)
      .sort((left, right) => left.updatedAt - right.updatedAt);
    let totalBytes = candidates.reduce((total, record) => total + record.bytes, 0);
    while (
      candidates.length >= this.limits.maxEntries ||
      totalBytes + bytes > this.limits.maxTotalBytes
    ) {
      const oldest = candidates.shift();
      if (!oldest) break;
      totalBytes -= oldest.bytes;
      await this.backend.delete(oldest.id);
    }
    if (totalBytes + bytes > this.limits.maxTotalBytes) throw new DraftQuotaError('storage-full');

    const record: DraftRecord<T> = {
      id,
      formatVersion: 1,
      workspaceId: key.workspaceId,
      feature: key.feature,
      recordId: key.recordId,
      value,
      bytes,
      updatedAt: now,
      expiresAt: now + this.limits.ttlMs,
    };
    try {
      await this.backend.put(record);
    } catch (error) {
      if (error instanceof DOMException && error.name === 'QuotaExceededError')
        throw new DraftQuotaError('storage-full');
      throw error;
    }
    return record;
  }

  async load<T extends JsonValue>(key: DraftKey): Promise<T | null> {
    const id = draftId(key);
    const record = (await this.backend.list()).find((candidate) => candidate.id === id);
    if (!record) return null;
    if (record.formatVersion !== 1 || record.expiresAt <= this.clock.now()) {
      await this.backend.delete(id);
      return null;
    }
    return record.value as T;
  }

  async clear(key: DraftKey): Promise<void> {
    await this.backend.delete(draftId(key));
  }

  async clearWorkspace(workspaceId: string): Promise<void> {
    const records = await this.backend.list();
    await Promise.all(
      records
        .filter((record) => record.workspaceId === workspaceId)
        .map((record) => this.backend.delete(record.id)),
    );
  }

  async submitWithDraft<T>(key: DraftKey, submit: () => Promise<T>): Promise<T> {
    const result = await submit();
    await this.clear(key);
    return result;
  }

  private async prune(now: number): Promise<DraftRecord[]> {
    const records = await this.backend.list();
    const stale = records.filter((record) => record.formatVersion !== 1 || record.expiresAt <= now);
    await Promise.all(stale.map((record) => this.backend.delete(record.id)));
    return records.filter((record) => record.formatVersion === 1 && record.expiresAt > now);
  }
}

class IndexedDbDraftBackend implements DraftBackend {
  async list(): Promise<DraftRecord[]> {
    if (!('indexedDB' in globalThis)) return [];
    const database = await openAppDatabase();
    try {
      const request = database
        .transaction('drafts', 'readonly')
        .objectStore('drafts')
        .getAll() as IDBRequest<unknown[]>;
      const values = await requestResult(request);
      return Array.isArray(values) ? values.filter(isDraftRecord) : [];
    } finally {
      database.close();
    }
  }

  async put(record: DraftRecord): Promise<void> {
    if (!('indexedDB' in globalThis)) return;
    const database = await openAppDatabase();
    try {
      await transactionComplete(database, 'readwrite', (store) => store.put(record));
    } finally {
      database.close();
    }
  }

  async delete(id: string): Promise<void> {
    if (!('indexedDB' in globalThis)) return;
    const database = await openAppDatabase();
    try {
      await transactionComplete(database, 'readwrite', (store) => store.delete(id));
    } finally {
      database.close();
    }
  }
}

function draftId(key: DraftKey): string {
  return [key.workspaceId, key.feature, key.recordId].map(encodeURIComponent).join(':');
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error('Draft storage request failed'));
  });
}

function isDraftRecord(value: unknown): value is DraftRecord {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Partial<DraftRecord>;
  return (
    typeof candidate.id === 'string' &&
    candidate.formatVersion === 1 &&
    typeof candidate.workspaceId === 'string' &&
    typeof candidate.feature === 'string' &&
    typeof candidate.recordId === 'string' &&
    typeof candidate.bytes === 'number' &&
    typeof candidate.updatedAt === 'number' &&
    typeof candidate.expiresAt === 'number' &&
    'value' in candidate
  );
}

function transactionComplete(
  database: IDBDatabase,
  mode: IDBTransactionMode,
  operation: (store: IDBObjectStore) => IDBRequest,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const transaction = database.transaction('drafts', mode);
    operation(transaction.objectStore('drafts'));
    transaction.oncomplete = () => resolve();
    transaction.onerror = () =>
      reject(transaction.error ?? new Error('Draft storage transaction failed'));
    transaction.onabort = () =>
      reject(transaction.error ?? new Error('Draft storage transaction aborted'));
  });
}
