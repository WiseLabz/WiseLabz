import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePreviewConnectorRecipe } from '../../api/generated/connectors/connectors';
import type { RecipePreview, RecipePreviewInput, SnapshotDependency, SnapshotEntity } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { LocatedErrors } from './LocatedErrors';
import { focusFirstLocatedError, locatedErrorsFrom, type RecipeFeedbackEntry } from './recipeForm';

type PreviewFeedback = Omit<RecipeFeedbackEntry, 'seq'>;

// Non-reversible 32-bit FNV-1a fingerprint, so panel state never holds typed credentials.
function fingerprint(value: string): string {
  let hash = 0x811c9dc5;
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 0x01000193);
  }
  return (hash >>> 0).toString(16);
}

export function TestRecipePanel({ onFeedback, ...input }: RecipePreviewInput & { onFeedback?: (feedback: PreviewFeedback) => void }) {
  const isAdmin = useIsInstanceAdmin();
  return isAdmin ? <AdminTestRecipePanel input={input} onFeedback={onFeedback} /> : null;
}

function AdminTestRecipePanel({ input, onFeedback }: { input: RecipePreviewInput; onFeedback?: (feedback: PreviewFeedback) => void }) {
  const { t } = useTranslation();
  const preview = usePreviewConnectorRecipe({ mutation: { gcTime: 0 } });
  const recipe = String(input.config.recipe ?? '').trim();
  const inputKey = fingerprint(JSON.stringify(input));
  const [submittedKey, setSubmittedKey] = useState<string | null>(null);
  const [completedKey, setCompletedKey] = useState<string | null>(null);
  const canPreview = Boolean(input.url.trim() && recipe);

  const hasCurrentResult = submittedKey === inputKey && completedKey === inputKey;
  const validationErrors = hasCurrentResult && preview.isError ? locatedErrorsFrom(preview.error) : [];
  useEffect(() => {
    if (!hasCurrentResult || !preview.isError) return;
    const errors = locatedErrorsFrom(preview.error);
    if (errors.length) focusFirstLocatedError(errors, 'recipe');
  }, [hasCurrentResult, preview.isError, preview.error]);

  return (
    <section aria-labelledby="test-recipe-title">
      <Panel className="p-5">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 id="test-recipe-title" className="text-sm font-semibold text-ink">
              {t('connectors.recipePreview.title')}
            </h2>
            <p className="mt-1 text-xs text-ink-muted">{t('connectors.recipePreview.description')}</p>
            {input.connectorId && (
              <p className="mt-1 text-xs text-ink-muted">{t('connectors.recipePreview.storedCredentialsHint')}</p>
            )}
          </div>
          <Button
            variant="secondary"
            size="md"
            disabled={!canPreview || preview.isPending}
            onClick={() => {
              setSubmittedKey(inputKey);
              setCompletedKey(null);
              preview.mutate(
                { data: input },
                {
                  onSuccess: (result) => {
                    setCompletedKey(inputKey);
                    onFeedback?.({ recipe: String(input.config.recipe ?? ''), errors: [], endpoints: result.endpoints });
                  },
                  onError: (error) => {
                    setCompletedKey(inputKey);
                    onFeedback?.({ recipe: String(input.config.recipe ?? ''), errors: locatedErrorsFrom(error) });
                  },
                },
              );
            }}
          >
            {preview.isPending ? t('connectors.recipePreview.testing') : t('connectors.recipePreview.test')}
          </Button>
        </div>

        {preview.isPending && (
          <p className="mt-4 text-xs text-ink-muted" aria-live="polite">
            {t('connectors.recipePreview.testing')}
          </p>
        )}
        {hasCurrentResult && preview.isError && validationErrors.length > 0 && (
          <div className="mt-4">
            <LocatedErrors title={t('connectors.recipePreview.validationTitle')} errors={validationErrors} />
          </div>
        )}
        {hasCurrentResult && preview.isError && validationErrors.length === 0 && (
          <p className="mt-4 text-xs text-err" role="alert">
            {t('connectors.recipePreview.requestError')}
          </p>
        )}
        {hasCurrentResult && !preview.isError && preview.data && <PreviewResults result={preview.data} />}
      </Panel>
    </section>
  );
}

