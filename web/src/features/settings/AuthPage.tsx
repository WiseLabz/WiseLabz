/**
 * Settings → Authentication (operator-only).
 *
 * SECURITY (ARCHITECTURE.md, decided 2026-06-25): OIDC providers are defined
 * entirely in config file / env. issuerUrl, clientId and the client secret NEVER
 * transit or are editable via the API — this page is READ-ONLY for provider
 * metadata and exposes only `secretConfigured` (presence, never the value) plus
 * the single safe per-provider enable/disable flag. The mutable instance settings
 * (local login, token TTLs, step-up) save via PUT /auth/config.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetAuthConfig,
  putAuthConfig,
  putAuthProvidersProviderIdEnabled,
  getGetAuthConfigQueryKey,
} from '../../api/generated/settings/settings';
import type { OidcProvider } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState } from '../../components/ui/states';
import { ToneTag } from '../../components/ui/ToneTag';
import { toast } from '../../lib/toast';
import { SubHeader, Section, Field, TextInput, ToggleRow, Toggle } from './parts';

/** Seconds → friendly "15 min" / "30 d" for the TTL hint. */
function humanizeSeconds(s: number): string {
  if (s % 86400 === 0) return `${s / 86400} d`;
  if (s % 3600 === 0) return `${s / 3600} h`;
  if (s % 60 === 0) return `${s / 60} min`;
  return `${s} s`;
}

