/**
 * Reusable connector creation form: type picker → schema-driven config → create.
 * Extracted from AddConnectorPage so the onboarding stepper reuses the exact same
 * form (no throwaway onboarding-only variant). The server validates the connection
 * on create (Connector.Validate) and rejects bad credentials; surfaced inline.
 * Creating a connector is operator-gated server-side.
 *
 * Owns no page chrome — the caller supplies surrounding layout and decides what
 * happens on success via `onCreated`.
 */
import { createElement, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import {
  getGetConnectorsQueryKey,
  postConnectors,
  useGetConnectors,
  useGetConnectorsSchema,
} from '../../api/generated/connectors/connectors';
import type { Connector, ConnectorCreate, ConnectorTypeSchema, SchemaField } from '../../api/model';
import { useStepUpMutation, elevationOptions } from '../../components/manager/useStepUpMutation';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { ErrorState, SkeletonRows } from '../../components/ui/states';
import { categoryIconFor } from '../../components/categoryIcon';
import { CheckIcon } from '../../components/icons';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { CertificateExpiryPackOffer } from '../compliance/CertificateExpiryPackOffer';
import { fieldDefault, isSecretField, isTlsProbeEndpointField, isToggleField, isTopLevelField, isVerifyTlsField, tlsProbeFieldTranslations } from './schemaFields';
import { LocatedErrors } from './LocatedErrors';
import { TestRecipePanel } from './TestRecipePanel';
import {
  errorsForField,
  focusFirstLocatedError,
  locatedErrorsFrom,
  readRecipeCategory,
  recipePreviewConfig,
} from './recipeForm';

type FormValues = Record<string, string | boolean>;

export function ConnectorForm({
  onCreated,
  onCancel,
  initialType,
  initialValues,
}: {
  /** Called with the created connector after the list cache is invalidated. */
  onCreated: (created: Connector) => void;
  onCancel?: () => void;
  /** Connector type to open with already chosen; read once, so remount (key) to change it. */
  initialType?: string;
  /**
   * Field values to open with, keyed by schema field name (e.g. `url`); read once.
   * `name` and `owner` prefill the form's own name and owner inputs.
   */
  initialValues?: Record<string, string | boolean>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: schemas, isLoading, isError, refetch } = useGetConnectorsSchema();
  const { data: connectors } = useGetConnectors();
  const isInstanceAdmin = useIsInstanceAdmin();

  const [typeKey, setTypeKey] = useState<string | null>(initialType ?? null);
  const [name, setName] = useState(() => String(initialValues?.name ?? ''));
  const [owner, setOwner] = useState(() => String(initialValues?.owner ?? ''));
  const [values, setValues] = useState<FormValues>(() => {
    const fields: FormValues = { ...initialValues };
    delete fields.name;
    delete fields.owner;
    return fields;
  });
  const [createdTlsProbe, setCreatedTlsProbe] = useState<Connector | null>(null);

  const schema = useMemo(
    () => schemas?.find((s) => s.type === typeKey) ?? null,
    [schemas, typeKey],
  );

  const createBody = (): ConnectorCreate => {
    if (!schema) throw new Error('no type selected');
    // An empty url is omitted for every type; the API rejects it with a field
    // error where the type requires one, and Caddy pasted-JSON mode needs it absent.
    const url = String(values.url ?? '') || undefined;
    const tlsField = schema.fields.find(isVerifyTlsField);
    // Verify TLS is on unless the user switched it off.
    const verifyTls = tlsField ? Boolean(values[tlsField.name] ?? fieldDefault(tlsField)) : true;
    const config: Record<string, unknown> = {};
    for (const f of schema.fields) {
      if (isTopLevelField(f)) continue;
      if (!isInstanceAdmin && isTlsProbeEndpointField(schema.type, f.name)) continue;
      config[f.name] = values[f.name] ?? fieldDefault(f);
    }
    // A custom recipe determines its category, so omit the schema default.
    const derivesCategory = String(config.recipe ?? '').trim() !== '';
    return {
      name,
      owner: owner || undefined,
      ...(derivesCategory ? {} : { category: schema.category }),
      type: schema.type,
      ...(url ? { url } : {}),
      verifyTls,
      config,
    };
  };
  const create = useStepUpMutation<ConnectorCreate, Connector>({
    action: 'connector.recipeActions',
    mutationFn: (body, token) => token ? postConnectors(body, elevationOptions(token)) : postConnectors(body),
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
      if (schema?.type === 'tlsprobe') setCreatedTlsProbe(created as Connector);
      else onCreated(created as Connector);
    },
  });

  const requiredFilled =
    !!name &&
    !!schema &&
    schema.fields.every((f) => !f.required || String(values[f.name] ?? '').length > 0);
  const createErrors = create.isError ? locatedErrorsFrom(create.error) : [];
  const tlsField = schema?.fields.find(isVerifyTlsField);
  const previewVerifyTls = tlsField ? Boolean(values[tlsField.name] ?? fieldDefault(tlsField)) : true;
  useEffect(() => {
    if (!create.isError) return;
    const errors = locatedErrorsFrom(create.error);
    if (errors.length) focusFirstLocatedError(errors, schema?.type === 'custom' ? 'recipe' : undefined);
  }, [create.isError, create.error, schema?.type]);

  if (isLoading) {
    return (
      <Panel className="p-6">
        <SkeletonRows rows={5} />
      </Panel>
    );
  }
  if (isError || !schemas) {
    return (
      <Panel className="min-h-[30vh]">
        <ErrorState description={t('connectors.loadTypesError')} onRetry={() => refetch()} />
      </Panel>
    );
  }

  return (
    <div className="space-y-4">
      {/* Step 1 — type */}
      <Panel className="p-5">
        <p className="mb-3 text-2xs text-ink-faint">{t('connectors.typeLabel')}</p>
        <div className="grid gap-2 sm:grid-cols-3">
          {schemas.map((s) => (
            <TypeCard
              key={s.type}
              schema={s}
              active={typeKey === s.type}
              onPick={() => {
                setTypeKey(s.type);
                setValues({});
              }}
            />
          ))}
        </div>
      </Panel>

      {/* Step 2 — config */}
      {schema && (
        <Panel className="p-5">
          <p className="mb-3 text-2xs text-ink-faint">
            {t('connectors.configLabel', { name: schema.displayName })}
          </p>
          <div className="space-y-3">
            <Field
              field={{
                name: 'name',
                label: t('connectors.displayName'),
                kind: 'string',
                required: true,
                placeholder: t('connectors.displayNamePlaceholder'),
              }}
              value={name}
              onChange={(v) => setName(String(v))}
            />
            <Field
              field={{ name: 'owner', label: t('connectors.owner'), kind: 'string', required: false }}
              value={owner}
              onChange={(v) => setOwner(String(v))}
            />
            {schema.fields.map((f) => {
              const probeTranslations = schema.type === 'tlsprobe' ? tlsProbeFieldTranslations[f.name] : undefined;
              const disabled = !isInstanceAdmin && isTlsProbeEndpointField(schema.type, f.name);
              return <div key={f.name}>
                <Field
                  field={probeTranslations ? { ...f, label: t(probeTranslations.label) } : f}
                  value={values[f.name] ?? fieldDefault(f)}
                  error={errorsForField(createErrors, f.name)}
                  disabled={disabled}
                  helperText={disabled ? t('connectors.tlsProbe.adminOnlyHint') : probeTranslations ? t(probeTranslations.hint) : undefined}
                  selectOptions={schema.type === 'tlsprobe' && f.name === 'import_connector_id'
                    ? [
                        { value: '', label: t('connectors.tlsProbe.noTraefikImport') },
                        ...(connectors ?? []).filter((connector) => connector.type === 'traefik').map((connector) => ({ value: connector.id, label: connector.name })),
                      ]
                    : undefined}
                  onChange={(v) => setValues((s) => ({ ...s, [f.name]: v }))}
                />
                {schema.type === 'custom' && f.name === 'recipe' && (
                  <RecipeCategoryDisplay recipe={String(values[f.name] ?? '')} />
                )}
              </div>;
            })}
          </div>

          {createErrors.length > 0 && (
            <div className="mt-3">
              <LocatedErrors title={t('connectors.recipePreview.validationTitle')} errors={createErrors} />
            </div>
          )}
          {create.isError && createErrors.length === 0 && (
            <p className="mt-3 text-2xs text-err" role="alert">
              {t('connectors.connectFailed')}
            </p>
          )}

          {schema.type === 'custom' && (
            <div className="mt-5">
              <TestRecipePanel
                url={String(values.url ?? '')}
                verifyTls={previewVerifyTls}
                config={recipePreviewConfig(schema, values)}
              />
            </div>
          )}

          <div className="mt-5 flex items-center justify-end gap-2">
            {onCancel && (
              <Button variant="ghost" size="md" onClick={onCancel}>
                {t('common.cancel')}
              </Button>
            )}
            <Button
              variant="primary"
              size="md"
              disabled={!requiredFilled || create.isPending || createdTlsProbe !== null}
              onClick={() => create.mutate(createBody())}
            >
              <CheckIcon size={15} />
              {create.isPending ? t('connectors.submitPending') : t('connectors.submitIdle')}
            </Button>
          </div>
        </Panel>
      )}
      {create.dialog}
      <CertificateExpiryPackOffer
        requested={createdTlsProbe !== null}
        onFinished={() => {
          if (!createdTlsProbe) return;
          const created = createdTlsProbe;
          setCreatedTlsProbe(null);
          onCreated(created);
        }}
      />
    </div>
  );
}