function PreviewResults({ result }: { result: RecipePreview }) {
  const { t } = useTranslation();
  const hasEndpointErrors = result.endpoints.some((endpoint) => endpoint.error);
  const hasErrors = result.errors.length > 0 || hasEndpointErrors;

  return (
    <div className="mt-4 space-y-4" aria-live="polite">
      <p className={hasErrors ? 'text-xs text-warn' : 'text-xs text-ok'}>
        {hasErrors ? t('connectors.recipePreview.partialSuccess') : t('connectors.recipePreview.success')}
      </p>

      {result.errors.length > 0 && (
        <div>
          <h3 className="text-xs font-medium text-ink">{t('connectors.recipePreview.errors')}</h3>
          <ul className="mt-1 list-disc space-y-1 pl-5 text-xs text-err">
            {result.errors.map((error, index) => (
              <li key={`${index}-${error}`} className="break-words">
                {error}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div>
        <h3 className="text-xs font-medium text-ink">{t('connectors.recipePreview.endpoints')}</h3>
        {result.endpoints.length === 0 ? (
          <p className="mt-1 text-xs text-ink-muted">{t('connectors.recipePreview.noEndpoints')}</p>
        ) : (
          <div className="mt-2 space-y-3">
            {result.endpoints.map((endpoint, index) => (
              <section
                key={`${endpoint.name}-${index}`}
                className="rounded-sm border border-line-soft p-3"
              >
                <h4 className="break-words text-xs font-semibold text-ink">{endpoint.name}</h4>
                <dl className="mt-2 grid grid-cols-1 gap-x-4 gap-y-1 text-xs sm:grid-cols-3">
                  <Metric label={t('connectors.recipePreview.items')} value={endpoint.items} />
                  <Metric label={t('connectors.recipePreview.entities')} value={endpoint.count} />
                  <Metric label={t('connectors.recipePreview.skipped')} value={endpoint.skipped} />
                </dl>
                {endpoint.error && (
                  <p className="mt-2 break-words text-xs text-err" role="alert">
                    {t('connectors.recipePreview.endpointError', { error: endpoint.error })}
                  </p>
                )}
                <EntitySamples samples={endpoint.samples ?? []} />
                <Dependencies dependencies={endpoint.dependencies ?? []} />
              </section>
            ))}
          </div>
        )}
      </div>

      <Dependencies dependencies={result.dependencies ?? []} aggregate />
    </div>
  );
}

function Metric({ label, value }: { label: string; value: number }) {
  const { i18n } = useTranslation();
  return (
    <div className="flex justify-between gap-2 sm:block">
      <dt className="text-ink-muted">{label}</dt>
      <dd className="tabular-nums text-ink">
        {new Intl.NumberFormat(i18n.resolvedLanguage || undefined).format(value)}
      </dd>
    </div>
  );
}

function EntitySamples({ samples }: { samples: SnapshotEntity[] }) {
  const { t } = useTranslation();
  return (
    <div className="mt-3">
      <h5 className="text-2xs font-medium text-ink-muted">{t('connectors.recipePreview.samples')}</h5>
      {samples.length === 0 ? (
        <p className="mt-1 text-xs text-ink-faint">{t('connectors.recipePreview.noSamples')}</p>
      ) : (
        <ul className="mt-1 space-y-2">
          {samples.map((sample, index) => (
            <li key={`${sample.kind}-${sample.externalId ?? sample.name}-${index}`}>
              <pre
                className="overflow-x-auto whitespace-pre-wrap break-words rounded-sm bg-canvas-sunken p-2 font-mono text-2xs text-ink"
                translate="no"
              >
                {JSON.stringify(sample, null, 2)}
              </pre>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

function Dependencies({ dependencies, aggregate = false }: { dependencies: SnapshotDependency[]; aggregate?: boolean }) {
  const { t } = useTranslation();
  return (
    <div className={aggregate ? '' : 'mt-3'}>
      <h5 className="text-2xs font-medium text-ink-muted">
        {aggregate ? t('connectors.recipePreview.allDependencies') : t('connectors.recipePreview.dependencies')}
      </h5>
      {dependencies.length === 0 ? (
        <p className="mt-1 text-xs text-ink-faint">{t('connectors.recipePreview.noDependencies')}</p>
      ) : (
        <ul className="mt-1 flex flex-wrap gap-1.5">
          {dependencies.map((dependency, index) => (
            <li
              key={`${dependency.kind}-${dependency.name}-${index}`}
              className="max-w-full rounded-sm bg-canvas-sunken px-2 py-1 text-2xs text-ink"
            >
              <span className="font-mono text-ink-faint">{dependency.kind}</span>
              <span>: </span>
              <span className="break-all">{dependency.name}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
