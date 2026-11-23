import authZhCN from '../../../web/src/locales/auth/zh-CN';
import coreZhCN from '../../../web/src/locales/core/zh-CN';
import gameDataZhCN from '../../../web/src/locales/game-data/zh-CN';
import gameZhCN from '../../../web/src/locales/game/zh-CN';
import homeZhCN from '../../../web/src/locales/home/zh-CN';
import tutorialZhCN from '../../../web/src/locales/tutorial/zh-CN';

type LocaleNode = Record<string, unknown>;

const locale = {
  ...coreZhCN,
  ...homeZhCN,
  ...authZhCN,
  ...gameZhCN,
  ...gameDataZhCN,
  ...tutorialZhCN
} as LocaleNode;

function lookup(path: string): unknown {
  return path.split('.').reduce<unknown>((node, key) => {
    if (!node || typeof node !== 'object' || Array.isArray(node)) return undefined;
    return (node as LocaleNode)[key];
  }, locale);
}

export function t(key: string, params: Record<string, string | number> = {}): string {
  const raw = lookup(key);
  if (typeof raw !== 'string') return key;
  return Object.entries(params).reduce((text, [name, value]) => text.split(`{${name}}`).join(String(value)), raw);
}

export function localizeKey(key: unknown, fallback = ''): string {
  const normalized = String(key || '').trim();
  if (!normalized) return fallback;
  const translated = t(normalized);
  return translated === normalized ? fallback || normalized : translated;
}
