import { afterEach, describe, expect, it } from 'vitest';
import i18n, { setLanguagePreference } from '.';
import { en } from './en';
import { ptBR } from './locales/pt-BR';
import { ConnectorCategory } from '../api/model';
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

  it('keeps every runbook step-kind key translated in English and pt-BR', () => {
    const paths = [
      'runbooks.runs.kind.config_push',
      'runbooks.runs.kind.wait_for_entity',
      'runbooks.runs.fieldLabel',
      'runbooks.runs.configPushValueLabel',
      'runbooks.runs.currentToTarget',
      'runbooks.runs.unknownCurrentToTarget',
      'runbooks.runs.waitConditionLabel',
      'runbooks.runs.waitCondition',
      'runbooks.runs.timeoutReason',
      'runbooks.runs.blocked.unsupported_field',
      'settings.runbooks.steps.kinds.config_push',
      'settings.runbooks.steps.kinds.wait_for_entity',
      'settings.runbooks.steps.fieldLabel',
      'settings.runbooks.steps.fieldPlaceholder',
      'settings.runbooks.steps.valueLabel',
      'settings.runbooks.steps.valuePlaceholder',
      'settings.runbooks.steps.entityLabel',
      'settings.runbooks.steps.attributeLabel',
      'settings.runbooks.steps.operatorLabel',
      'settings.runbooks.steps.operators.eq',
      'settings.runbooks.steps.operators.neq',
      'settings.runbooks.steps.operators.contains',
      'settings.runbooks.steps.operators.regex',
      'settings.runbooks.steps.operators.gt',
      'settings.runbooks.steps.operators.lt',
      'settings.runbooks.steps.expectedValueLabel',
      'settings.runbooks.steps.timeoutMinutesLabel',
    ];
    const lookup = (catalog: object, path: string) =>
      path
        .split('.')
        .reduce<unknown>((value, key) => (value as Record<string, unknown> | undefined)?.[key], catalog);

    for (const path of paths) {
      expect(lookup(en, path), `English translation for ${path}`).toEqual(expect.any(String));
      expect(lookup(ptBR, path), `pt-BR translation for ${path}`).toEqual(expect.any(String));
    }
  });
});

describe('connector category labels (#513)', () => {
  const lookup = (catalog: object, path: string) =>
    path.split('.').reduce<unknown>((o, k) => (o as Record<string, unknown> | undefined)?.[k], catalog);

  it.each(['en', 'pt-BR'])('%s labels every connector category', async (code) => {
    const { load } = LANGUAGES[code];
    const catalog = load ? await load() : en;
    for (const cat of Object.values(ConnectorCategory)) {
      const label = lookup(catalog, `services.category.${cat}`);
      expect(label, `services.category.${cat}`).toEqual(expect.any(String));
      expect((label as string).trim(), `services.category.${cat}`).not.toBe('');
    }
  });
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
