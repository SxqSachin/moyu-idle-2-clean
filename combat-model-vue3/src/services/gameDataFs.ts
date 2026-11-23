import { COMBAT_TABLES, type Catalog, type CombatTable, createEmptyCatalog } from '../model/catalog';
import { isNotFoundError, parsePresentTableText, type TableReadResult } from './sourceTableState';

const ROOT_DIR_NAME = 'game-data';

type FileSystemDirectoryHandle = {
  name: string;
  getDirectoryHandle(name: string, options?: { create?: boolean }): Promise<FileSystemDirectoryHandle>;
  getFileHandle(name: string, options?: { create?: boolean }): Promise<FileSystemFileHandle>;
  queryPermission?(descriptor?: { mode?: 'read' | 'readwrite' }): Promise<PermissionState>;
  requestPermission?(descriptor?: { mode?: 'read' | 'readwrite' }): Promise<PermissionState>;
};

type FileSystemFileHandle = {
  getFile(): Promise<File>;
  createWritable(): Promise<{ write(data: string): Promise<void>; close(): Promise<void> }>;
};

declare global {
  interface Window {
    showDirectoryPicker?: (options?: { mode?: 'read' | 'readwrite' }) => Promise<FileSystemDirectoryHandle>;
    showOpenFilePicker?: (options?: {
      multiple?: boolean;
      types?: Array<{ description?: string; accept: Record<string, string[]> }>;
    }) => Promise<FileSystemFileHandle[]>;
  }
}

async function readJsonFile(fileHandle: FileSystemFileHandle): Promise<unknown> {
  const file = await fileHandle.getFile();
  return JSON.parse(await file.text()) as unknown;
}

async function readFileText(fileHandle: FileSystemFileHandle): Promise<string> {
  const file = await fileHandle.getFile();
  return await file.text();
}

const HANDLE_DB_NAME = 'combat-model-vue3-handles';
const HANDLE_STORE_NAME = 'handles';
const GAME_DATA_HANDLE_KEY = 'game-data-dir';

function openHandleDB(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(HANDLE_DB_NAME, 1);
    request.onupgradeneeded = () => {
      request.result.createObjectStore(HANDLE_STORE_NAME);
    };
    request.onsuccess = () => resolve(request.result);
    request.onerror = () => reject(request.error ?? new Error('无法打开目录授权缓存'));
  });
}

async function readStoredGameDataDir(): Promise<FileSystemDirectoryHandle | null> {
  if (typeof indexedDB === 'undefined') return null;
  try {
    const db = await openHandleDB();
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(HANDLE_STORE_NAME, 'readonly');
      const request = tx.objectStore(HANDLE_STORE_NAME).get(GAME_DATA_HANDLE_KEY);
      request.onsuccess = () => resolve((request.result as FileSystemDirectoryHandle | undefined) ?? null);
      request.onerror = () => reject(request.error ?? new Error('无法读取目录授权缓存'));
      tx.oncomplete = () => db.close();
      tx.onerror = () => db.close();
    });
  } catch {
    return null;
  }
}

async function storeGameDataDir(handle: FileSystemDirectoryHandle): Promise<void> {
  if (typeof indexedDB === 'undefined') return;
  try {
    const db = await openHandleDB();
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(HANDLE_STORE_NAME, 'readwrite');
      tx.objectStore(HANDLE_STORE_NAME).put(handle, GAME_DATA_HANDLE_KEY);
      tx.oncomplete = () => {
        db.close();
        resolve();
      };
      tx.onerror = () => {
        db.close();
        reject(tx.error ?? new Error('无法保存目录授权缓存'));
      };
    });
  } catch {
    // Older browsers may not support structured-cloning file handles. Loading still works for this session.
  }
}

