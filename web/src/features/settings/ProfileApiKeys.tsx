/** Settings → Profile → API keys: create (step-up gated), list, revoke. */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetAuthApiKeys,
  postAuthApiKeys,
  deleteAuthApiKeysId,
  getGetAuthApiKeysQueryKey,
} from '../../api/generated/auth/auth';
import { useGetConnectors } from '../../api/generated/connectors/connectors';
import type { ApiKey } from '../../api/model';
import { ApiKeyScope } from '../../api/model/apiKeyScope';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { Dialog } from '../../components/ui/Dialog';
import { TimeAgo } from '../../components/ui/TimeAgo';
import { ToneTag } from '../../components/ui/ToneTag';
import { copyText } from '../../lib/clipboard';
import { toast } from '../../lib/toast';
import { Section, Field, TextInput, Select } from './parts';
import { KeyIcon, CopyIcon } from '../../components/icons';
import { elevationOptions, useStepUpMutation } from '../../components/manager/useStepUpMutation';

export function ApiKeysSection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetAuthApiKeys();

  const [name, setName] = useState('');
  const [expiresAt, setExpiresAt] = useState('');
  const [scope, setScope] = useState<ApiKeyScope>(ApiKeyScope.full);
  const [connectorIds, setConnectorIds] = useState<string[]>([]);
  const [created, setCreated] = useState<string | null>(null);
  const [revoke, setRevoke] = useState<ApiKey | null>(null);
  const { data: connectors } = useGetConnectors();
  const connectorName = (id: string) => connectors?.find((c) => c.id === id)?.name ?? id;

  const create = useStepUpMutation({
    action: 'apiKey.create',
    mutationFn: (_: void, token) =>
      postAuthApiKeys(
        {
          name,
          expiresAt: expiresAt ? new Date(`${expiresAt}T23:59:59.999Z`).toISOString() : undefined,
          scope,
          connectorIds: connectorIds.length > 0 ? connectorIds : undefined,
        },
        elevationOptions(token)
      ),
    onSuccess: (result) => {
      queryClient.invalidateQueries({ queryKey: getGetAuthApiKeysQueryKey() });
      setCreated(result.token);
      setName('');
      setExpiresAt('');
      setScope(ApiKeyScope.full);
      setConnectorIds([]);
    },
    onError: () => toast.error(t('settings.profile.apiKeys.createError')),
  });

  const doRevoke = useMutation({
    mutationFn: (id: string) => deleteAuthApiKeysId(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetAuthApiKeysQueryKey() });
      toast.success(t('settings.profile.apiKeys.revoked'));
      setRevoke(null);
    },
    onError: () => toast.error(t('settings.profile.apiKeys.revokeError')),
  });

  return (
    <Section
      title={t('settings.profile.apiKeys.title')}
      description={t('settings.profile.apiKeys.subtitle')}
    >
      {create.dialog}
      <form
        className="grid gap-4 sm:grid-cols-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim() && !create.isPending) create.mutate();
        }}
      >
        <Field label={t('settings.profile.apiKeys.name')} htmlFor="apikey-name">
          <TextInput
            id="apikey-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t('settings.profile.apiKeys.namePlaceholder')}
          />
        </Field>
        <Field label={t('settings.profile.apiKeys.expiresAt')} htmlFor="apikey-expires">
          <TextInput
            id="apikey-expires"
            type="date"
            value={expiresAt}
            onChange={(e) => setExpiresAt(e.target.value)}
          />
        </Field>
        <Field
          label={t('settings.profile.apiKeys.scope')}
          htmlFor="apikey-scope"
          hint={t('settings.profile.apiKeys.scopeHint')}
        >
          <Select
            id="apikey-scope"
            value={scope}
            onChange={(e) => setScope(e.target.value as ApiKeyScope)}
          >
            <option value={ApiKeyScope.full}>{t('settings.profile.apiKeys.scopeFull')}</option>
            <option value={ApiKeyScope.read}>{t('settings.profile.apiKeys.scopeRead')}</option>
          </Select>
        </Field>
        {connectors && connectors.length > 0 && (
          <fieldset className="sm:col-span-3">
            <legend className="mb-1.5 block font-mono text-2xs text-ink-faint">
              {t('settings.profile.apiKeys.connectors')}
            </legend>
            <div className="flex flex-wrap gap-x-4 gap-y-1.5">
              {connectors.map((c) => (
                <label key={c.id} className="flex items-center gap-1.5 text-xs text-ink">
                  <input
                    type="checkbox"
                    checked={connectorIds.includes(c.id)}
                    onChange={(e) =>
                      setConnectorIds((ids) =>
                        e.target.checked ? [...ids, c.id] : ids.filter((id) => id !== c.id)
                      )
                    }
                  />
                  {c.name}
                </label>
              ))}
            </div>
            <span className="mt-1 block text-2xs leading-relaxed text-ink-faint">
              {t('settings.profile.apiKeys.connectorsHint')}
            </span>
          </fieldset>
        )}
        <div className="flex items-end">
          <Button
            type="submit"
            variant="primary"
            size="sm"
            disabled={!name.trim() || create.isPending}
          >
            {t('settings.profile.apiKeys.create')}
          </Button>
        </div>
      </form>

      <div className="mt-5 border-t border-line-soft pt-4">
        {isLoading ? (
          <SkeletonRows rows={2} className="p-0" />
        ) : isError || !data ? (
          <ErrorState
            description={t('settings.profile.apiKeys.loadError')}
            onRetry={() => refetch()}
          />
        ) : data.length === 0 ? (
          <EmptyState icon={<KeyIcon size={18} />} title={t('settings.profile.apiKeys.empty')} />
        ) : (
          <ul className="divide-y divide-line-soft">
            {data.map((k) => (
              <li key={k.id} className="flex items-center gap-4 py-3 first:pt-0 last:pb-0">
                <div className="min-w-0 flex-1">
                  <p className="flex items-center gap-2 text-sm text-ink">
                    <span className="truncate">{k.name}</span>
                    {k.scope === ApiKeyScope.read && (
                      <ToneTag tone="idle" label={t('settings.profile.apiKeys.readOnlyLabel')} />
                    )}
                    {k.revokedAt && (
                      <ToneTag tone="err" label={t('settings.profile.apiKeys.revokedLabel')} />
                    )}
                  </p>
                  {k.connectorIds.length > 0 && (
                    <p className="mt-0.5 truncate text-2xs text-ink-faint">
                      {t('settings.profile.apiKeys.limitedTo', {
                        connectors: k.connectorIds.map(connectorName).join(', '),
                      })}
                    </p>
                  )}
                  <p className="mt-0.5 flex items-center gap-1.5 font-mono text-2xs text-ink-faint">
                    <span>
                      {t('settings.profile.apiKeys.created')} <TimeAgo at={k.createdAt} />
                    </span>
                    <span>·</span>
                    <span>
                      {k.lastUsedAt ? (
                        <>
                          {t('settings.profile.apiKeys.lastUsed')} <TimeAgo at={k.lastUsedAt} />
                        </>
                      ) : (
                        t('settings.profile.apiKeys.neverUsed')
                      )}
                    </span>
                    {k.expiresAt && (
                      <>
                        <span>·</span>
                        <span>
                          {t('settings.profile.apiKeys.expires')} <TimeAgo at={k.expiresAt} />
                        </span>
                      </>
                    )}
                  </p>
                </div>
                {!k.revokedAt && (
                  <Button
                    variant="danger"
                    size="sm"
                    disabled={doRevoke.isPending}
                    onClick={() => setRevoke(k)}
                  >
                    {t('settings.profile.apiKeys.revoke')}
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </div>

      <ConfirmDialog
        open={revoke !== null}
        onClose={() => setRevoke(null)}
        onConfirm={() => revoke && doRevoke.mutate(revoke.id)}
        tone="danger"
        title={t('settings.profile.apiKeys.revokeTitle')}
        description={t('settings.profile.apiKeys.revokeConfirm')}
        confirmLabel={t('settings.profile.apiKeys.revoke')}
        cancelLabel={t('common.cancel')}
        confirmDisabled={doRevoke.isPending}
      />

      <NewApiKeyDialog token={created} onClose={() => setCreated(null)} />
    </Section>
  );
}

function NewApiKeyDialog({ token, onClose }: { token: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    if (!token) return;
    try {
      await copyText(token);
      setCopied(true);
    } catch {
      toast.error(
        t('common.copyError', {
          defaultValue: 'Could not copy. Select and copy the text manually.',
        })
      );
    }
  };

  return (
    <Dialog
      open={token !== null}
      onClose={() => {
        setCopied(false);
        onClose();
      }}
      title={t('settings.profile.apiKeys.createdTitle')}
      size="sm"
    >
      <p className="text-sm leading-relaxed text-ink-muted">
        {t('settings.profile.apiKeys.createdDesc')}
      </p>
      <div className="mt-3 flex items-center gap-2 rounded-md border border-line-soft bg-canvas-sunken p-2">
        <code className="flex-1 select-text overflow-x-auto whitespace-nowrap font-mono text-xs text-ink">
          {token}
        </code>
        <Button variant="ghost" size="sm" onClick={() => void copy()}>
          <CopyIcon size={14} />
          {copied ? t('settings.profile.apiKeys.copied') : t('settings.profile.apiKeys.copy')}
        </Button>
      </div>
      <div className="mt-5 flex justify-end">
        <Button
          variant="primary"
          size="sm"
          onClick={() => {
            setCopied(false);
            onClose();
          }}
        >
          {t('settings.profile.apiKeys.done')}
        </Button>
      </div>
    </Dialog>
  );
}
