import { afterEach, describe, expect, it } from 'vitest';
import i18n, { setLanguagePreference } from '.';
import { en } from './en';
import { LANGUAGES, detectLanguage, getLanguagePreference, resolveLanguage } from './languages';

function keys(obj: object, prefix = ''): string[] {
  return Object.entries(obj).flatMap(([k, v]) =>
    v && typeof v === 'object' ? keys(v, `${prefix}${k}.`) : [`${prefix}${k}`]
  );
}

describe('detectLanguage', () => {
  it('matches the exact tag, case-insensitively', () => {
    expect(detectLanguage(['pt-br'])).toBe('pt-BR');
  });
  it('falls back to the primary subtag', () => {
    expect(detectLanguage(['pt-PT'])).toBe('pt-BR');
    expect(detectLanguage(['pt'])).toBe('pt-BR');
  });
  it('honours browser preference order and skips unknown languages', () => {
    expect(detectLanguage(['xx', 'pt-BR', 'en'])).toBe('pt-BR');
    expect(detectLanguage(['en-US', 'pt-BR'])).toBe('en');
  });
  it('defaults to English', () => {
    expect(detectLanguage(['de-DE'])).toBe('en');
    expect(detectLanguage([])).toBe('en');
  });
  it('resolves an explicit preference without consulting the browser', () => {
    expect(resolveLanguage('pt-BR')).toBe('pt-BR');
  });
});

describe('locale catalogs', () => {
  it.each(Object.entries(LANGUAGES).filter(([, l]) => l.load))(
    '%s only uses keys that exist in English',
    async (_code, lang) => {
      const catalog = await lang.load!();
      const known = new Set(keys(en));
      expect(keys(catalog).filter((k) => !known.has(k))).toEqual([]);
    }
  );
});

describe('setLanguagePreference', () => {
  afterEach(async () => {
    localStorage.clear();
    await setLanguagePreference('en');
  });

  it('lazy-loads the locale, persists the choice and sets <html lang>', async () => {
    await setLanguagePreference('pt-BR');
    expect(i18n.t('nav.settings')).toBe('Configurações');
    expect(getLanguagePreference()).toBe('pt-BR');
    expect(document.documentElement.lang).toBe('pt-BR');
  });

  it('falls back to English for untranslated keys', async () => {
    await setLanguagePreference('pt-BR');
    expect(i18n.t('settings.motion.fullLabel')).toBe(en.settings.motion.fullLabel);
  });
});
