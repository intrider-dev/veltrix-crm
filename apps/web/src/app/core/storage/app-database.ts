import { productConfig } from '@veltrix-crm/product-config';

const databaseVersion = 2;
const databaseName = `${productConfig.cookiePrefix}-app-state`;

export function openAppDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(databaseName, databaseVersion);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains('preferences')) {
        database.createObjectStore('preferences');
      }
      if (!database.objectStoreNames.contains('drafts')) {
        const drafts = database.createObjectStore('drafts', { keyPath: 'id' });
        drafts.createIndex('workspace', 'workspaceId', { unique: false });
        drafts.createIndex('expiresAt', 'expiresAt', { unique: false });
      }
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error('Unable to open application state'));
    request.onblocked = () => reject(new Error('Application state upgrade is blocked'));
  });
}
