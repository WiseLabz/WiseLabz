/**
 * i18n bootstrap. Imported from main.tsx BEFORE first render so `t()` is
 * available everywhere. English is bundled and always the fallback; other
 * locales load on demand (see languages.ts). The language comes from the
 * stored Appearance setting, else the browser's language list.
 */
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import { en } from './en';
import {
  FALLBACK_LANGUAGE,
  LANGUAGES,
  getLanguagePreference,
  resolveLanguage,
  saveLanguagePreference,
  type LanguagePreference,
} from './languages';

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en } },
  lng: FALLBACK_LANGUAGE,
  fallbackLng: FALLBACK_LANGUAGE,
  interpolation: { escapeValue: false }, // React already escapes
  returnNull: false,
});

i18n.on('languageChanged', (lng) => {
  if (typeof document !== 'undefined') document.documentElement.lang = lng;
});

/** Load (if needed) and switch to a locale. Falls back to English on failure. */
async function activate(code: string) {
  const lang = LANGUAGES[code];
  if (lang?.load && !i18n.hasResourceBundle(code, 'translation')) {
    try {
      i18n.addResourceBundle(code, 'translation', await lang.load());
    } catch {
      code = FALLBACK_LANGUAGE;
    }
  }
  await i18n.changeLanguage(code);
}

/** Apply a language preference ('auto' follows the browser) and remember it. */
export async function setLanguagePreference(pref: LanguagePreference) {
  saveLanguagePreference(pref);
  await activate(resolveLanguage(pref));
}

/** Resolves once the initial locale is loaded; main.tsx awaits it before render. */
export const i18nReady = activate(resolveLanguage(getLanguagePreference()));

export default i18n;