function TypeCard({
  schema,
  active,
  onPick,
}: {
  schema: ConnectorTypeSchema;
  active: boolean;
  onPick: () => void;
}) {
  const { t } = useTranslation();
  return (
    <button
      onClick={onPick}
      disabled={schema.stub}
      aria-pressed={active}
      className={
        'flex items-center gap-2.5 rounded-lg border p-3 text-left transition-colors disabled:pointer-events-none disabled:opacity-40 ' +
        (active ? 'border-accent-primary-soft bg-accent-primary-tint' : 'border-line-soft hover:border-line-strong')
      }
    >
      <span className="flex h-8 w-8 items-center justify-center rounded-md bg-canvas-sunken text-ink-faint">
        {createElement(categoryIconFor(schema.category), { size: 16 })}
      </span>
      <span className="text-sm font-medium text-ink">{schema.displayName}</span>
      {schema.stub && (
        <span className="ml-auto rounded-sm bg-warn-tint px-1.5 py-0.5 font-mono text-2xs text-warn">
          {t('connectors.comingSoon')}
        </span>
      )}
    </button>
  );
}

export function Field({
  field,
  value,
  onChange,
  error,
  disabled = false,
  helperText,
  selectOptions,
}: {
  field: SchemaField;
  value: string | boolean;
  onChange: (v: string | boolean) => void;
  error?: string;
  disabled?: boolean;
  helperText?: string;
  selectOptions?: { value: string; label: string; disabled?: boolean }[];
}) {
  const fieldId = `connector-field-${field.name}`;
  const errorId = error ? `${fieldId}-error` : undefined;
  const helperId = helperText ? `${fieldId}-hint` : undefined;
  const describedBy = [helperId, errorId].filter(Boolean).join(' ') || undefined;
  if (isToggleField(field)) {
    return (
      <label className="flex items-center justify-between gap-3">
        <span className="text-sm text-ink">{field.label}</span>
        <button
          type="button"
          role="switch"
          aria-label={field.label}
          aria-checked={!!value}
          aria-describedby={describedBy}
          disabled={disabled}
          onClick={() => onChange(!value)}
          className="relative h-5 w-9 rounded-full transition-colors disabled:opacity-50"
          style={{ backgroundColor: value ? 'var(--color-accent-primary)' : 'var(--color-line-strong)' }}
        >
          <span
            className="absolute top-0.5 h-4 w-4 rounded-full bg-ink transition-[left]"
            style={{ left: value ? '18px' : '2px' }}
          />
        </button>
      </label>
    );
  }
  if (selectOptions || (field.kind === 'select' && field.options && field.options.length > 0)) {
    const options: { value: string; label: string; disabled?: boolean }[] =
      selectOptions ?? (field.options ?? []).map((option) => ({ value: option, label: option }));
    return (
      <div>
        <label className="block" htmlFor={fieldId}>
          <span className="mb-1 block text-2xs text-ink-faint">
            {field.label}
            {field.required && <span className="text-err"> *</span>}
          </span>
          <select
            id={fieldId}
            name={field.name}
            value={String(value)}
            disabled={disabled}
            aria-invalid={!!error}
            aria-describedby={describedBy}
            onChange={(e) => onChange(e.target.value)}
            className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none focus-visible:border-accent-primary-soft disabled:opacity-50"
          >
            {!selectOptions && !field.required && <option value="" />}
            {options.map((option) => (
              <option key={option.value} value={option.value} disabled={option.disabled}>{option.label}</option>
            ))}
          </select>
        </label>
        {helperText && <p id={helperId} className="mt-1 text-2xs text-ink-faint">{helperText}</p>}
        {error && <p id={errorId} className="mt-1 text-2xs text-err" aria-live="polite">{error}</p>}
      </div>
    );
  }
  if (field.kind === 'secret' || field.kind === 'textarea') {
    return (
      <div>
        <label className="block">
          <span className="mb-1 block text-2xs text-ink-faint">
            {field.label}
            {field.required && <span className="text-err"> *</span>}
          </span>
          <textarea
            id={fieldId}
            name={field.name}
            value={String(value)}
            placeholder={field.placeholder}
            rows={field.kind === 'textarea' ? 12 : 4}
            autoComplete="off"
            spellCheck={false}
            aria-invalid={!!error}
            aria-describedby={describedBy}
            disabled={disabled}
            onChange={(e) => onChange(e.target.value)}
            className={`w-full rounded-sm border border-line bg-surface px-2.5 py-2 font-mono text-xs text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft ${field.kind === 'textarea' ? 'resize-y' : ''}`}
          />
        </label>
        {helperText && <p id={helperId} className="mt-1 text-2xs text-ink-faint">{helperText}</p>}
        {error && <p id={errorId} className="mt-1 text-2xs text-err" aria-live="polite">{error}</p>}
      </div>
    );
  }
  const type =
    isSecretField(field)
      ? 'password'
      : field.kind === 'number'
        ? 'number'
        : 'text';
  return (
    <div>
      <label className="block">
        <span className="mb-1 block text-2xs text-ink-faint">
          {field.label}
          {field.required && <span className="text-err"> *</span>}
        </span>
        <input
          id={fieldId}
          name={field.name}
          type={type}
          value={String(value)}
          placeholder={field.placeholder}
          autoComplete={type === 'password' ? 'new-password' : 'off'}
          aria-invalid={!!error}
          aria-describedby={describedBy}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value)}
          className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft"
        />
      </label>
      {helperText && <p id={helperId} className="mt-1 text-2xs text-ink-faint">{helperText}</p>}
      {error && <p id={errorId} className="mt-1 text-2xs text-err" aria-live="polite">{error}</p>}
    </div>
  );
}

export function RecipeCategoryDisplay({ recipe }: { recipe: string }) {
  const { t } = useTranslation();
  const category = readRecipeCategory(recipe);
  return (
    <dl className="mt-2 flex items-baseline gap-2 text-xs">
      <dt className="text-ink-muted">{t('connectors.recipePreview.categoryLabel')}</dt>
      <dd className="text-ink" aria-live="polite">
        {category ? t(`services.category.${category}`) : t('connectors.recipePreview.categoryUnset')}
      </dd>
    </dl>
  );
}
