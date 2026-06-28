/**
 * i18n bootstrap. Imported for its side effect from main.tsx BEFORE first render
 * so `t()` is available everywhere. English-only for v1; the infrastructure is in
 * place so community locales land without a retrofit. All copy goes through t().
 */
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import { en } from './en';

void i18n.use(initReactI18next).init({
  resources: { en: { translation: en } },
  lng: 'en',
  fallbackLng: 'en',
  interpolation: { escapeValue: false }, // React already escapes
  returnNull: false,
});

export default i18n;
