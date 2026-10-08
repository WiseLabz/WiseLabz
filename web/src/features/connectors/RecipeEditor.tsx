import { Component, Suspense, lazy, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ComponentType, ErrorInfo, KeyboardEvent, ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { FieldError, RecipePreview } from '../../api/model';
import { recipeFieldId, resolveRecipeError } from './recipeErrors';
import { parseRecipeDocument } from './recipeDocument';

type RecipeTab = 'form' | 'yaml';

type RecipeDiagnostic = {
  from: number;
  to: number;
  severity: 'error';
  message: string;
  source?: string;
};

type RecipeYamlEditorProps = {
  value: string;
  onChange: (value: string) => void;
  label: string;
  readOnly: boolean;
  diagnostics: RecipeDiagnostic[];
  focusLine?: number;
  focusRequest: number;
};

class RecipeYamlErrorBoundary extends Component<{
  children: ReactNode;
  fallback: ReactNode;
  onError: () => void;
}, { failed: boolean }> {
  state = { failed: false };

  static getDerivedStateFromError(): { failed: boolean } {
    return { failed: true };
  }

  componentDidCatch(_error: Error, _info: ErrorInfo) {
    this.props.onError();
  }

  render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}

const RecipeBuilder = lazy(() =>
  import('./RecipeBuilder').then((module) => ({ default: module.RecipeBuilder })),
);

const YAML_TAB_STORAGE_KEY = 'connector-recipe-editor-tab';

function readRememberedTab(): RecipeTab | undefined {
  try {
    const saved = window.localStorage.getItem(YAML_TAB_STORAGE_KEY);
    return saved === 'form' || saved === 'yaml' ? saved : undefined;
  } catch {
    return undefined;
  }
}

function rememberTab(tab: RecipeTab): void {
  try {
    window.localStorage.setItem(YAML_TAB_STORAGE_KEY, tab);
  } catch {
    // Storage can be unavailable in private browsing or restricted contexts.
  }
}

function lineStart(text: string, oneBasedLine: number): number {
  const line = Math.max(1, oneBasedLine);
  let offset = 0;
  for (let current = 1; current < line && offset < text.length; current += 1) {
    const next = text.indexOf('\n', offset);
    if (next < 0) return text.length;
    offset = next + 1;
  }
  return offset;
}

type RecipeDocumentIssue = NonNullable<
  ReturnType<typeof parseRecipeDocument>['error'] | ReturnType<typeof parseRecipeDocument>['unsupported']
>;

function parserDiagnostic(value: string, issue: RecipeDocumentIssue | undefined): RecipeDiagnostic[] {
  if (!issue || issue.kind !== 'syntax') return [];
  const from = lineStart(value, issue.line ?? 1);
  const lineEnd = value.indexOf('\n', from);
  const to = lineEnd < 0 ? value.length : lineEnd;
  return [{ from, to: Math.max(from, to), severity: 'error', message: issue.message, source: 'YAML' }];
}

/** The element for a located path, or the nearest ancestor field that is drawn. */
function nearestFormElement(path: (string | number)[]): HTMLElement | null {
  for (let length = path.length; length > 0; length -= 1) {
    const element = document.getElementById(recipeFieldId(path.slice(0, length)));
    if (element) return element;
  }
  return null;
}

export type RecipeEditorProps = {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  label?: string;
  errors?: FieldError[];
  errorRecipe?: string;
  endpointResults?: RecipePreview['endpoints'];
};

export function RecipeEditor({
  value,
  onChange,
  disabled = false,
  label,
  errors = [],
  errorRecipe,
  endpointResults,
}: RecipeEditorProps) {
  const { t } = useTranslation();
  const fieldLabel = label ?? t('connectors.recipeBuilder.recipe');
  const [parsed, setParsed] = useState(() => ({ value, result: parseRecipeDocument(value) }));
  const [tab, setTab] = useState<RecipeTab>(() => {
    const issue = value.trim() ? parsed.result.error ?? parsed.result.unsupported : undefined;
    const available = !issue;
    const remembered = readRememberedTab();
    if (remembered === 'yaml') return 'yaml';
    if (remembered === 'form' && available) return 'form';
    return available ? 'form' : 'yaml';
  });
  const [yamlEditor, setYamlEditor] = useState<ComponentType<RecipeYamlEditorProps> | null>(null);
  const [yamlLoadFailed, setYamlLoadFailed] = useState(false);
  const [focusLine, setFocusLine] = useState<number>();
  const [focusRequest, setFocusRequest] = useState(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const focusedRequest = useRef(0);

  const parseResult = parsed.value === value ? parsed.result : undefined;
  const issue = value.trim() ? parseResult?.error ?? parseResult?.unsupported : undefined;
  const formAvailable = !issue;
  const visibleTab = tab === 'form' && !formAvailable ? 'yaml' : tab;
  const errorsAreStale = errors.length > 0 && errorRecipe !== undefined && errorRecipe !== value;

  useEffect(() => {
    if (parsed.value === value) return;
    const timer = window.setTimeout(() => {
      setParsed({ value, result: parseRecipeDocument(value) });
    }, 100);
    return () => window.clearTimeout(timer);
  }, [value, parsed.value]);

  useEffect(() => {
    if (visibleTab !== 'yaml' || yamlEditor || yamlLoadFailed) return;
    let active = true;
    import('./RecipeYamlEditor')
      .then((module) => {
        if (active) setYamlEditor(() => module.RecipeYamlEditor);
      })
      .catch(() => {
        if (active) setYamlLoadFailed(true);
      });
    return () => {
      active = false;
    };
  }, [visibleTab, yamlEditor, yamlLoadFailed]);

  const diagnostics = useMemo(() => {
    const found = parserDiagnostic(value, issue);
    if (errorsAreStale) return found;
    for (const error of errors) {
      const located = resolveRecipeError(value, error.field);
      if (!located) continue;
      const from = Math.max(0, Math.min(value.length, located.from));
      const to = Math.max(from, Math.min(value.length, located.to));
      found.push({ from, to, severity: 'error', message: error.msg, source: 'Server' });
    }
    return found;
  }, [errors, errorsAreStale, issue, value]);

  const selectTab = useCallback((next: RecipeTab) => {
    if (next === 'form') {
      const result = parsed.value === value ? parsed.result : parseRecipeDocument(value);
      setParsed({ value, result });
      if (value.trim() && (result.error || result.unsupported)) return;
    }
    setTab(next);
    rememberTab(next);
  }, [parsed, value]);

  useEffect(() => {
    const handleFocus = (rawEvent: Event) => {
      const event = rawEvent as CustomEvent<{ field?: string }>;
      const requested = event.detail?.field;
      if (!requested) return;
      // The event names the first server error; later ones are fallbacks when it has nowhere to land.
      const fields = errorsAreStale ? [requested] : [requested, ...errors.map((error) => error.field).filter((field) => field !== requested)];
      const located = fields.flatMap((field) => resolveRecipeError(value, field) ?? []);
      if (!located.length) return;

      const nextTab = visibleTab;
      let resolved = located[0];
      let formTarget: HTMLElement | null = null;
      if (nextTab === 'form') {
        for (const candidate of located) {
          formTarget = nearestFormElement(candidate.path);
          if (formTarget) {
            resolved = candidate;
            break;
          }
        }
        // Nothing located is drawn in the Form: land on the panel rather than lose the request.
        formTarget ??= document.getElementById('recipe-panel-form');
        if (!formTarget) return;
      }
      event.preventDefault();
      setFocusLine(resolved.line);
      setFocusRequest((current) => current + 1);
      if (nextTab !== tab) {
        setTab(nextTab);
        rememberTab(nextTab);
      }

      const focusTarget = () => {
        if (nextTab === 'form') {
          formTarget?.focus();
          return;
        }
        const textarea = textareaRef.current;
        if (!textarea) return;
        const offset = lineStart(value, resolved.line);
        textarea.focus();
        textarea.setSelectionRange(offset, offset);
      };
      if (typeof window.requestAnimationFrame === 'function') window.requestAnimationFrame(focusTarget);
      else window.setTimeout(focusTarget, 0);
    };
    window.addEventListener('connector-recipe-focus', handleFocus);
    return () => window.removeEventListener('connector-recipe-focus', handleFocus);
  }, [errors, errorsAreStale, tab, visibleTab, value]);

  // Each request is handled once: the YAML editor acts on it as it commits, then it is cleared so
  // later edits or re-renders do not pull focus back.
  useEffect(() => {
    if (!focusRequest) {
      focusedRequest.current = 0;
      return;
    }
    if (visibleTab === 'yaml' && !yamlEditor) {
      const textarea = textareaRef.current;
      if (textarea && focusLine && focusedRequest.current !== focusRequest) {
        focusedRequest.current = focusRequest;
        const offset = lineStart(value, focusLine);
        textarea.focus();
        textarea.setSelectionRange(offset, offset);
      }
      // Keep the request while the lazy editor is still loading so it can take focus when it mounts.
      if (!yamlLoadFailed) return;
    }
    // Clear after the commit so the YAML editor has already seen this request.
    const timer = window.setTimeout(() => setFocusRequest(0), 0);
    return () => window.clearTimeout(timer);
  }, [focusLine, focusRequest, value, visibleTab, yamlEditor, yamlLoadFailed]);

  const tabKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    const tabs: RecipeTab[] = formAvailable ? ['form', 'yaml'] : ['yaml'];
    const current = tabs.indexOf(visibleTab);
    let next: RecipeTab | undefined;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') next = tabs[(current + 1) % tabs.length];
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') next = tabs[(current - 1 + tabs.length) % tabs.length];
    else if (event.key === 'Home') next = tabs[0];
    else if (event.key === 'End') next = tabs[tabs.length - 1];
    if (!next) return;
    event.preventDefault();
    selectTab(next);
    document.getElementById(`recipe-tab-${next}`)?.focus();
  };

  const unavailable = issue
    ? t('connectors.recipeBuilder.unavailable', {
        line: issue.line ?? 1,
        reason: t(`connectors.recipeBuilder.parser_${issue.kind}`, { message: issue.message }),
      })
    : undefined;
  const RecipeYamlEditor = yamlEditor;
  const yamlFallback = (
    <textarea
      ref={textareaRef}
      id="connector-field-recipe"
      aria-label={fieldLabel}
      value={value}
      onChange={(event) => onChange(event.currentTarget.value)}
      readOnly={disabled}
      rows={12}
      spellCheck={false}
      className="w-full rounded-sm border border-line bg-canvas-sunken p-3 font-mono text-xs text-ink focus-visible:outline-2 focus-visible:outline-accent-primary"
    />
  );

  return (
    <div className="space-y-3">
      <div role="tablist" aria-label={t('connectors.recipeBuilder.views')} className="flex gap-1 border-b border-line-soft">
        <button
          id="recipe-tab-form"
          type="button"
          role="tab"
          aria-selected={visibleTab === 'form'}
          aria-controls="recipe-panel-form"
          aria-disabled={!formAvailable}
          disabled={!formAvailable}
          tabIndex={visibleTab === 'form' ? 0 : -1}
          onClick={() => selectTab('form')}
          onKeyDown={tabKeyDown}
          className="border-b-2 border-transparent px-3 py-2 text-xs text-ink-muted aria-selected:border-accent-primary aria-selected:text-ink disabled:cursor-not-allowed disabled:opacity-50"
        >
          {t('connectors.recipeBuilder.form')}
        </button>
        <button
          id="recipe-tab-yaml"
          type="button"
          role="tab"
          aria-selected={visibleTab === 'yaml'}
          aria-controls="recipe-panel-yaml"
          tabIndex={visibleTab === 'yaml' ? 0 : -1}
          onClick={() => selectTab('yaml')}
          onKeyDown={tabKeyDown}
          className="border-b-2 border-transparent px-3 py-2 text-xs text-ink-muted aria-selected:border-accent-primary aria-selected:text-ink"
        >
          {t('connectors.recipeBuilder.yaml')}
        </button>
      </div>

      {unavailable && <p role="status" className="text-xs text-warn">{unavailable}</p>}
      {errorsAreStale && <p role="status" className="text-xs text-warn">{t('connectors.recipeBuilder.staleErrors')}</p>}

      {visibleTab === 'form' ? (
        <div id="recipe-panel-form" role="tabpanel" aria-labelledby="recipe-tab-form" tabIndex={0}>
          <Suspense fallback={<p className="text-xs text-ink-muted" aria-live="polite">{t('common.loading')}</p>}>
            <RecipeBuilder
              value={value}
              onChange={onChange}
              disabled={disabled}
              errors={errors}
              errorRecipe={errorRecipe}
              endpointResults={endpointResults}
            />
          </Suspense>
        </div>
      ) : (
        <div id="recipe-panel-yaml" role="tabpanel" aria-labelledby="recipe-tab-yaml" tabIndex={0}>
          {disabled && <p className="mb-2 text-xs text-ink-muted">{t('connectors.recipeBuilder.readOnly')}</p>}
          {RecipeYamlEditor ? (
            <RecipeYamlErrorBoundary fallback={yamlFallback} onError={() => setYamlLoadFailed(true)}>
              <RecipeYamlEditor
                value={value}
                onChange={onChange}
                label={fieldLabel}
                readOnly={disabled}
                diagnostics={diagnostics}
                focusLine={focusLine}
                focusRequest={focusRequest}
              />
            </RecipeYamlErrorBoundary>
          ) : (
            yamlFallback
          )}
        </div>
      )}
    </div>
  );
}

export default RecipeEditor;
