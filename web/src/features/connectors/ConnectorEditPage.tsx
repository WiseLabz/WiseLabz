/**
 * Edit an existing connector (`/connectors/:id/edit`). The type is fixed; the
 * schema-driven fields are reused from the add flow (<Field/>), prefilled from the
 * connector read. Credentials are write-only over the API — secret fields come back
 * empty and are only sent when re-entered (credential rotation). Re-test and save
 * are operator actions, enforced server-side.
 */
import { useEffect, useMemo, useState } from 'react';
import { isAxiosError } from 'axios';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetConnectorsConnectorId,
  useGetConnectors,
  useGetConnectorsSchema,
  putConnectorsConnectorId,
  postConnectorsConnectorIdTest,
  getGetConnectorsQueryKey,
} from '../../api/generated/connectors/connectors';
import type { Connector, ConnectorUpdate, SchemaField } from '../../api/model';
import { Field, RecipeCategoryDisplay } from './ConnectorForm';
import { ConnectorPermissionsTab } from './ConnectorPermissionsTab';
import { useStepUpMutation, elevationOptions } from '../../components/manager/useStepUpMutation';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { SkeletonRows, ErrorState } from '../../components/ui/states';
import { toast } from '../../lib/toast';
import { ArrowRightIcon, CheckIcon } from '../../components/icons';
import { useConnectorRole, useIsInstanceAdmin } from '../../hooks/useRole';
import { fieldDefault, isSecretField, isTlsProbeEndpointField, isToggleField, isTopLevelField, isVerifyTlsField, tlsProbeFieldTranslations } from './schemaFields';
import { LocatedErrors } from './LocatedErrors';
import { TestRecipePanel } from './TestRecipePanel';
import { errorsForField, focusFirstLocatedError, locatedErrorsFrom, recipePreviewConfig } from './recipeForm';

type FormValues = Record<string, string | boolean>;

/** Whole days between an RFC3339 timestamp and now, floored at 0. */
function daysAgo(iso: string): number {
  const ms = Date.now() - new Date(iso).getTime();
  return Math.max(0, Math.floor(ms / (24 * 60 * 60 * 1000)));
}

function editFieldValue(field: SchemaField, connector: Connector, values: FormValues): string | boolean {
  if (values[field.name] !== undefined) return values[field.name];
  if (field.name === 'url') return connector.url ?? '';
  if (isVerifyTlsField(field)) return Boolean(connector.verifyTls);
  if (isTlsProbeEndpointField(connector.type, field.name)) {
    return String(connector.config?.[field.name] ?? (field.name === 'import_port' ? fieldDefault(field) : ''));
  }
  if (isToggleField(field)) return false;
  if (field.kind === 'textarea') return connector.config?.[field.name] ?? '';
  return '';
}

