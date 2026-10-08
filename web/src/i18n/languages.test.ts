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
  it('keeps certificate expiry keys present in both catalogs', () => {
    const paths = [
      'connectors.tlsProbe.targetsLabel',
      'connectors.tlsProbe.targetsHint',
      'connectors.tlsProbe.importConnectorLabel',
      'connectors.tlsProbe.importConnectorHint',
      'connectors.tlsProbe.importPortLabel',
      'connectors.tlsProbe.importPortHint',
      'connectors.tlsProbe.adminOnlyHint',
      'connectors.tlsProbe.noTraefikImport',
      'connectors.tlsProbe.currentImportUnavailable',
      'compliance.daysLeftLt',
      'compliance.daysLeftGt',
      'compliance.certificateExpiryOffer.title',
      'compliance.certificateExpiryOffer.description',
      'compliance.certificateExpiryOffer.install',
      'compliance.certificateExpiryOffer.installing',
      'compliance.certificateExpiryOffer.notNow',
      'compliance.certificateExpiryOffer.installError',
      'dashboard.widget.certificates',
      'widgets.loadCertificatesError',
      'widgets.certificates.emptyTitle',
      'widgets.certificates.emptyDesc',
      'widgets.certificates.band.expired',
      'widgets.certificates.band.week',
      'widgets.certificates.band.month',
      'widgets.certificates.band.later',
      'widgets.certificates.daysLeft_one',
      'widgets.certificates.daysLeft_other',
      'widgets.certificates.expiredDaysAgo_one',
      'widgets.certificates.expiredDaysAgo_other',
      'widgets.certificates.expiredRecently',
      'widgets.certificates.unreachable',
    ];
    for (const catalog of [en, ptBR]) {
      const translatedKeys = new Set(keys(catalog));
      expect(paths.filter((path) => !translatedKeys.has(path))).toEqual([]);
    }
  });
  it('keeps the network discovery keys present in both catalogs', () => {
    const paths = [
      'discovery.title',
      'discovery.lead',
      'discovery.note',
      'discovery.rangeLabel',
      'discovery.suggestions',
      'discovery.source.client',
      'discovery.source.server',
      'discovery.start',
      'discovery.scanAgain',
      'discovery.cancel',
      'discovery.running',
      'discovery.progress',
      'discovery.summary',
      'discovery.partial',
      'discovery.state.running',
      'discovery.state.completed',
      'discovery.state.cancelled',
      'discovery.state.failed',
      'discovery.noResults',
      'discovery.noResultsDetail',
      'discovery.alreadyConnected',
      'discovery.viewConnector',
      'discovery.connectSelected_one',
      'discovery.connectSelected_other',
      'discovery.errors.invalidRange',
      'discovery.errors.conflict',
      'discovery.errors.rateLimited',
      'discovery.errors.rateLimitedSoon',
      'discovery.errors.generic',
      'discovery.queue.heading',
      'discovery.queue.position',
      'discovery.queue.skip',
      'discovery.queue.stop',
      'onboarding.connect.manualTitle',
      'onboarding.connect.continueWithConnected_one',
      'onboarding.connect.continueWithConnected_other',
      'onboarding.sync.titleMany',
      'connectors.scanNetwork',
      'connectors.backToForm',
    ];
    for (const catalog of [en, ptBR]) {
      const translatedKeys = new Set(keys(catalog));
      expect(paths.filter((path) => !translatedKeys.has(path))).toEqual([]);
    }
  });
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
        .reduce<unknown>(
          (value, key) => (value as Record<string, unknown> | undefined)?.[key],
          catalog
        );

    for (const path of paths) {
      expect(lookup(en, path), `English translation for ${path}`).toEqual(expect.any(String));
      expect(lookup(ptBR, path), `pt-BR translation for ${path}`).toEqual(expect.any(String));
    }
  });
});

describe('connector category labels (#513)', () => {
  const lookup = (catalog: object, path: string) =>
    path
      .split('.')
      .reduce<unknown>((o, k) => (o as Record<string, unknown> | undefined)?.[k], catalog);

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
