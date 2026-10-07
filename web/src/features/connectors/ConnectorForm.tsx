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
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getGetConnectorsQueryKey,
  postConnectors,
  useGetConnectorsSchema,
} from '../../api/generated/connectors/connectors';
import type { Connector, ConnectorTypeSchema, SchemaField } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { ErrorState, SkeletonRows } from '../../components/ui/states';
import { categoryIconFor } from '../../components/categoryIcon';
import { CheckIcon } from '../../components/icons';
import { fieldDefault, isSecretField, isToggleField, isTopLevelField, isVerifyTlsField } from './schemaFields';
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
}: {
  /** Called with the created connector after the list cache is invalidated. */
  onCreated: (created: Connector) => void;
  onCancel?: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data: schemas, isLoading, isError, refetch } = useGetConnectorsSchema();

  const [typeKey, setTypeKey] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [owner, setOwner] = useState('');
  const [values, setValues] = useState<FormValues>({});

  const schema = useMemo(
    () => schemas?.find((s) => s.type === typeKey) ?? null,
    [schemas, typeKey],
  );

  const create = useMutation({
    mutationFn: () => {
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
        config[f.name] = values[f.name] ?? fieldDefault(f);
      }
      // A custom recipe determines its category, so omit the schema default.
      const derivesCategory = String(config.recipe ?? '').trim() !== '';
      return postConnectors({
        name,
        owner: owner || undefined,
        ...(derivesCategory ? {} : { category: schema.category }),
        type: schema.type,
        ...(url ? { url } : {}),
        verifyTls,
        config,
      });
    },
    onSuccess: (created) => {
      queryClient.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
      onCreated(created as Connector);
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
            {schema.fields.map((f) => (
              <div key={f.name}>
                <Field
                  field={f}
                  value={values[f.name] ?? fieldDefault(f)}
                  error={errorsForField(createErrors, f.name)}
                  onChange={(v) => setValues((s) => ({ ...s, [f.name]: v }))}
                />
                {schema.type === 'custom' && f.name === 'recipe' && (
                  <RecipeCategoryDisplay recipe={String(values[f.name] ?? '')} />
                )}
              </div>
            ))}
          </div>

          {createErrors.length > 0 && (
            <div className="mt-3">
              <LocatedErrors title={t('connectors.recipePreview.validationTitle')} errors={createErrors} />
            </div>
          )}
          {create.isError && createErrors.length === 0 && (
            <p className="mt-3 text-2xs text-err" role="alert" aria-live="polite">
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
              disabled={!requiredFilled || create.isPending}
              onClick={() => create.mutate()}
            >
              <CheckIcon size={15} />
              {create.isPending ? t('connectors.submitPending') : t('connectors.submitIdle')}
            </Button>
          </div>
        </Panel>
      )}
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
}: {
  field: SchemaField;
  value: string | boolean;
  onChange: (v: string | boolean) => void;
  error?: string;
}) {
  const fieldId = `connector-field-${field.name}`;
  const errorId = error ? `${fieldId}-error` : undefined;
  if (isToggleField(field)) {
    return (
      <label className="flex items-center justify-between gap-3">
        <span className="text-sm text-ink">{field.label}</span>
        <button
          type="button"
          role="switch"
          aria-label={field.label}
          aria-checked={!!value}
          onClick={() => onChange(!value)}
          className="relative h-5 w-9 rounded-full transition-colors"
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
            aria-describedby={errorId}
            onChange={(e) => onChange(e.target.value)}
            className={`w-full rounded-sm border border-line bg-surface px-2.5 py-2 font-mono text-xs text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft ${field.kind === 'textarea' ? 'resize-y' : ''}`}
          />
        </label>
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
          aria-describedby={errorId}
          onChange={(e) => onChange(e.target.value)}
          className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft"
        />
      </label>
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
