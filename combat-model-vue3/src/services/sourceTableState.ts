export type TableReadResult = { present: false; rows: [] } | { present: true; rows: unknown[] };

export function isNotFoundError(error: unknown): boolean {
  return Boolean(error) && typeof error === 'object' && (error as { name?: unknown }).name === 'NotFoundError';
}

export function tableRowsFromParsed(parsed: unknown, table: string, path: string): unknown[] {
  if (Array.isArray(parsed)) return parsed;
  if (parsed && typeof parsed === 'object' && Array.isArray((parsed as Record<string, unknown>)[table])) {
    return (parsed as Record<string, unknown>)[table] as unknown[];
  }
  throw new Error(`${path}: JSON 顶层必须是数组，或包含 ${table} 数组`);
}

export function parsePresentTableText(text: string, table: string, path: string): TableReadResult {
  try {
    return { present: true, rows: tableRowsFromParsed(JSON.parse(text) as unknown, table, path) };
  } catch (error) {
    if (error instanceof Error && error.message.startsWith(`${path}:`)) throw error;
    throw new Error(`${path}: ${error instanceof Error ? error.message : String(error)}`);
  }
}