export function AuthPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetAuthConfig();

  const [accessTtl, setAccessTtl] = useState('');
  const [refreshTtl, setRefreshTtl] = useState('');
  // Adjust state during render (React-blessed alternative to a syncing effect):
  // re-seed whenever the query yields a fresh reference, e.g. after an invalidate.
  const [seeded, setSeeded] = useState<typeof data | null>(null);
  if (data && data !== seeded) {
    setSeeded(data);
    setAccessTtl(String(data.accessTokenTtl));
    setRefreshTtl(String(data.refreshTokenTtl));
  }

  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetAuthConfigQueryKey() });

  const update = useMutation({
    mutationFn: (body: Parameters<typeof putAuthConfig>[0]) => putAuthConfig(body),
    onSuccess: () => {
      invalidate();
      toast.success(t('settings.auth.saved', { defaultValue: 'Authentication settings saved.' }));
    },
    onError: () =>
      toast.error(t('settings.auth.saveError', { defaultValue: 'Could not save settings.' })),
  });

  const toggleProvider = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      putAuthProvidersProviderIdEnabled(id, { enabled }),
    onSuccess: () => {
      invalidate();
      toast.success(t('settings.auth.providerUpdated', { defaultValue: 'Provider updated.' }));
    },
    onError: () =>
      toast.error(t('settings.auth.providerError', { defaultValue: 'Could not update provider.' })),
  });

  if (isLoading) return <Loading />;
  if (isError || !data)
    return (
      <div>
        <SubHeader title={t('settings.auth.title', { defaultValue: 'Authentication' })} />
        <ErrorState
          description={t('settings.auth.loadError', {
            defaultValue: 'Could not load authentication settings.',
          })}
          onRetry={() => refetch()}
        />
      </div>
    );

  const ttlDirty =
    Number(accessTtl) !== data.accessTokenTtl || Number(refreshTtl) !== data.refreshTokenTtl;
  const providers: OidcProvider[] = data.oidcProviders ?? [];

  return (
    <div>
      <SubHeader
        title={t('settings.auth.title', { defaultValue: 'Authentication' })}
        description={t('settings.auth.subtitle', {
          defaultValue: 'Sign-in methods, token lifetimes, and step-up protection.',
        })}
      />

      <Section title={t('settings.auth.methodsTitle', { defaultValue: 'Sign-in methods' })}>
        <div className="space-y-3">
          <ToggleRow
            title={t('settings.auth.localTitle', { defaultValue: 'Local username + password' })}
            description={t('settings.auth.localDesc', {
              defaultValue: 'Allow accounts stored in this instance to sign in directly.',
            })}
            checked={data.localEnabled}
            disabled={update.isPending}
            onChange={(localEnabled) => update.mutate({ localEnabled })}
          />
          <ToggleRow
            title={t('settings.auth.stepUpTitle', {
              defaultValue: 'Require re-auth before destructive actions',
            })}
            description={t('settings.auth.stepUpDesc', {
              defaultValue:
                'Removing a connector asks for your password (or 2FA) first. Recommended; turn off only on a single-admin homelab.',
            })}
            checked={data.stepUpForDestructive}
            disabled={update.isPending}
            onChange={(stepUpForDestructive) => update.mutate({ stepUpForDestructive })}
          />
        </div>
      </Section>

      <Section
        title={t('settings.auth.ttlTitle', { defaultValue: 'Token lifetimes' })}
        description={t('settings.auth.ttlDesc', {
          defaultValue: 'Lifetimes in seconds for issued access and refresh tokens.',
        })}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field
            label={t('settings.auth.accessTtl', { defaultValue: 'Access token TTL' })}
            htmlFor="access-ttl"
            hint={`≈ ${humanizeSeconds(Number(accessTtl) || 0)}`}
          >
            <TextInput
              id="access-ttl"
              type="number"
              min={60}
              value={accessTtl}
              onChange={(e) => setAccessTtl(e.target.value)}
            />
          </Field>
          <Field
            label={t('settings.auth.refreshTtl', { defaultValue: 'Refresh token TTL' })}
            htmlFor="refresh-ttl"
            hint={`≈ ${humanizeSeconds(Number(refreshTtl) || 0)}`}
          >
            <TextInput
              id="refresh-ttl"
              type="number"
              min={300}
              value={refreshTtl}
              onChange={(e) => setRefreshTtl(e.target.value)}
            />
          </Field>
        </div>
        <div className="mt-4 flex justify-end">
          <Button
            variant="primary"
            size="sm"
            disabled={!ttlDirty || update.isPending}
            onClick={() =>
              update.mutate({
                accessTokenTtl: Number(accessTtl),
                refreshTokenTtl: Number(refreshTtl),
              })
            }
          >
            {t('common.save', { defaultValue: 'Save' })}
          </Button>
        </div>
      </Section>

      <Section
        title={t('settings.auth.oidcTitle', { defaultValue: 'OIDC providers' })}
        description={t('settings.auth.oidcDesc', {
          defaultValue:
            'Defined in config.yaml / env and surfaced read-only. The client secret is never shown or editable here — only whether one is configured.',
        })}
      >
        {providers.length === 0 ? (
          <p className="rounded-lg border border-dashed border-line-soft px-4 py-6 text-center text-xs text-ink-muted">
            {t('settings.auth.noProviders', {
              defaultValue:
                'No OIDC providers detected. Add one in config.yaml / env to surface it here.',
            })}
          </p>
        ) : (
          <ul className="space-y-3">
            {providers.map((p) => (
              <li
                key={p.id}
                className="flex flex-wrap items-start justify-between gap-4 rounded-lg border border-line-soft p-4"
              >
                <div className="min-w-0 flex-1 space-y-2">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-medium text-ink">{p.displayName}</p>
                    <ToneTag tone="idle" label={p.source} />
                    <ToneTag
                      tone={p.secretConfigured ? 'ok' : 'warn'}
                      label={
                        p.secretConfigured
                          ? t('settings.auth.secretSet', { defaultValue: 'secret configured' })
                          : t('settings.auth.secretMissing', { defaultValue: 'no secret' })
                      }
                    />
                  </div>
                  <dl className="grid gap-x-6 gap-y-1 font-mono text-2xs text-ink-faint sm:grid-cols-2">
                    <div className="flex gap-1.5">
                      <dt className="text-ink-faint">
                        {t('settings.auth.issuer', { defaultValue: 'issuer' })}
                      </dt>
                      <dd className="truncate text-ink-muted">{p.issuerUrl || '—'}</dd>
                    </div>
                    <div className="flex gap-1.5">
                      <dt className="text-ink-faint">
                        {t('settings.auth.clientId', { defaultValue: 'client id' })}
                      </dt>
                      <dd className="truncate text-ink-muted">{p.clientId || '—'}</dd>
                    </div>
                  </dl>
                </div>
                <div className="flex items-center gap-2">
                  <span className="font-mono text-2xs text-ink-faint">
                    {p.enabled
                      ? t('common.enabled', { defaultValue: 'Enabled' })
                      : t('common.disabled', { defaultValue: 'Disabled' })}
                  </span>
                  <Toggle
                    checked={p.enabled}
                    disabled={toggleProvider.isPending}
                    onChange={(enabled) => toggleProvider.mutate({ id: p.id, enabled })}
                    label={t('settings.auth.toggleProvider', { defaultValue: 'Enable provider' })}
                  />
                </div>
              </li>
            ))}
          </ul>
        )}
      </Section>
    </div>
  );
}

function Loading() {
  return (
    <div>
      <SubHeader title="Authentication" />
      <SkeletonRows rows={6} />
    </div>
  );
}