async function ensureReadwritePermission(handle: FileSystemDirectoryHandle, prompt: boolean): Promise<boolean> {
  const descriptor = { mode: 'readwrite' as const };
  if (typeof handle.queryPermission === 'function') {
    const state = await handle.queryPermission(descriptor);
    if (state === 'granted') return true;
  }
  if (!prompt || typeof handle.requestPermission !== 'function') return false;
  return (await handle.requestPermission(descriptor)) === 'granted';
}

async function tryReadTableFromSourceJson(
  gameDataDirHandle: FileSystemDirectoryHandle,
  table: CombatTable
): Promise<TableReadResult> {
  const path = `source/json/${table}.json`;
  try {
    const sourceDir = await gameDataDirHandle.getDirectoryHandle('source');
    const jsonDir = await sourceDir.getDirectoryHandle('json');
    const file = await jsonDir.getFileHandle(`${table}.json`);
    return parsePresentTableText(await readFileText(file), table, path);
  } catch (error) {
    if (isNotFoundError(error)) return { present: false, rows: [] };
    if (error instanceof Error && error.message.startsWith(`${path}:`)) throw error;
    throw new Error(`${path}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

async function tryReadTableFromChunks(
  gameDataDirHandle: FileSystemDirectoryHandle,
  table: CombatTable
): Promise<TableReadResult> {
  const filename = table === 'items' ? 'core.json' : `${table}.json`;
  const path = `generated/chunks/${filename}`;
  try {
    const generated = await gameDataDirHandle.getDirectoryHandle('generated');
    const chunks = await generated.getDirectoryHandle('chunks');
    const file = await chunks.getFileHandle(filename);
    return parsePresentTableText(await readFileText(file), table, path);
  } catch (error) {
    if (isNotFoundError(error)) return { present: false, rows: [] };
    if (error instanceof Error && error.message.startsWith(`${path}:`)) throw error;
    throw new Error(`${path}: ${error instanceof Error ? error.message : String(error)}`);
  }
}

export async function requestGameDataDir(): Promise<FileSystemDirectoryHandle> {
  if (typeof window.showDirectoryPicker !== 'function') {
    throw new Error('当前浏览器不支持 File System Access API，请使用 Chrome/Edge 打开本工具。');
  }
  const cached = await readStoredGameDataDir();
  if (cached?.name === ROOT_DIR_NAME && await ensureReadwritePermission(cached, true)) {
    return cached;
  }
  const handle = await window.showDirectoryPicker({ mode: 'readwrite' });
  if (handle.name !== ROOT_DIR_NAME) {
    throw new Error('请选择 packages/game-data 目录。');
  }
  await storeGameDataDir(handle);
  return handle;
}

export async function restoreGameDataDir(): Promise<FileSystemDirectoryHandle | null> {
  const cached = await readStoredGameDataDir();
  if (cached?.name !== ROOT_DIR_NAME) return null;
  return await ensureReadwritePermission(cached, false) ? cached : null;
}

export async function loadCatalog(gameDataDirHandle: FileSystemDirectoryHandle): Promise<Catalog> {
  const catalog = createEmptyCatalog();
  for (const table of COMBAT_TABLES) {
    const fromSource = await tryReadTableFromSourceJson(gameDataDirHandle, table);
    const fromChunks = fromSource.present
      ? { present: false as const, rows: [] }
      : await tryReadTableFromChunks(gameDataDirHandle, table);
    const rows = fromSource.present ? fromSource.rows : fromChunks.rows;
    catalog[table] = rows.filter((row) => row && typeof row === 'object' && !Array.isArray(row)) as Catalog[CombatTable];
  }
  return catalog;
}

export async function saveCatalog(gameDataDirHandle: FileSystemDirectoryHandle, catalog: Catalog): Promise<void> {
  const sourceDir = await gameDataDirHandle.getDirectoryHandle('source', { create: true });
  const jsonDir = await sourceDir.getDirectoryHandle('json', { create: true });
  for (const table of COMBAT_TABLES) {
    const payload = {
      version: 1,
      [table]: Array.isArray(catalog[table]) ? catalog[table] : []
    };
    const file = await jsonDir.getFileHandle(`${table}.json`, { create: true });
    const writable = await file.createWritable();
    await writable.write(`${JSON.stringify(payload, null, 2)}\n`);
    await writable.close();
  }
}

export type AttributeExpProfileRow = {
  id: string;
  attr?: string;
  level: number;
  expRequired: number;
  [key: string]: unknown;
};

function normalizeAttributeExpProfiles(parsed: unknown): AttributeExpProfileRow[] {
  const rows: unknown[] = Array.isArray(parsed)
    ? parsed
    : parsed && typeof parsed === 'object' && Array.isArray((parsed as Record<string, unknown>).attribute_exp_profiles)
      ? ((parsed as Record<string, unknown>).attribute_exp_profiles as unknown[])
      : [];
  return rows
    .filter((row): row is Record<string, unknown> => Boolean(row) && typeof row === 'object' && !Array.isArray(row))
    .map((row) => ({
      ...row,
      id: String(row.id || '').trim(),
      attr: row.attr == null ? undefined : String(row.attr),
      level: Math.round(Number(row.level || 0)),
      expRequired: Math.round(Number(row.expRequired || 0))
    }))
    .filter((row) => row.id && row.level > 0 && row.expRequired >= 0);
}

function attributeExpProfileKey(row: AttributeExpProfileRow): string {
  return `${row.id}::${row.level}`;
}

function mergeAttributeExpProfiles(baseRows: AttributeExpProfileRow[], overrideRows: AttributeExpProfileRow[]): AttributeExpProfileRow[] {
  const out = [...baseRows];
  const indexByKey = new Map(out.map((row, index) => [attributeExpProfileKey(row), index]));
  for (const row of overrideRows) {
    const key = attributeExpProfileKey(row);
    const existingIndex = indexByKey.get(key);
    if (existingIndex == null) {
      indexByKey.set(key, out.length);
      out.push(row);
    } else {
      out[existingIndex] = row;
    }
  }
  return out;
}

export async function loadAttributeExpProfiles(gameDataDirHandle: FileSystemDirectoryHandle): Promise<AttributeExpProfileRow[]> {
  const fromSource = await (async () => {
    try {
      const sourceDir = await gameDataDirHandle.getDirectoryHandle('source');
      const jsonDir = await sourceDir.getDirectoryHandle('json');
      const file = await jsonDir.getFileHandle('attribute_exp_profiles.json');
      return normalizeAttributeExpProfiles(await readJsonFile(file));
    } catch {
      return [];
    }
  })();
  const fromChunks = await (async () => {
    try {
      const generated = await gameDataDirHandle.getDirectoryHandle('generated');
      const chunks = await generated.getDirectoryHandle('chunks');
      const file = await chunks.getFileHandle('attribute_exp_profiles.json');
      return normalizeAttributeExpProfiles(await readJsonFile(file));
    } catch {
      return [];
    }
  })();
  return mergeAttributeExpProfiles(fromChunks, fromSource);
}

export async function saveAttributeExpProfiles(
  gameDataDirHandle: FileSystemDirectoryHandle,
  rows: AttributeExpProfileRow[]
): Promise<void> {
  const sourceDir = await gameDataDirHandle.getDirectoryHandle('source', { create: true });
  const jsonDir = await sourceDir.getDirectoryHandle('json', { create: true });
  const file = await jsonDir.getFileHandle('attribute_exp_profiles.json', { create: true });
  const payload = {
    version: 1,
    attribute_exp_profiles: rows
  };
  const writable = await file.createWritable();
  await writable.write(`${JSON.stringify(payload, null, 2)}\n`);
  await writable.close();
}

export type GameDataDirectoryHandle = FileSystemDirectoryHandle;
export type GameDataFileHandle = FileSystemFileHandle;
