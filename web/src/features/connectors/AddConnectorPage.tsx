/**
 * Add-connector flow — a dedicated surface (not a row action), per
 * ARCHITECTURE.md. Pick a connector type, fill its schema-driven config (each
 * type exposes different fields; secrets render as write-only password inputs),
 * then create. The server validates the connection on create (Connector.Validate)
 * and rejects bad credentials; we surface that inline. Creating a connector is a
 * mutating action gated on the operator role server-side.
 */
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getGetConnectorsQueryKey,
  postConnectors,
  useGetConnectorsSchema,
} from '../../api/generated/connectors/connectors';
import type { ConnectorTypeSchema, SchemaField } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { ErrorState, SkeletonRows } from '../../components/ui/states';
import { categoryIcon } from '../../components/categoryIcon';
import { ArrowRightIcon, CheckIcon } from '../../components/icons';

type FormValues = Record<string, string | boolean>;

export function AddConnectorPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { data: schemas, isLoading, isError, refetch } = useGetConnectorsSchema();

  const [typeKey, setTypeKey] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [values, setValues] = useState<FormValues>({});

  const schema = useMemo(
    () => schemas?.find((s) => s.type === typeKey) ?? null,
    [schemas, typeKey]
  );

  const create = useMutation({
    mutationFn: () => {
      if (!schema) throw new Error('no type selected');
      const url = String(values.url ?? '');
      const verifyTls = Boolean(values.verifyTls ?? false);
      // Everything except the top-level fields goes into the schema-driven config.
      const config: Record<string, unknown> = {};
      for (const f of schema.fields) {
        if (f.name === 'url' || f.name === 'verifyTls') continue;
        config[f.name] = values[f.name] ?? '';
      }
      return postConnectors({
        name,
        category: schema.category,
        type: schema.type,
        url,
        verifyTls,
        config,
      });
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: getGetConnectorsQueryKey() });
      navigate('/services');
    },
  });

  const requiredFilled =
    !!name &&
    !!schema &&
    schema.fields.every((f) => !f.required || String(values[f.name] ?? '').length > 0);

  return (
    <div className="mx-auto max-w-170 px-6 py-6">
      <button
        onClick={() => navigate('/services')}
        className="mb-4 inline-flex items-center gap-1.5 text-xs text-ink-muted transition-colors hover:text-ink"
      >
        <ArrowRightIcon size={13} className="rotate-180" />
        {t('connectors.back')}
      </button>

      <header className="mb-5">
        <h1 className="text-xl font-semibold tracking-tight text-ink">{t('connectors.title')}</h1>
        <p className="text-sm text-ink-muted">
          {t('connectors.subtitle')}
        </p>
      </header>

      {isLoading ? (
        <Panel className="p-6">
          <SkeletonRows rows={5} />
        </Panel>
      ) : isError || !schemas ? (
        <Panel className="min-h-[30vh]">
          <ErrorState description={t('connectors.loadTypesError')} onRetry={() => refetch()} />
        </Panel>
      ) : (
        <div className="space-y-4">
          {/* Step 1 — type */}
          <Panel className="p-5">
            <p className="mb-3 text-2xs uppercase tracking-wider text-ink-faint">{t('connectors.typeLabel')}</p>
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
              <p className="mb-3 text-2xs uppercase tracking-wider text-ink-faint">
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
                {schema.fields.map((f) => (
                  <Field
                    key={f.name}
                    field={f}
                    value={values[f.name] ?? (f.kind === 'boolean' ? false : '')}
                    onChange={(v) => setValues((s) => ({ ...s, [f.name]: v }))}
                  />
                ))}
              </div>

              {create.isError && (
                <p className="mt-3 text-2xs text-err">
                  {t('connectors.connectFailed')}
                </p>
              )}

              <div className="mt-5 flex items-center justify-end gap-2">
                <Button variant="ghost" size="md" onClick={() => navigate('/services')}>
                  {t('common.cancel')}
                </Button>
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
  const Icon = categoryIcon[schema.category];
  return (
    <button
      onClick={onPick}
      aria-pressed={active}
      className={
        'flex items-center gap-2.5 rounded-lg border p-3 text-left transition-colors ' +
        (active ? 'border-signal-soft bg-signal-tint' : 'border-line-soft hover:border-line-strong')
      }
    >
      <span className="flex h-8 w-8 items-center justify-center rounded-md bg-canvas-sunken text-ink-faint">
        <Icon size={16} />
      </span>
      <span className="text-sm font-medium text-ink">{schema.displayName}</span>
    </button>
  );
}

function Field({
  field,
  value,
  onChange,
}: {
  field: SchemaField;
  value: string | boolean;
  onChange: (v: string | boolean) => void;
}) {
  if (field.kind === 'boolean') {
    return (
      <label className="flex items-center justify-between gap-3">
        <span className="text-sm text-ink">{field.label}</span>
        <button
          type="button"
          role="switch"
          aria-checked={!!value}
          onClick={() => onChange(!value)}
          className="relative h-5 w-9 rounded-full transition-colors"
          style={{ backgroundColor: value ? 'var(--color-signal)' : 'var(--color-line-strong)' }}
        >
          <span
            className="absolute top-0.5 h-4 w-4 rounded-full bg-white transition-[left]"
            style={{ left: value ? '18px' : '2px' }}
          />
        </button>
      </label>
    );
  }
  const type =
    field.kind === 'password' || field.secret
      ? 'password'
      : field.kind === 'number'
        ? 'number'
        : 'text';
  return (
    <label className="block">
      <span className="mb-1 block text-2xs uppercase tracking-wider text-ink-faint">
        {field.label}
        {field.required && <span className="text-err"> *</span>}
      </span>
      <input
        type={type}
        value={String(value)}
        placeholder={field.placeholder}
        autoComplete={type === 'password' ? 'new-password' : 'off'}
        onChange={(e) => onChange(e.target.value)}
        className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none placeholder:text-ink-faint focus-visible:border-signal-soft"
      />
    </label>
  );
}
