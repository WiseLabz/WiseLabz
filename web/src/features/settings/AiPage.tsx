/**
 * Settings → AI (operator-only). Configure the opt-in, provider-agnostic AI doc
 * module and run a connection test. The API key is write-only: it's never returned
 * by the API, so the field stays blank on load and is only sent when re-entered.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetAiConfig,
  putAiConfig,
  postAiConfigTest,
  getGetAiConfigQueryKey,
} from '../../api/generated/settings/settings';
import { AiConfigMode, AiConfigProvider } from '../../api/model';
import type { AiConfig, TestResult } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState } from '../../components/ui/states';
import { ToneTag } from '../../components/ui/ToneTag';
import { toast } from '../../lib/toast';
import { SubHeader, Section, Field, TextInput, Select, ToggleRow } from './parts';

export function AiPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetAiConfig();

  const [form, setForm] = useState<AiConfig | null>(null);
  const [apiKey, setApiKey] = useState('');
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  // Seed the editable form from server data, re-seeding whenever the query
  // returns a fresh reference (e.g. after an invalidate). Adjusting state during
  // render is the React-blessed alternative to a syncing effect.
  const [seeded, setSeeded] = useState<AiConfig | null>(null);
  if (data && data !== seeded) {
    setSeeded(data);
    setForm({ ...data });
  }

  const save = useMutation({
    mutationFn: (body: AiConfig) => putAiConfig(apiKey ? { ...body, apiKey } : body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetAiConfigQueryKey() });
      toast.success(t('settings.ai.saved', { defaultValue: 'AI settings saved.' }));
      setApiKey('');
    },
    onError: () =>
      toast.error(t('settings.ai.saveError', { defaultValue: 'Could not save AI settings.' })),
  });

  const test = useMutation({
    mutationFn: () => postAiConfigTest(),
    onSuccess: (result) => {
      setTestResult(result);
      if (result.ok)
        toast.success(t('settings.ai.testOk', { defaultValue: 'AI provider reachable.' }));
      else
        toast.error(
          result.message ?? t('settings.ai.testFail', { defaultValue: 'AI test failed.' })
        );
    },
    onError: () => toast.error(t('settings.ai.testFail', { defaultValue: 'AI test failed.' })),
  });

  if (isLoading) return <Loading />;
  if (isError || !data || !form)
    return (
      <div>
        <SubHeader title={t('settings.ai.title', { defaultValue: 'AI' })} />
        <ErrorState
          description={t('settings.ai.loadError', { defaultValue: 'Could not load AI settings.' })}
          onRetry={() => refetch()}
        />
      </div>
    );

  const set = <K extends keyof AiConfig>(key: K, value: AiConfig[K]) =>
    setForm((f) => (f ? { ...f, [key]: value } : f));
  const isOllama = form.provider === AiConfigProvider.ollama;

  return (
    <div>
      <SubHeader
        title={t('settings.ai.title', { defaultValue: 'AI' })}
        description={t('settings.ai.subtitle', {
          defaultValue: 'Opt-in, provider-agnostic AI doc assistance.',
        })}
      />

      <Section title={t('settings.ai.moduleTitle', { defaultValue: 'AI module' })}>
        <ToggleRow
          title={t('settings.ai.enableTitle', { defaultValue: 'Enable AI doc assistance' })}
          description={t('settings.ai.enableDesc', {
            defaultValue: 'When off, the diff engine emits alerts instead of drafting doc updates.',
          })}
          checked={form.enabled}
          onChange={(enabled) => set('enabled', enabled)}
        />
      </Section>

      <Section title={t('settings.ai.providerTitle', { defaultValue: 'Provider' })}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label={t('settings.ai.provider', { defaultValue: 'Provider' })}
            htmlFor="ai-provider"
          >
            <Select
              id="ai-provider"
              value={form.provider ?? ''}
              disabled={!form.enabled}
              onChange={(e) => set('provider', (e.target.value || null) as AiConfig['provider'])}
            >
              <option value="">{t('common.none', { defaultValue: 'None' })}</option>
              <option value={AiConfigProvider.anthropic}>Anthropic</option>
              <option value={AiConfigProvider.openai}>OpenAI</option>
              <option value={AiConfigProvider.ollama}>Ollama</option>
            </Select>
          </Field>
          <Field label={t('settings.ai.model', { defaultValue: 'Model' })} htmlFor="ai-model">
            <TextInput
              id="ai-model"
              value={form.model ?? ''}
              disabled={!form.enabled}
              placeholder="claude-sonnet-4-5"
              onChange={(e) => set('model', e.target.value)}
            />
          </Field>
          {isOllama ? (
            <Field
              label={t('settings.ai.baseUrl', { defaultValue: 'Base URL' })}
              htmlFor="ai-baseurl"
              hint={t('settings.ai.baseUrlHint', {
                defaultValue: 'For Ollama / self-hosted endpoints.',
              })}
            >
              <TextInput
                id="ai-baseurl"
                value={form.baseUrl ?? ''}
                disabled={!form.enabled}
                placeholder="http://localhost:11434"
                onChange={(e) => set('baseUrl', e.target.value)}
              />
            </Field>
          ) : (
            <Field
              label={t('settings.ai.apiKey', { defaultValue: 'API key' })}
              htmlFor="ai-apikey"
              hint={t('settings.ai.apiKeyHint', {
                defaultValue: 'Write-only — leave blank to keep the existing key.',
              })}
            >
              <TextInput
                id="ai-apikey"
                type="password"
                autoComplete="off"
                value={apiKey}
                disabled={!form.enabled}
                placeholder="••••••••••••"
                onChange={(e) => setApiKey(e.target.value)}
              />
            </Field>
          )}
          <Field
            label={t('settings.ai.mode', { defaultValue: 'Update mode' })}
            htmlFor="ai-mode"
            hint={t('settings.ai.modeHint', { defaultValue: 'How AI drafts are applied to docs.' })}
          >
            <Select
              id="ai-mode"
              value={form.mode}
              disabled={!form.enabled}
              onChange={(e) => set('mode', e.target.value as AiConfig['mode'])}
            >
              <option value={AiConfigMode.suggest_only}>
                {t('settings.ai.modeSuggest', {
                  defaultValue: 'Suggest only (review before apply)',
                })}
              </option>
              <option value={AiConfigMode.auto_update}>
                {t('settings.ai.modeAuto', { defaultValue: 'Auto-update doc sections' })}
              </option>
            </Select>
          </Field>
        </div>

        <div className="mt-4 flex items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Button
              variant="secondary"
              size="sm"
              disabled={!form.enabled || test.isPending}
              onClick={() => test.mutate()}
            >
              {test.isPending
                ? t('settings.ai.testing', { defaultValue: 'Testing…' })
                : t('settings.ai.test', { defaultValue: 'Test connection' })}
            </Button>
            {testResult && (
              <span className="flex items-center gap-2 text-2xs text-ink-muted">
                <ToneTag
                  tone={testResult.ok ? 'ok' : 'err'}
                  label={
                    testResult.ok
                      ? t('settings.ai.ok', { defaultValue: 'reachable' })
                      : t('settings.ai.failed', { defaultValue: 'failed' })
                  }
                />
                {testResult.latencyMs != null && (
                  <span className="font-mono">{testResult.latencyMs}ms</span>
                )}
              </span>
            )}
          </div>
          <Button
            variant="primary"
            size="sm"
            disabled={save.isPending}
            onClick={() => save.mutate(form)}
          >
            {t('common.save', { defaultValue: 'Save' })}
          </Button>
        </div>
      </Section>
    </div>
  );
}

function Loading() {
  return (
    <div>
      <SubHeader title="AI" />
      <SkeletonRows rows={6} />
    </div>
  );
}
