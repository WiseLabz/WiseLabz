/**
 * Locale registry + language preference. English is bundled (it is the
 * fallback for every missing key); every other locale is a lazy-loaded chunk.
 *
 * To add a locale: create `locales/<code>.ts` (see docs/TRANSLATING.md) and
 * add one entry to LANGUAGES below.
 */
import type { en } from './en';

/** A locale may translate any subset of the English catalog. */
export type Catalog = DeepPartial<typeof en>;
type DeepPartial<T> = {
  [K in keyof T]?: T[K] extends object ? DeepPartial<T[K]> : T[K] extends string ? string : T[K];
};

export interface Language {
  /** Native name, shown in the picker regardless of the active language. */
  name: string;
  /** Lazy loader; absent for the bundled English catalog. */
  load?: () => Promise<Catalog>;
}

export const FALLBACK_LANGUAGE = 'en';

export const LANGUAGES: Record<string, Language> = {
  en: { name: 'English' },
  'pt-BR': {
    name: 'Português (Brasil)',
    load: () => import('./locales/pt-BR').then((m) => m.ptBR),
  },
};

/** Stored preference: a LANGUAGES key, or 'auto' to follow the browser. */
export type LanguagePreference = 'auto' | string;

const STORAGE_KEY = 'wiselabz.language';

export function getLanguagePreference(): LanguagePreference {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved && (saved === 'auto' || saved in LANGUAGES)) return saved;
  } catch {
    /* storage unavailable */
  }
  return 'auto';
}

export function saveLanguagePreference(pref: LanguagePreference) {
  try {
    localStorage.setItem(STORAGE_KEY, pref);
  } catch {
    /* storage unavailable */
  }
}

/**
 * Match the browser's ordered language list against LANGUAGES: exact tag
 * first (pt-BR), then the primary subtag (pt → pt-BR), else English.
 */
export function detectLanguage(
  browserLanguages: readonly string[] = typeof navigator === 'undefined'
    ? []
    : navigator.languages?.length
      ? navigator.languages
      : [navigator.language]
): string {
  const codes = Object.keys(LANGUAGES);
  for (const tag of browserLanguages) {
    const lower = tag.toLowerCase();
    const exact = codes.find((c) => c.toLowerCase() === lower);
    if (exact) return exact;
    const primary = lower.split('-')[0];
    const partial = codes.find((c) => c.toLowerCase().split('-')[0] === primary);
    if (partial) return partial;
  }
  return FALLBACK_LANGUAGE;
}

export function resolveLanguage(pref: LanguagePreference): string {
  return pref === 'auto' ? detectLanguage() : pref;
}