export function ConnectorEditPage() {
  const { t } = useTranslation();
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const connector = useGetConnectorsConnectorId(id);
  const { data: connectors } = useGetConnectors();
  const { data: schemas } = useGetConnectorsSchema();
  const canEdit = useConnectorRole(id) === 'operator';
  const isInstanceAdmin = useIsInstanceAdmin();

  const schema = useMemo(
    () => schemas?.find((s) => s.type === connector.data?.type) ?? null,
    [schemas, connector.data?.type],
  );

  const [name, setName] = useState<string | null>(null);
  const [owner, setOwner] = useState<string | null>(null);
  const [values, setValues] = useState<FormValues>({});
  const [userExpiresAt, setUserExpiresAt] = useState<string | null>(null);
  const [rotationMaxAgeDays, setRotationMaxAgeDays] = useState<string | null>(null);
  // Prefilled values fall back to the connector read until the user edits a field.
  const nameValue = name ?? connector.data?.name ?? '';
  const ownerValue = owner ?? connector.data?.owner ?? '';
  const userExpiresAtValue = userExpiresAt ?? connector.data?.userExpiresAt?.slice(0, 10) ?? '';
  const rotationMaxAgeDaysValue =
    rotationMaxAgeDays ?? (connector.data?.rotationMaxAgeDays != null ? String(connector.data.rotationMaxAgeDays) : '');

  const test = useMutation({
    mutationFn: () => postConnectorsConnectorIdTest(id),
    onSuccess: (r) => toast.success(t('connectors.edit.testOk', { ms: r.latencyMs ?? 0 })),
    onError: () => toast.error(t('connectors.edit.testFail')),
  });

  const saveBody = (): ConnectorUpdate => {
    const config: Record<string, unknown> = {};
    const tlsField = schema?.fields.find(isVerifyTlsField);
    for (const f of schema?.fields ?? []) {
      if (isTopLevelField(f)) continue;
      if (isTlsProbeEndpointField(connector.data?.type ?? '', f.name)) {
        if (isInstanceAdmin && values[f.name] !== undefined) config[f.name] = values[f.name];
        continue;
      }
      if (f.kind === 'textarea') {
        const value = values[f.name] ?? connector.data?.config?.[f.name];
        if (value !== undefined) config[f.name] = value;
        continue;
      }
      // Only send secret/config fields the user actually re-entered.
      if (values[f.name] !== undefined && String(values[f.name]).length > 0) config[f.name] = values[f.name];
    }
    // Moving an optional-url type (Caddy) from pasted-JSON mode to url mode:
    // the stored pasted blob is write-only and omitted fields are kept
    // server-side, so clear it explicitly or both inputs would be set.
    const urlField = schema?.fields.find((f) => f.name === 'url');
    const newUrl = values.url !== undefined ? String(values.url) : (connector.data?.url ?? '');
    if (urlField && !urlField.required && !connector.data?.url && newUrl) {
      for (const f of schema?.fields ?? []) {
        if (f.kind === 'secret' && config[f.name] === undefined) config[f.name] = '';
      }
    }
    return {
      name: nameValue,
      owner: ownerValue,
      url: values.url !== undefined ? String(values.url) : connector.data?.url,
      verifyTls: tlsField && values[tlsField.name] !== undefined ? Boolean(values[tlsField.name]) : connector.data?.verifyTls,
      config,
      ...(userExpiresAt !== null && {
        userExpiresAt: userExpiresAt ? `${userExpiresAt}T00:00:00Z` : null,
      }),
      ...(rotationMaxAgeDays !== null && {
        rotationMaxAgeDays: rotationMaxAgeDays ? Number(rotationMaxAgeDays) : null,
      }),
    };
  };
  const save = useStepUpMutation<ConnectorUpdate, Connector>({
    action: 'connector.recipeActions',
    target: () => id,
    mutationFn: (body, token) => token
      ? putConnectorsConnectorId(id, body, elevationOptions(token))
      : putConnectorsConnectorId(id, body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
      toast.success(t('connectors.edit.saved'));
      navigate(`/services/${id}`);
    },
    onError: (error) => {
      const details = isAxiosError(error)
        ? (error.response?.data as { details?: { field: string; msg: string }[] } | undefined)?.details
        : undefined;
      const fieldMsg = details?.map((d) => `${d.field}: ${d.msg}`).join('; ');
      toast.error(fieldMsg ? `${t('connectors.edit.saveError')} ${fieldMsg}` : t('connectors.edit.saveError'));
    },
  });
  const saveErrors = save.isError ? locatedErrorsFrom(save.error) : [];
  const storedRecipe = String(connector.data?.config?.recipe ?? '');
  const recipeDirty = values.recipe !== undefined && String(values.recipe) !== storedRecipe;
  useEffect(() => {
    if (save.isError) {
      const errors = locatedErrorsFrom(save.error);
      if (errors.length) focusFirstLocatedError(errors, schema?.type === 'custom' ? 'recipe' : undefined);
    }
  }, [save.isError, save.error, schema?.type]);
  useEffect(() => {
    if (!recipeDirty) return;
    const warnBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = '';
    };
    window.addEventListener('beforeunload', warnBeforeUnload);
    return () => window.removeEventListener('beforeunload', warnBeforeUnload);
  }, [recipeDirty]);

  if (connector.isLoading) {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="p-6">
          <SkeletonRows rows={5} />
        </Panel>
      </div>
    );
  }
  if (connector.isError || !connector.data) {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="min-h-[30vh]">
          <ErrorState description={t('services.detail.notFound')} onRetry={() => connector.refetch()} />
        </Panel>
      </div>
    );
  }

  const c = connector.data;

  if (c.managedBy === 'config' || c.managedBy === 'config-orphaned') {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="min-h-[30vh]">
          <ErrorState
            title={t('connectors.managed.editTitle')}
            description={t(
              c.managedBy === 'config' ? 'connectors.managed.editDesc' : 'connectors.managed.orphanedEditDesc',
            )}
          />
        </Panel>
      </div>
    );
  }

  if (!canEdit) {
    return (
      <div className="mx-auto max-w-170 px-6 py-6">
        <Panel className="min-h-[30vh]">
          <ErrorState
            title={t('connectors.edit.noAccessTitle')}
            description={t('connectors.edit.noAccessDesc')}
          />
        </Panel>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-170 px-6 py-6">
      <button
        onClick={() => navigate(`/services/${id}`)}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('connectors.edit.back')}
      </button>

      <header className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-ink">{t('connectors.edit.title', { name: c.name })}</h1>
        <p className="text-sm text-ink-muted">{t('connectors.edit.subtitle')}</p>
      </header>

      <Panel className="p-5">
        <div className="space-y-3">
          <Field
            field={{ name: 'name', label: t('connectors.displayName'), kind: 'string', required: true }}
            value={nameValue}
            onChange={(v) => setName(String(v))}
          />
          <Field
            field={{ name: 'owner', label: t('connectors.owner'), kind: 'string', required: false }}
            value={ownerValue}
            onChange={(v) => setOwner(String(v))}
          />
          {schema?.fields.map((f) => {
            const probeTranslations = c.type === 'tlsprobe' ? tlsProbeFieldTranslations[f.name] : undefined;
            const disabled = !isInstanceAdmin && isTlsProbeEndpointField(c.type, f.name);
            const selectedImportId = String(c.config?.import_connector_id ?? '');
            const visibleTraefik = (connectors ?? []).filter((item) => item.type === 'traefik');
            const selectedImportAvailable = visibleTraefik.some((item) => item.id === selectedImportId);
            return <div key={f.name}>
              <Field
                field={probeTranslations
                  ? { ...f, label: t(probeTranslations.label) }
                  : isSecretField(f) ? { ...f, placeholder: t('connectors.edit.secretPlaceholder') } : f}
                value={editFieldValue(f, c, values)}
                error={errorsForField(saveErrors, f.name)}
                disabled={disabled}
                helperText={disabled ? t('connectors.tlsProbe.adminOnlyHint') : probeTranslations ? t(probeTranslations.hint) : undefined}
                selectOptions={isInstanceAdmin && c.type === 'tlsprobe' && f.name === 'import_connector_id'
                  ? [
                      { value: '', label: t('connectors.tlsProbe.noTraefikImport') },
                      ...(selectedImportId && !selectedImportAvailable
                        ? [{ value: selectedImportId, label: t('connectors.tlsProbe.currentImportUnavailable', { id: selectedImportId }), disabled: true }]
                        : []),
                      ...visibleTraefik.map((item) => ({ value: item.id, label: item.name })),
                    ]
                  : undefined}
                onChange={(v) => setValues((s) => ({ ...s, [f.name]: v }))}
              />
              {schema?.type === 'custom' && f.name === 'recipe' && (
                <RecipeCategoryDisplay recipe={String(values[f.name] ?? c.config?.[f.name] ?? '')} />
              )}
            </div>;
          })}
          {!schema?.isCredentialRefresher && (
            <>
              {c.secretRotatedAt && (
                <p className="text-xs text-ink-muted">
                  {t('connectors.edit.rotationLastRotated', {
                    count: daysAgo(c.secretRotatedAt),
                    days: daysAgo(c.secretRotatedAt),
                  })}
                </p>
              )}
              <label className="block text-xs text-ink-muted">
                {t('connectors.edit.rotationExpiryLabel')}
                <input
                  type="date"
                  value={userExpiresAtValue}
                  onChange={(e) => setUserExpiresAt(e.target.value)}
                  className="mt-1 block h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none focus-visible:border-accent-primary-soft"
                />
              </label>
              <label className="block text-xs text-ink-muted">
                {t('connectors.edit.rotationMaxAgeLabel')}
                <input
                  type="number"
                  min={1}
                  value={rotationMaxAgeDaysValue}
                  onChange={(e) => setRotationMaxAgeDays(e.target.value)}
                  className="mt-1 block h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none focus-visible:border-accent-primary-soft"
                />
              </label>
            </>
          )}
        </div>

        {saveErrors.length > 0 && (
          <div className="mt-3">
            <LocatedErrors title={t('connectors.recipePreview.validationTitle')} errors={saveErrors} />
          </div>
        )}

        <div className="mt-5 flex items-center justify-between gap-2">
          <Button
            variant="secondary"
            size="md"
            onClick={() => test.mutate()}
            disabled={test.isPending || schema?.stub}
            title={schema?.stub ? t('connectors.edit.testUnavailable') : undefined}
          >
            {test.isPending ? t('connectors.edit.testing') : t('connectors.edit.test')}
          </Button>
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="md" onClick={() => navigate(`/services/${id}`)}>
              {t('common.cancel')}
            </Button>
            <Button variant="primary" size="md" onClick={() => save.mutate(saveBody())} disabled={!nameValue || save.isPending}>
              <CheckIcon size={15} />
              {save.isPending ? t('connectors.edit.saving') : t('common.save')}
            </Button>
          </div>
        </div>
      </Panel>

      {schema?.type === 'custom' && (
        <div className="mt-4">
          <TestRecipePanel
            connectorId={id}
            url={String(values.url ?? c.url ?? '')}
            verifyTls={Boolean(values[schema.fields.find(isVerifyTlsField)?.name ?? 'verify_tls'] ?? c.verifyTls ?? true)}
            config={recipePreviewConfig(schema, values, c.config ?? {})}
          />
        </div>
      )}

      {save.dialog}
      <ConnectorPermissionsTab connectorId={id} />
    </div>
  );
}
