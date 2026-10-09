import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { FieldError, RecipePreview } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { recipeFieldId, resolveRecipeError } from './recipeErrors';
import {
  editRecipeDocument,
  parseRecipeDocument,
} from './recipeDocument';
import { recipeFormat, unknownRecipeKeys } from './recipeFormat';

type RecipePath = readonly (string | number)[];
type DocumentPath = Parameters<typeof editRecipeDocument>[1]['path'];
type RecipeEdit = Parameters<typeof editRecipeDocument>[1];
type Values = Record<string, unknown>;
type RecipeBuilderProps = {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  errors?: FieldError[];
  errorRecipe?: string;
  endpointResults?: RecipePreview['endpoints'];
};

const authModes = recipeFormat.fields.auth.item?.fields.mode.options ?? [];
const paginationStyles = recipeFormat.fields.endpoints.item?.fields.pagination.item?.fields.type.options ?? [];
const attributeTypes = recipeFormat.fields.endpoints.item?.fields.entity.item?.fields.attributes.item?.fields.type.options ?? [];
const dependencyKinds = recipeFormat.fields.dependencies.item?.fields.kind.options ?? [];
const actionMethods = recipeFormat.fields.actions.item?.fields.method.options ?? [];
const actionLimit = 10;
// Offered first when an action is added; other names are used once these are taken.
const lifecycleActionNames = ['restart', 'start', 'stop'];
// Keys each pagination style accepts (server validatePagination); other owned keys are dropped on style change.
const paginationFieldsByStyle: Record<string, readonly string[]> = {
  page: ['param', 'size_param', 'size', 'start'],
  offset: ['param', 'size_param', 'size', 'start'],
  cursor: ['param', 'cursor_path', 'size_param', 'size'],
  next_link: ['next_path', 'link_header'],
};
const paginationOwnedFields = ['param', 'size_param', 'size', 'start', 'cursor_path', 'next_path', 'link_header'];

export function RecipeBuilder({
  value,
  onChange,
  disabled = false,
  errors = [],
  errorRecipe,
  endpointResults = [],
}: RecipeBuilderProps) {
  const { t } = useTranslation();
  const [editFailed, setEditFailed] = useState(false);
  const parsed = useMemo(() => parseRecipeDocument(value), [value]);
  const document = parsed.document;
  const recipeValue = useMemo(() => document?.toJS(), [document]);
  const unknown = useMemo(() => unknownRecipeKeys(document), [document]);
  const locatedErrors = useMemo(() => {
    if (errorRecipe !== undefined && errorRecipe !== value) return [];
    return errors.flatMap((error) => {
      const target = resolveRecipeError(value, error.field);
      return target ? [{ ...target, message: error.msg }] : [];
    });
  }, [errors, errorRecipe, value]);

  const read = (path: RecipePath): unknown => valueAtPath(recipeValue, path);
  const editMany = (operations: RecipeEdit[]) => {
    try {
      let text = value;
      for (const operation of operations) {
        if (operation.type === 'append') {
          const current = parseRecipeDocument(text).document?.toJS();
          if (valueAtPath(current, operation.path) === undefined) {
            text = editRecipeDocument(text, { type: 'set', path: operation.path, value: [] });
          }
        }
        if (operation.type === 'delete') {
          const current = parseRecipeDocument(text).document?.toJS();
          if (valueAtPath(current, operation.path) === undefined) continue;
        }
        text = editRecipeDocument(text, operation);
      }
      setEditFailed(false);
      if (text !== value) onChange(text);
    } catch {
      // The engine refuses an edit it cannot make without touching other text; leave the recipe as it is.
      setEditFailed(true);
    }
  };
  const edit = (operation: RecipeEdit) => editMany([operation]);
  const issue = (path: RecipePath) => {
    const matching = locatedErrors.filter((error) => samePath(error.path, path));
    return {
      id: recipeFieldId([...path]),
      message: matching.map((error) => error.message).join(' '),
    };
  };
  const unknownAt = (path: RecipePath) => unknown.find((entry) => samePath(entry.path, path));
  const hasUnknownBelow = (path: RecipePath) => unknown.some((entry) => isPrefix(path, entry.path));
  const removeConfirm = (path: RecipePath) =>
    !hasUnknownBelow(path) || window.confirm(t('connectors.recipeBuilder.unknownRemoveConfirm'));

  if (!value.trim()) {
    return (
      <section className="rounded-sm border border-line-soft p-4" aria-labelledby="recipe-builder-start-title">
        <h3 id="recipe-builder-start-title" className="text-sm font-semibold text-ink">
          {t('connectors.recipeBuilder.startTitle')}
        </h3>
        <p className="mt-1 text-xs text-ink-muted">{t('connectors.recipeBuilder.startDescription')}</p>
        {disabled ? (
          <p className="mt-3 text-xs text-ink-faint">{t('connectors.recipeBuilder.emptyRecipe')}</p>
        ) : (
          <Button
            className="mt-3"
            variant="secondary"
            size="sm"
            onClick={() => onChange(startRecipe(value))}
          >
            {t('connectors.recipeBuilder.startRecipe')}
          </Button>
        )}
      </section>
    );
  }

  const unavailable = parsed.error ?? parsed.unsupported;
  if (unavailable || !document) {
    const reason = unavailable
      ? unavailable.kind === 'syntax'
        ? t('connectors.recipeBuilder.parser_syntax', { message: unavailable.message })
        : unavailable.kind === 'documents'
          ? t('connectors.recipeBuilder.parser_documents')
          : unavailable.kind === 'root'
            ? t('connectors.recipeBuilder.parser_root')
            : unavailable.kind === 'anchors'
              ? t('connectors.recipeBuilder.parser_anchors')
              : unavailable.kind === 'aliases'
                ? t('connectors.recipeBuilder.parser_aliases')
                : t('connectors.recipeBuilder.parser_merge')
      : '';
    return (
      <p className="rounded-sm border border-warn/30 bg-warn/5 p-3 text-xs text-warn" role="alert">
        {unavailable?.line
          ? t('connectors.recipeBuilder.unavailable', { line: unavailable.line, reason })
          : t('connectors.recipeBuilder.formUnavailable') + (reason ? ' ' + reason : '')}
      </p>
    );
  }

  const category = String(read(['category']) ?? '');
  const authMode = String(read(['auth', 'mode']) ?? 'none');
  const endpoints = list(read(['endpoints']));
  const dependencies = list(read(['dependencies']));
  const rootUnknown = unknownAt([]);
  const authUnknown = unknownAt(['auth']);

  return (
    <div className="space-y-4">
      {editFailed && (
        <p className="rounded-sm border border-warn/30 bg-warn/5 p-3 text-xs text-warn" role="alert">
          {t('connectors.recipeBuilder.editFailed')}
        </p>
      )}
      {rootUnknown && <UnknownMarker keys={rootUnknown.keys} />}

      <section className="rounded-sm border border-line-soft p-4">
        <h3 className="mb-3 text-sm font-semibold text-ink">{t('connectors.recipeBuilder.recipe')}</h3>
        <div
          id={issue(['version']).id}
          tabIndex={-1}
          aria-invalid={issue(['version']).message ? true : undefined}
          aria-describedby={issue(['version']).message ? issue(['version']).id + '-error' : undefined}
        >
          <ReadOnlyOrField label={t('connectors.recipeBuilder.version')} value={String(read(['version']) ?? '')} />
          {issue(['version']).message && <InlineIssue issue={issue(['version'])} />}
        </div>
        <SelectField
          label={t('connectors.recipeBuilder.category')}
          path={['category']}
          value={category}
          options={recipeFormat.fields.category.options ?? []}
          optionLabels={Object.fromEntries(
            (recipeFormat.fields.category.options ?? []).map((option) => [
              option,
              t('services.category.' + option),
            ]),
          )}
          includeEmpty={!category}
          disabled={disabled}
          issue={issue(['category'])}
          onChange={(next) => {
            if (next) edit({ type: 'set', path: ['category'], value: next });
          }}
        />

        <div className="mt-4 border-t border-line-soft pt-4">
          <div className="mb-2 flex items-center gap-2">
            <h4 className="text-xs font-semibold text-ink">{t('connectors.recipeBuilder.auth')}</h4>
            {authUnknown && <UnknownMarker keys={authUnknown.keys} />}
          </div>
          <SelectField
            label={t('connectors.recipeBuilder.authMode')}
            path={['auth', 'mode']}
            value={authMode}
            options={authModes}
            optionLabels={Object.fromEntries(
              authModes.map((option) => [option, t('connectors.recipeBuilder.auth' + capitalize(option))]),
            )}
            disabled={disabled}
            issue={issue(['auth', 'mode'])}
            onChange={(next) => {
              const operations: RecipeEdit[] = [{ type: 'set', path: ['auth', 'mode'], value: next }];
              if (next !== 'header' && next !== 'query') operations.push({ type: 'delete', path: ['auth', 'name'] });
              if (next !== 'header') operations.push({ type: 'delete', path: ['auth', 'prefix'] });
              editMany(operations);
            }}
          />
          {(authMode === 'header' || authMode === 'query') && (
            <TextField
              className="mt-3"
              label={t('connectors.recipeBuilder.authName')}
              path={['auth', 'name']}
              value={String(read(['auth', 'name']) ?? '')}
              disabled={disabled}
              issue={issue(['auth', 'name'])}
              onChange={(next) => edit({ type: 'set', path: ['auth', 'name'], value: next })}
            />
          )}
          {authMode === 'header' && (
            <TextField
              className="mt-3"
              label={t('connectors.recipeBuilder.authPrefix')}
              path={['auth', 'prefix']}
              value={String(read(['auth', 'prefix']) ?? '')}
              disabled={disabled}
              issue={issue(['auth', 'prefix'])}
              onChange={(next) => edit({ type: 'set', path: ['auth', 'prefix'], value: next })}
            />
          )}
        </div>
      </section>

      <section className="space-y-3" aria-labelledby="recipe-builder-endpoints-title">
        <div className="flex items-center justify-between gap-3">
          <h3 id="recipe-builder-endpoints-title" className="text-sm font-semibold text-ink">
            {t('connectors.recipeBuilder.endpoints')}
          </h3>
          {!disabled && (
            <Button size="sm" onClick={() => edit({ type: 'append', path: ['endpoints'], value: newEndpoint(endpoints) })}>
              {t('connectors.recipeBuilder.addEndpoint')}
            </Button>
          )}
        </div>
        {issue(['endpoints']).message && <InlineIssue issue={issue(['endpoints'])} />}
        {endpoints.map((endpoint, index) => (
          <EndpointEditor
            key={index}
            index={index}
            endpoint={record(endpoint)}
            disabled={disabled}
            endpointCount={endpoints.length}
            endpointResult={endpointResults.find((result) => result.name === String(record(endpoint).name ?? ''))}
            read={read}
            issue={issue}
            edit={edit}
            editMany={editMany}
            unknownAt={unknownAt}
            removeConfirm={removeConfirm}
            onMove={(to) => edit({ type: 'move', path: ['endpoints'], index, to })}
            onRemove={() => {
              if (removeConfirm(['endpoints', index])) edit({ type: 'remove', path: ['endpoints'], index });
            }}
          />
        ))}
        {endpoints.length === 0 && (
          <p className="rounded-sm border border-dashed border-line-soft p-3 text-xs text-ink-muted">
            {t('connectors.recipeBuilder.noEndpoints')}
          </p>
        )}
      </section>

      <DependencyEditor
        label={t('connectors.recipeBuilder.recipeDependencies')}
        path={['dependencies']}
        entries={dependencies}
        disabled={disabled}
        dependencyKinds={dependencyKinds}
        issue={issue}
        unknownAt={unknownAt}
        edit={edit}
        editMany={editMany}
        removeConfirm={removeConfirm}
      />

      <ActionsEditor
        label={t('connectors.recipeBuilder.serviceActions')}
        path={['actions']}
        entityActions={false}
        disabled={disabled}
        issue={issue}
        unknownAt={unknownAt}
        read={read}
        edit={edit}
        removeConfirm={removeConfirm}
      />
    </div>
  );
}

function EndpointEditor({
  index,
  endpoint,
  disabled,
  endpointCount,
  endpointResult,
  read,
  issue,
  edit,
  editMany,
  unknownAt,
  removeConfirm,
  onMove,
  onRemove,
}: {
  index: number;
  endpoint: Values;
  disabled: boolean;
  endpointCount: number;
  endpointResult?: RecipePreview['endpoints'][number];
  read: (path: RecipePath) => unknown;
  issue: (path: RecipePath) => { id: string; message: string };
  edit: (operation: RecipeEdit) => void;
  editMany: (operations: RecipeEdit[]) => void;
  unknownAt: (path: RecipePath) => { keys: string[] } | undefined;
  removeConfirm: (path: RecipePath) => boolean;
  onMove: (to: number) => void;
  onRemove: () => void;
}) {
  const { t } = useTranslation();
  const base: RecipePath = ['endpoints', index];
  const entity = record(read([...base, 'entity']));
  const endpointUnknown = unknownAt(base);
  const pagination = record(read([...base, 'pagination']));
  const paginationType = String(pagination.type ?? '');
  const query = record(read([...base, 'query']));
  const headers = record(read([...base, 'headers']));
  const attributes = record(read([...base, 'entity', 'attributes']));
  const dependencies = list(read([...base, 'dependencies']));
  const body = read([...base, 'body']);
  const endpointIssue = issue(base);

  return (
    <fieldset
      id={endpointIssue.id}
      tabIndex={-1}
      aria-invalid={endpointIssue.message ? true : undefined}
      aria-describedby={endpointIssue.message ? endpointIssue.id + '-error' : undefined}
      className="space-y-4 rounded-sm border border-line-soft p-4"
    >
      <legend className="px-1 text-xs font-semibold text-ink">
        {t('connectors.recipeBuilder.endpoint', { index: index + 1 })}
      </legend>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          {endpointUnknown && <UnknownMarker keys={endpointUnknown.keys} />}
          {endpointResult && (
            <span className={endpointResult.error ? 'text-2xs text-err' : 'text-2xs text-ink-muted'}>
              {endpointResult.error
                ? t('connectors.recipeBuilder.endpointFailed')
                : t('connectors.recipeBuilder.itemCount', { count: endpointResult.items })}
            </span>
          )}
        </div>
        {!disabled && (
          <div className="flex gap-1">
            <Button
              size="sm"
              aria-label={t('connectors.recipeBuilder.moveEndpointUp')}
              disabled={index === 0}
              onClick={() => onMove(index - 1)}
            >
              ↑
            </Button>
            <Button
              size="sm"
              aria-label={t('connectors.recipeBuilder.moveEndpointDown')}
              disabled={index === endpointCount - 1}
              onClick={() => onMove(index + 1)}
            >
              ↓
            </Button>
            <Button size="sm" variant="danger" onClick={onRemove}>
              {t('connectors.recipeBuilder.removeEndpoint')}
            </Button>
          </div>
        )}
      </div>
      {endpointIssue.message && <InlineIssue issue={endpointIssue} />}

      <div className="grid gap-3 sm:grid-cols-2">
        <TextField
          label={t('connectors.recipeBuilder.endpointName')}
          path={[...base, 'name']}
          value={String(endpoint.name ?? '')}
          disabled={disabled}
          issue={issue([...base, 'name'])}
          onChange={(next) => edit({ type: 'set', path: [...base, 'name'], value: next })}
        />
        <TextField
          label={t('connectors.recipeBuilder.path')}
          path={[...base, 'path']}
          value={String(endpoint.path ?? '')}
          disabled={disabled}
          issue={issue([...base, 'path'])}
          onChange={(next) => edit({ type: 'set', path: [...base, 'path'], value: next })}
        />
        <SelectField
          label={t('connectors.recipeBuilder.method')}
          path={[...base, 'method']}
          value={String(endpoint.method ?? 'GET')}
          options={recipeFormat.fields.endpoints.item?.fields.method.options ?? []}
          disabled={disabled}
          issue={issue([...base, 'method'])}
          onChange={(next) => edit({ type: 'set', path: [...base, 'method'], value: next })}
        />
        <TextField
          label={t('connectors.recipeBuilder.itemsPath')}
          path={[...base, 'items']}
          value={String(endpoint.items ?? '')}
          disabled={disabled}
          issue={issue([...base, 'items'])}
          onChange={(next) => edit({ type: 'set', path: [...base, 'items'], value: next })}
        />
      </div>

      <StringMapEditor
        label={t('connectors.recipeBuilder.query')}
        path={[...base, 'query']}
        values={query}
        disabled={disabled}
        issue={issue}
        edit={edit}
        onConfirmRemove={(entryPath) => removeConfirm(entryPath)}
      />
      <StringMapEditor
        label={t('connectors.recipeBuilder.headers')}
        path={[...base, 'headers']}
        values={headers}
        disabled={disabled}
        issue={issue}
        edit={edit}
        onConfirmRemove={(entryPath) => removeConfirm(entryPath)}
      />
      <BodyEditor
        key={formatBlock(body)}
        path={[...base, 'body']}
        value={body}
        disabled={disabled}
        issue={issue([...base, 'body'])}
        edit={edit}
      />

      <PaginationEditor
        path={[...base, 'pagination']}
        pagination={pagination}
        paginationType={paginationType}
        disabled={disabled}
        issue={issue}
        edit={edit}
        editMany={editMany}
        unknownAt={unknownAt}
        removeConfirm={removeConfirm}
      />

      <section
        id={issue([...base, 'entity']).id}
        tabIndex={-1}
        aria-invalid={issue([...base, 'entity']).message ? true : undefined}
        aria-describedby={issue([...base, 'entity']).message ? issue([...base, 'entity']).id + '-error' : undefined}
        className="space-y-3 rounded-sm border border-line-soft p-3"
      >
        <div className="flex items-center gap-2">
          <h4 className="text-xs font-semibold text-ink">{t('connectors.recipeBuilder.entity')}</h4>
          {unknownAt([...base, 'entity']) && (
            <UnknownMarker keys={unknownAt([...base, 'entity'])?.keys ?? []} />
          )}
        </div>
        {issue([...base, 'entity']).message && <InlineIssue issue={issue([...base, 'entity'])} />}
        <div className="grid gap-3 sm:grid-cols-2">
          {([
            ['kind', t('connectors.recipeBuilder.kind')],
            ['name', t('connectors.recipeBuilder.name')],
            ['external_id', t('connectors.recipeBuilder.externalId')],
            ['hostname', t('connectors.recipeBuilder.hostname')],
            ['ip', t('connectors.recipeBuilder.ip')],
            ['mac', t('connectors.recipeBuilder.mac')],
            ['aliases', t('connectors.recipeBuilder.aliases')],
          ] as const).map(([field, label]) => (
            <TextField
              key={field}
              label={label}
              path={[...base, 'entity', field]}
              value={String(entity[field] ?? '')}
              disabled={disabled}
              issue={issue([...base, 'entity', field])}
              onChange={(next) => edit({ type: 'set', path: [...base, 'entity', field], value: next })}
            />
          ))}
        </div>

        <div
          id={issue([...base, 'entity', 'attributes']).id}
          tabIndex={-1}
          aria-invalid={issue([...base, 'entity', 'attributes']).message ? true : undefined}
          aria-describedby={issue([...base, 'entity', 'attributes']).message ? issue([...base, 'entity', 'attributes']).id + '-error' : undefined}
          className="space-y-3 border-t border-line-soft pt-3"
        >
          <div className="flex items-center justify-between gap-2">
            <div className="flex items-center gap-2">
              <h5 className="text-xs font-semibold text-ink">{t('connectors.recipeBuilder.attributes')}</h5>
              {unknownAt([...base, 'entity', 'attributes']) && (
                <UnknownMarker keys={unknownAt([...base, 'entity', 'attributes'])?.keys ?? []} />
              )}
            </div>
            {!disabled && (
              <Button
                size="sm"
                onClick={() => {
                  const name = uniqueName(attributes, 'new_attribute');
                  edit({ type: 'set', path: [...base, 'entity', 'attributes', name], value: { path: '', type: 'string' } });
                }}
              >
                {t('connectors.recipeBuilder.addAttribute')}
              </Button>
            )}
          </div>
          {issue([...base, 'entity', 'attributes']).message && (
            <InlineIssue issue={issue([...base, 'entity', 'attributes'])} />
          )}
          {Object.entries(attributes).map(([name, attribute]) => (
            <AttributeEditor
              key={name}
              endpointPath={base}
              name={name}
              attribute={record(attribute)}
              disabled={disabled}
              unknownAt={unknownAt}
              issue={issue}
              edit={edit}
              editMany={editMany}
              read={read}
              removeConfirm={removeConfirm}
            />
          ))}
          {Object.keys(attributes).length === 0 && (
            <p className="text-xs text-ink-muted">{t('connectors.recipeBuilder.noAttributes')}</p>
          )}
        </div>

        <ActionsEditor
          label={t('connectors.recipeBuilder.entityActions')}
          path={[...base, 'entity', 'actions']}
          entityActions
          disabled={disabled}
          issue={issue}
          unknownAt={unknownAt}
          read={read}
          edit={edit}
          removeConfirm={removeConfirm}
        />
      </section>

      <DependencyEditor
        label={t('connectors.recipeBuilder.endpointDependencies')}
        path={[...base, 'dependencies']}
        entries={dependencies}
        disabled={disabled}
        dependencyKinds={dependencyKinds}
        issue={issue}
        unknownAt={unknownAt}
        edit={edit}
        editMany={editMany}
        removeConfirm={removeConfirm}
      />
    </fieldset>
  );
}

function PaginationEditor({
  path,
  pagination,
  paginationType,
  disabled,
  issue,
  edit,
  editMany,
  unknownAt,
  removeConfirm,
}: {
  path: RecipePath;
  pagination: Values;
  paginationType: string;
  disabled: boolean;
  issue: (path: RecipePath) => { id: string; message: string };
  edit: (operation: RecipeEdit) => void;
  editMany: (operations: RecipeEdit[]) => void;
  unknownAt: (path: RecipePath) => { keys: string[] } | undefined;
  removeConfirm: (path: RecipePath) => boolean;
}) {
  const { t } = useTranslation();
  const paginationIssue = issue(path);
  const setType = (next: string) => {
    if (!next) {
      if (removeConfirm(path)) edit({ type: 'delete', path });
      return;
    }
    const allowed: readonly string[] = paginationFieldsByStyle[next] ?? [];
    editMany([
      ...paginationOwnedFields
        .filter((field) => !allowed.includes(field))
        .map((field) => ({ type: 'delete', path: [...path, field] }) as RecipeEdit),
      { type: 'set', path: [...path, 'type'], value: next },
    ]);
  };
  const stringInput = (field: string, label: string) => (
    <TextField
      key={field}
      label={label}
      path={[...path, field]}
      value={String(pagination[field] ?? '')}
      disabled={disabled}
      issue={issue([...path, field])}
      onChange={(next) => {
        if (next) edit({ type: 'set', path: [...path, field], value: next });
        else edit({ type: 'delete', path: [...path, field] });
      }}
    />
  );
  const numberInput = (field: string, label: string) => (
    <TextField
      key={field}
      type="number"
      label={label}
      path={[...path, field]}
      value={pagination[field] === undefined ? '' : String(pagination[field])}
      disabled={disabled}
      issue={issue([...path, field])}
      onChange={(next) => {
        if (next === '') edit({ type: 'delete', path: [...path, field] });
        else if (Number.isFinite(Number(next))) edit({ type: 'set', path: [...path, field], value: Number(next) });
      }}
    />
  );
  const unknown = unknownAt(path);

  return (
    <section
      id={paginationIssue.id}
      tabIndex={-1}
      aria-invalid={paginationIssue.message ? true : undefined}
      aria-describedby={paginationIssue.message ? paginationIssue.id + '-error' : undefined}
      className="space-y-3 rounded-sm border border-line-soft p-3"
    >
      <div className="flex items-center gap-2">
        <h4 className="text-xs font-semibold text-ink">{t('connectors.recipeBuilder.pagination')}</h4>
        {unknown && <UnknownMarker keys={unknown.keys} />}
      </div>
      {paginationIssue.message && <InlineIssue issue={paginationIssue} />}
      <SelectField
        label={t('connectors.recipeBuilder.paginationType')}
        path={[...path, 'type']}
        value={paginationType}
        options={paginationStyles}
        optionLabels={{
          page: t('connectors.recipeBuilder.paginationPage'),
          offset: t('connectors.recipeBuilder.paginationOffset'),
          cursor: t('connectors.recipeBuilder.paginationCursor'),
          next_link: t('connectors.recipeBuilder.paginationNextLink'),
        }}
        includeEmpty
        emptyLabel={t('connectors.recipeBuilder.noPagination')}
        disabled={disabled}
        issue={issue([...path, 'type'])}
        onChange={setType}
      />
      {paginationType === 'page' && (
        <div className="grid gap-3 sm:grid-cols-2">
          {stringInput('param', t('connectors.recipeBuilder.param'))}
          {stringInput('size_param', t('connectors.recipeBuilder.sizeParam'))}
          {numberInput('size', t('connectors.recipeBuilder.size'))}
          {numberInput('start', t('connectors.recipeBuilder.start'))}
        </div>
      )}
      {paginationType === 'offset' && (
        <div className="grid gap-3 sm:grid-cols-2">
          {stringInput('param', t('connectors.recipeBuilder.param'))}
          {stringInput('size_param', t('connectors.recipeBuilder.sizeParam'))}
          {numberInput('size', t('connectors.recipeBuilder.size'))}
          {numberInput('start', t('connectors.recipeBuilder.start'))}
        </div>
      )}
      {paginationType === 'cursor' && (
        <div className="grid gap-3 sm:grid-cols-2">
          {stringInput('param', t('connectors.recipeBuilder.param'))}
          {stringInput('cursor_path', t('connectors.recipeBuilder.cursorPath'))}
          {stringInput('size_param', t('connectors.recipeBuilder.sizeParam'))}
          {numberInput('size', t('connectors.recipeBuilder.size'))}
        </div>
      )}
      {paginationType === 'next_link' && (
        <div className="space-y-3">
          <CheckboxField
            label={t('connectors.recipeBuilder.linkHeader')}
            path={[...path, 'link_header']}
            checked={pagination.link_header === true}
            disabled={disabled}
            issue={issue([...path, 'link_header'])}
            onChange={(checked) => editMany([
              { type: 'delete', path: [...path, 'next_path'] },
              ...(checked
                ? [{ type: 'set', path: [...path, 'link_header'], value: true } as RecipeEdit]
                : [{ type: 'delete', path: [...path, 'link_header'] } as RecipeEdit]),
            ])}
          />
          {pagination.link_header !== true && stringInput('next_path', t('connectors.recipeBuilder.nextPath'))}
        </div>
      )}
    </section>
  );
}

function AttributeEditor({
  endpointPath,
  name,
  attribute,
  disabled,
  unknownAt,
  issue,
  edit,
  editMany,
  read,
  removeConfirm,
}: {
  endpointPath: RecipePath;
  name: string;
  attribute: Values;
  disabled: boolean;
  unknownAt: (path: RecipePath) => { keys: string[] } | undefined;
  issue: (path: RecipePath) => { id: string; message: string };
  edit: (operation: RecipeEdit) => void;
  editMany: (operations: RecipeEdit[]) => void;
  read: (path: RecipePath) => unknown;
  removeConfirm: (path: RecipePath) => boolean;
}) {
  const { t } = useTranslation();
  const path: RecipePath = [...endpointPath, 'entity', 'attributes', name];
  const attributeIssue = issue(path);
  const source = attribute.path !== undefined ? 'path' : attribute.const !== undefined ? 'const' : attribute.template !== undefined ? 'template' : 'path';
  const valueMap = record(read([...path, 'map']));
  const unknown = unknownAt(path);

  const rename = (next: string) => {
    if (!next || next === name || record(read([...endpointPath, 'entity', 'attributes']))[next] !== undefined) return;
    edit({ type: 'rename', path, to: next });
  };
  const switchSource = (next: string) => {
    const sourceKeys = ['path', 'const', 'template'];
    const operations: RecipeEdit[] = [
      { type: 'set', path: [...path, next], value: '' },
      ...sourceKeys
        .filter((key) => key !== next)
        .map((key) => ({ type: 'delete', path: [...path, key] }) as RecipeEdit),
    ];
    editMany(operations);
  };
  const scalarPath = [...path, source] as RecipePath;

  return (
    <fieldset
      id={attributeIssue.id}
      tabIndex={-1}
      aria-invalid={attributeIssue.message ? true : undefined}
      aria-describedby={attributeIssue.message ? attributeIssue.id + '-error' : undefined}
      className="space-y-3 rounded-sm border border-line-soft p-3"
    >
      <legend className="px-1 text-xs font-medium text-ink">{name}</legend>
      <div className="flex flex-wrap items-end justify-between gap-2">
        <KeyField
          label={t('connectors.recipeBuilder.attributeName')}
          path={[...path, 'key']}
          value={name}
          disabled={disabled}
          onCommit={rename}
        />
        <div className="flex items-center gap-2">
          {unknown && <UnknownMarker keys={unknown.keys} />}
          {!disabled && (
            <Button
              size="sm"
              variant="danger"
              onClick={() => {
                if (removeConfirm(path)) edit({ type: 'delete', path });
              }}
            >
              {t('connectors.recipeBuilder.removeAttribute')}
            </Button>
          )}
        </div>
      </div>
      {attributeIssue.message && <InlineIssue issue={attributeIssue} />}
      <SelectField
        label={t('connectors.recipeBuilder.source')}
        path={[...path, 'source']}
        value={source}
        options={['path', 'const', 'template']}
        optionLabels={{
          path: t('connectors.recipeBuilder.sourcePath'),
          const: t('connectors.recipeBuilder.sourceConstant'),
          template: t('connectors.recipeBuilder.sourceTemplate'),
        }}
        disabled={disabled}
        onChange={switchSource}
      />
      {source === 'const' ? (
        <ScalarField
          label={t('connectors.recipeBuilder.const')}
          path={scalarPath}
          value={attribute.const}
          disabled={disabled}
          issue={issue(scalarPath)}
          onChange={(next) => edit({ type: 'set', path: scalarPath, value: next })}
        />
      ) : (
        <TextField
          label={source === 'path' ? t('connectors.recipeBuilder.sourcePath') : t('connectors.recipeBuilder.sourceTemplate')}
          path={scalarPath}
          value={String(attribute[source] ?? '')}
          disabled={disabled}
          issue={issue(scalarPath)}
          onChange={(next) => edit({ type: 'set', path: scalarPath, value: next })}
        />
      )}
      <SelectField
        label={t('connectors.recipeBuilder.type')}
        path={[...path, 'type']}
        value={String(attribute.type ?? '')}
        options={attributeTypes}
        optionLabels={{
          string: t('connectors.recipeBuilder.scalarString'),
          number: t('connectors.recipeBuilder.scalarNumber'),
          bool: t('connectors.recipeBuilder.scalarBoolean'),
          boolean: t('connectors.recipeBuilder.scalarBoolean'),
          list: t('connectors.recipeBuilder.attributeTypeList'),
          string_array: t('connectors.recipeBuilder.attributeTypeStringArray'),
        }}
        includeEmpty
        emptyLabel={t('connectors.recipeBuilder.noType')}
        disabled={disabled}
        issue={issue([...path, 'type'])}
        onChange={(next) => {
          if (next) edit({ type: 'set', path: [...path, 'type'], value: next });
          else edit({ type: 'delete', path: [...path, 'type'] });
        }}
      />

      <div
        id={issue([...path, 'map']).id}
        tabIndex={-1}
        aria-invalid={issue([...path, 'map']).message ? true : undefined}
        aria-describedby={issue([...path, 'map']).message ? issue([...path, 'map']).id + '-error' : undefined}
        className="space-y-2 border-t border-line-soft pt-3"
      >
        <div className="flex items-center justify-between gap-2">
          <h5 className="text-2xs font-semibold text-ink">{t('connectors.recipeBuilder.valueMap')}</h5>
          {!disabled && (
            <Button
              size="sm"
              onClick={() => {
                const key = uniqueName(valueMap, 'new_value');
                edit({ type: 'set', path: [...path, 'map', key], value: '' });
              }}
            >
              {t('connectors.recipeBuilder.addMapEntry')}
          </Button>
          )}
        </div>
        {issue([...path, 'map']).message && <InlineIssue issue={issue([...path, 'map'])} />}
        {Object.entries(valueMap).map(([key, value]) => {
          const entryPath = [...path, 'map', key] as RecipePath;
          const renameMapEntry = (next: string) => {
            if (!next || next === key || valueMap[next] !== undefined) return;
            edit({ type: 'rename', path: entryPath, to: next });
          };
          return (
            <div key={key} className="grid gap-2 sm:grid-cols-[1fr_2fr_auto]">
              <KeyField
                label={t('connectors.recipeBuilder.key')}
                path={[...entryPath, 'key']}
                value={key}
                disabled={disabled}
                onCommit={renameMapEntry}
              />
              <ScalarField
                label={t('connectors.recipeBuilder.value')}
                path={entryPath}
                value={value}
                disabled={disabled}
                issue={issue(entryPath)}
                onChange={(next) => edit({ type: 'set', path: entryPath, value: next })}
              />
              {!disabled && (
                <Button
                  size="sm"
                  variant="danger"
                  className="self-end"
                  onClick={() => {
                    if (removeConfirm(entryPath)) {
                      edit({
                        type: 'delete',
                        path: Object.keys(valueMap).length === 1 ? [...path, 'map'] : entryPath,
                      });
                    }
                  }}
                >
                  {t('connectors.recipeBuilder.removeEntry')}
                </Button>
              )}
            </div>
          );
        })}
      </div>

      <div className="space-y-2 border-t border-line-soft pt-3">
        <h5 className="text-2xs font-semibold text-ink">{t('connectors.recipeBuilder.default')}</h5>
        {attribute.default !== undefined ? (
          <div className="flex items-end gap-2">
            <ScalarField
              label={t('connectors.recipeBuilder.default')}
              path={[...path, 'default']}
              value={attribute.default}
              disabled={disabled}
              issue={issue([...path, 'default'])}
              onChange={(next) => edit({ type: 'set', path: [...path, 'default'], value: next })}
            />
            {!disabled && (
              <Button
                size="sm"
                variant="danger"
                onClick={() => {
                  if (removeConfirm([...path, 'default'])) edit({ type: 'delete', path: [...path, 'default'] });
                }}
              >
                {t('connectors.recipeBuilder.removeEntry')}
              </Button>
            )}
          </div>
        ) : disabled ? (
          <p className="text-xs text-ink-muted">{t('connectors.recipeBuilder.noDefault')}</p>
        ) : (
          <Button
            size="sm"
            onClick={() => edit({ type: 'set', path: [...path, 'default'], value: '' })}
          >
            {t('connectors.recipeBuilder.addDefault')}
          </Button>
        )}
      </div>
    </fieldset>
  );
}

function DependencyEditor({
  label,
  path,
  entries,
  disabled,
  dependencyKinds,
  issue,
  unknownAt,
  edit,
  editMany,
  removeConfirm,
}: {
  label: string;
  path: RecipePath;
  entries: unknown[];
  disabled: boolean;
  dependencyKinds: readonly string[];
  issue: (path: RecipePath) => { id: string; message: string };
  unknownAt: (path: RecipePath) => { keys: string[] } | undefined;
  edit: (operation: RecipeEdit) => void;
  editMany: (operations: RecipeEdit[]) => void;
  removeConfirm: (path: RecipePath) => boolean;
}) {
  const { t } = useTranslation();
  const containerUnknown = unknownAt(path);
  return (
    <section
      id={issue(path).id}
      tabIndex={-1}
      aria-invalid={issue(path).message ? true : undefined}
      aria-describedby={issue(path).message ? issue(path).id + '-error' : undefined}
      className="space-y-3 rounded-sm border border-line-soft p-4"
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <h3 className="text-xs font-semibold text-ink">{label}</h3>
          {containerUnknown && <UnknownMarker keys={containerUnknown.keys} />}
        </div>
        {!disabled && (
          <Button
            size="sm"
            onClick={() => edit({ type: 'append', path, value: { kind: 'host', path: '' } })}
          >
            {t('connectors.recipeBuilder.addDependency')}
          </Button>
        )}
      </div>
      {issue(path).message && <InlineIssue issue={issue(path)} />}
      {entries.map((entry, index) => {
        const base = [...path, index] as RecipePath;
        const value = record(entry);
        const rowUnknown = unknownAt(base);
        const source = value.path !== undefined ? 'path' : value.const !== undefined ? 'const' : 'path';
        const rowIssue = issue(base);
        const sourcePath = [...base, source] as RecipePath;
        return (
          <fieldset
            key={index}
            id={rowIssue.id}
            tabIndex={-1}
            aria-invalid={rowIssue.message ? true : undefined}
            aria-describedby={rowIssue.message ? rowIssue.id + '-error' : undefined}
            className="space-y-3 rounded-sm border border-line-soft p-3"
          >
            <legend className="px-1 text-2xs font-medium text-ink">
              {t('connectors.recipeBuilder.dependency', { index: index + 1 })}
            </legend>
            <div className="flex flex-wrap items-center justify-between gap-2">
              {rowUnknown && <UnknownMarker keys={rowUnknown.keys} />}
              {!disabled && (
                <Button
                  size="sm"
                  variant="danger"
                  onClick={() => {
                    if (removeConfirm(base)) edit({ type: 'remove', path, index });
                  }}
                >
                  {t('connectors.recipeBuilder.removeDependency')}
                </Button>
              )}
            </div>
            {rowIssue.message && <InlineIssue issue={rowIssue} />}
            <div className="grid gap-3 sm:grid-cols-2">
              <SelectField
                label={t('connectors.recipeBuilder.dependencyKind')}
                path={[...base, 'kind']}
                value={String(value.kind ?? 'host')}
                options={dependencyKinds}
                optionLabels={{
                  host: t('connectors.recipeBuilder.dependencyHost'),
                  network: t('connectors.recipeBuilder.dependencyNetwork'),
                  storage: t('connectors.recipeBuilder.dependencyStorage'),
                  upstream_service: t('connectors.recipeBuilder.dependencyUpstreamService'),
                }}
                disabled={disabled}
                issue={issue([...base, 'kind'])}
                onChange={(next) => edit({ type: 'set', path: [...base, 'kind'], value: next })}
              />
              <SelectField
                label={t('connectors.recipeBuilder.dependencySource')}
                path={[...base, 'source']}
                value={source}
                options={['path', 'const']}
                optionLabels={{
                  path: t('connectors.recipeBuilder.sourcePath'),
                  const: t('connectors.recipeBuilder.sourceConstant'),
                }}
                disabled={disabled}
                onChange={(next) => {
                  const opposite = next === 'path' ? 'const' : 'path';
                  editMany([
                    { type: 'set', path: [...base, next], value: '' },
                    { type: 'delete', path: [...base, opposite] },
                  ]);
                }}
              />
              <TextField
                className="sm:col-span-2"
                label={source === 'path' ? t('connectors.recipeBuilder.sourcePath') : t('connectors.recipeBuilder.sourceConstant')}
                path={sourcePath}
                value={String(value[source] ?? '')}
                disabled={disabled}
                issue={issue(sourcePath)}
                onChange={(next) => edit({ type: 'set', path: sourcePath, value: next })}
              />
            </div>
          </fieldset>
        );
      })}
      {entries.length === 0 && (
        <p className="text-xs text-ink-muted">{t('connectors.recipeBuilder.noDependencies')}</p>
      )}
    </section>
  );
}

function ActionsEditor({
  label,
  path,
  entityActions,
  disabled,
  issue,
  unknownAt,
  read,
  edit,
  removeConfirm,
}: {
  label: string;
  path: RecipePath;
  entityActions: boolean;
  disabled: boolean;
  issue: (path: RecipePath) => { id: string; message: string };
  unknownAt: (path: RecipePath) => { keys: string[] } | undefined;
  read: (path: RecipePath) => unknown;
  edit: (operation: RecipeEdit) => void;
  removeConfirm: (path: RecipePath) => boolean;
}) {
  const { t } = useTranslation();
  const actions = record(read(path));
  const names = Object.keys(actions);
  const containerIssue = issue(path);
  const containerUnknown = unknownAt(path);
  const full = names.length >= actionLimit;
  const addAction = () => {
    const name = lifecycleActionNames.find((candidate) => !(candidate in actions)) ?? uniqueName(actions, 'action');
    edit({ type: 'set', path: [...path, name], value: { method: 'POST', path: '' } });
  };

  return (
    <section
      id={containerIssue.id}
      tabIndex={-1}
      aria-invalid={containerIssue.message ? true : undefined}
      aria-describedby={containerIssue.message ? containerIssue.id + '-error' : undefined}
      className="space-y-3 rounded-sm border border-line-soft p-3"
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <h4 className="text-xs font-semibold text-ink">{label}</h4>
          {containerUnknown && <UnknownMarker keys={containerUnknown.keys} />}
        </div>
        {!disabled && (
          <Button size="sm" disabled={full} onClick={addAction}>
            {t('connectors.recipeBuilder.addAction')}
          </Button>
        )}
      </div>
      {entityActions && <p className="text-2xs text-ink-muted">{t('connectors.recipeBuilder.entityActionsHelp')}</p>}
      {!disabled && full && <p className="text-2xs text-ink-muted">{t('connectors.recipeBuilder.actionsLimit')}</p>}
      {containerIssue.message && <InlineIssue issue={containerIssue} />}
      {names.map((name) => (
        <ActionRow
          key={name}
          path={[...path, name]}
          name={name}
          action={record(actions[name])}
          siblings={actions}
          disabled={disabled}
          issue={issue}
          unknownAt={unknownAt}
          edit={edit}
          removeConfirm={removeConfirm}
        />
      ))}
      {names.length === 0 && <p className="text-xs text-ink-muted">{t('connectors.recipeBuilder.noActions')}</p>}
    </section>
  );
}

function ActionRow({
  path,
  name,
  action,
  siblings,
  disabled,
  issue,
  unknownAt,
  edit,
  removeConfirm,
}: {
  path: RecipePath;
  name: string;
  action: Values;
  siblings: Values;
  disabled: boolean;
  issue: (path: RecipePath) => { id: string; message: string };
  unknownAt: (path: RecipePath) => { keys: string[] } | undefined;
  edit: (operation: RecipeEdit) => void;
  removeConfirm: (path: RecipePath) => boolean;
}) {
  const { t } = useTranslation();
  const rowIssue = issue(path);
  const rowUnknown = unknownAt(path);
  const rename = (next: string) => {
    if (!next || next === name || siblings[next] !== undefined) return;
    edit({ type: 'rename', path, to: next });
  };
  const optionalText = (field: 'label' | 'description', fieldLabel: string, className = '') => (
    <TextField
      className={className}
      label={fieldLabel}
      path={[...path, field]}
      value={String(action[field] ?? '')}
      disabled={disabled}
      issue={issue([...path, field])}
      onChange={(next) => {
        if (next) edit({ type: 'set', path: [...path, field], value: next });
        else edit({ type: 'delete', path: [...path, field] });
      }}
    />
  );

  return (
    <fieldset
      id={rowIssue.id}
      tabIndex={-1}
      aria-invalid={rowIssue.message ? true : undefined}
      aria-describedby={rowIssue.message ? rowIssue.id + '-error' : undefined}
      className="space-y-3 rounded-sm border border-line-soft p-3"
    >
      <legend className="px-1 text-2xs font-medium text-ink">{name}</legend>
      <div className="flex flex-wrap items-end justify-between gap-2">
        <KeyField
          label={t('connectors.recipeBuilder.actionName')}
          path={[...path, 'key']}
          value={name}
          disabled={disabled}
          onCommit={rename}
        />
        <div className="flex items-center gap-2">
          {rowUnknown && <UnknownMarker keys={rowUnknown.keys} />}
          {!disabled && (
            <Button
              size="sm"
              variant="danger"
              onClick={() => {
                if (removeConfirm(path)) edit({ type: 'delete', path });
              }}
            >
              {t('connectors.recipeBuilder.removeAction')}
            </Button>
          )}
        </div>
      </div>
      {rowIssue.message && <InlineIssue issue={rowIssue} />}
      <div className="grid gap-3 sm:grid-cols-2">
        <SelectField
          label={t('connectors.recipeBuilder.actionMethod')}
          path={[...path, 'method']}
          value={String(action.method ?? '')}
          options={actionMethods}
          includeEmpty={action.method === undefined || action.method === null}
          disabled={disabled}
          issue={issue([...path, 'method'])}
          onChange={(next) => {
            if (next) edit({ type: 'set', path: [...path, 'method'], value: next });
          }}
        />
        <TextField
          label={t('connectors.recipeBuilder.actionPath')}
          path={[...path, 'path']}
          value={String(action.path ?? '')}
          disabled={disabled}
          issue={issue([...path, 'path'])}
          onChange={(next) => edit({ type: 'set', path: [...path, 'path'], value: next })}
        />
        {optionalText('label', t('connectors.recipeBuilder.actionLabel'))}
        {optionalText('description', t('connectors.recipeBuilder.actionDescription'), 'sm:col-span-2')}
        <TextField
          type="number"
          label={t('connectors.recipeBuilder.downtimeSeconds')}
          path={[...path, 'downtime_seconds']}
          value={action.downtime_seconds === undefined ? '' : String(action.downtime_seconds)}
          disabled={disabled}
          issue={issue([...path, 'downtime_seconds'])}
          onChange={(next) => {
            if (next === '') edit({ type: 'delete', path: [...path, 'downtime_seconds'] });
            else if (Number.isFinite(Number(next))) edit({ type: 'set', path: [...path, 'downtime_seconds'], value: Number(next) });
          }}
        />
      </div>
      <StringMapEditor
        label={t('connectors.recipeBuilder.query')}
        path={[...path, 'query']}
        values={record(action.query)}
        disabled={disabled}
        issue={issue}
        edit={edit}
        onConfirmRemove={removeConfirm}
      />
      <StringMapEditor
        label={t('connectors.recipeBuilder.headers')}
        path={[...path, 'headers']}
        values={record(action.headers)}
        disabled={disabled}
        issue={issue}
        edit={edit}
        onConfirmRemove={removeConfirm}
      />
      <BodyEditor
        key={formatBlock(action.body)}
        path={[...path, 'body']}
        value={action.body}
        disabled={disabled}
        issue={issue([...path, 'body'])}
        edit={edit}
      />
    </fieldset>
  );
}

function StringMapEditor({
  label,
  path,
  values,
  disabled,
  issue,
  edit,
  onConfirmRemove,
}: {
  label: string;
  path: RecipePath;
  values: Values;
  disabled: boolean;
  issue: (path: RecipePath) => { id: string; message: string };
  edit: (operation: RecipeEdit) => void;
  onConfirmRemove: (path: RecipePath) => boolean;
}) {
  const { t } = useTranslation();
  return (
    <section
      id={issue(path).id}
      tabIndex={-1}
      aria-invalid={issue(path).message ? true : undefined}
      aria-describedby={issue(path).message ? issue(path).id + '-error' : undefined}
      className="space-y-2 rounded-sm border border-line-soft p-3"
    >
      <div className="flex items-center justify-between gap-2">
        <h4 className="text-xs font-semibold text-ink">{label}</h4>
        {!disabled && (
          <Button
            size="sm"
            onClick={() => {
              const key = uniqueName(values, 'new_key');
              edit({ type: 'set', path: [...path, key], value: '' });
            }}
          >
            {t('connectors.recipeBuilder.addEntry')}
          </Button>
        )}
      </div>
      {issue(path).message && <InlineIssue issue={issue(path)} />}
      {Object.entries(values).map(([key, value]) => {
        const entryPath = [...path, key] as RecipePath;
        const rename = (next: string) => {
          if (!next || next === key || values[next] !== undefined) return;
          edit({ type: 'rename', path: entryPath, to: next });
        };
        return (
          <div key={key} className="grid gap-2 sm:grid-cols-[1fr_2fr_auto]">
            <KeyField label={t('connectors.recipeBuilder.key')} path={[...entryPath, 'key']} value={key} disabled={disabled} onCommit={rename} />
            <TextField label={t('connectors.recipeBuilder.value')} path={entryPath} value={String(value ?? '')} disabled={disabled} issue={issue(entryPath)} onChange={(next) => edit({ type: 'set', path: entryPath, value: next })} />
            {!disabled && (
              <Button
                size="sm"
                variant="danger"
                className="self-end"
                onClick={() => {
                  if (onConfirmRemove(entryPath)) {
                    edit({ type: 'delete', path: Object.keys(values).length === 1 ? path : entryPath });
                  }
                }}
              >
                {t('connectors.recipeBuilder.removeEntry')}
              </Button>
            )}
          </div>
        );
      })}
      {Object.keys(values).length === 0 && <p className="text-xs text-ink-muted">{t('connectors.recipeBuilder.noEntries')}</p>}
    </section>
  );
}

function BodyEditor({
  path,
  value,
  disabled,
  issue,
  edit,
}: {
  path: RecipePath;
  value: unknown;
  disabled: boolean;
  issue: { id: string; message: string };
  edit: (operation: RecipeEdit) => void;
}) {
  const { t } = useTranslation();
  const valueText = formatBlock(value);
  const [draft, setDraft] = useState(valueText);
  const [parseMessage, setParseMessage] = useState('');
  const shown = draft;
  if (disabled) return <ReadOnlyOrField label={t('connectors.recipeBuilder.body')} value={valueText} />;
  return (
    <div>
      <label htmlFor={issue.id} className="mb-1 block text-xs font-medium text-ink">
        {t('connectors.recipeBuilder.body')}
      </label>
      <p className="mb-2 text-2xs text-ink-muted">{t('connectors.recipeBuilder.bodyHelp')}</p>
      <textarea
        id={issue.id}
        rows={5}
        className={controlClass}
        value={shown}
        aria-invalid={issue.message || parseMessage ? true : undefined}
        aria-describedby={issue.message ? issue.id + '-error' : parseMessage ? issue.id + '-parse-error' : undefined}
        onChange={(event) => {
          setDraft(event.currentTarget.value);
          setParseMessage('');
        }}
        onBlur={() => {
          if (shown === valueText) return;
          if (!shown.trim()) {
            edit({ type: 'delete', path });
            setDraft('');
            return;
          }
          try {
            edit({ type: 'block', path, text: shown });
            setParseMessage('');
          } catch (error) {
            setParseMessage(t('connectors.recipeBuilder.bodyParseError', {
              message: error instanceof Error ? error.message : '',
            }));
          }
        }}
      />
      {issue.message && <InlineIssue issue={issue} />}
      {parseMessage && <p id={issue.id + '-parse-error'} className="mt-1 text-2xs text-err" role="alert">{parseMessage}</p>}
    </div>
  );
}

function ScalarField({
  label,
  path,
  value,
  disabled,
  issue,
  onChange,
}: {
  label: string;
  path: RecipePath;
  value: unknown;
  disabled: boolean;
  issue?: { id: string; message: string };
  onChange: (value: unknown) => void;
}) {
  const { t } = useTranslation();
  if (disabled) return <ReadOnlyOrField label={label} value={formatScalar(value)} />;
  const id = issue?.id ?? recipeFieldId([...path]);
  const type = scalarType(value);
  const options = ['string', 'number', 'boolean', 'null'];
  const setType = (next: string) => {
    if (next === 'null') onChange(null);
    else if (next === 'string') onChange(value === null || value === undefined ? '' : String(value));
    else if (next === 'boolean') onChange(value === true || value === 'true');
    else if (next === 'number') {
      const numeric = Number(value);
      onChange(Number.isFinite(numeric) ? numeric : 0);
    }
  };
  return (
    <div className="min-w-0">
      <label htmlFor={id} className="mb-1 block text-xs font-medium text-ink">{label}</label>
      <div className="grid grid-cols-[minmax(7rem,auto)_1fr] gap-2">
        <select
          id={type === 'null' ? id : id + '-type'}
          aria-label={t('connectors.recipeBuilder.scalarType')}
          value={type}
          className={controlClass}
          onChange={(event) => setType(event.currentTarget.value)}
        >
          {options.map((option) => <option key={option} value={option}>{t('connectors.recipeBuilder.scalar' + capitalize(option))}</option>)}
        </select>
        {type === 'null' ? (
          <span className="flex items-center text-xs text-ink-muted">{t('connectors.recipeBuilder.scalarNull')}</span>
        ) : type === 'boolean' ? (
          <select id={id} value={value === true ? 'true' : 'false'} className={controlClass} aria-invalid={issue?.message ? true : undefined} aria-describedby={issue?.message ? id + '-error' : undefined} onChange={(event) => onChange(event.currentTarget.value === 'true')}>
            <option value="true">{t('connectors.recipeBuilder.true')}</option>
            <option value="false">{t('connectors.recipeBuilder.false')}</option>
          </select>
        ) : (
          <input
            id={id}
            type={type === 'number' ? 'number' : 'text'}
            className={controlClass}
            value={value === undefined || value === null ? '' : String(value)}
            aria-invalid={issue?.message ? true : undefined}
            aria-describedby={issue?.message ? id + '-error' : undefined}
            onChange={(event) => {
              if (type === 'number') {
                if (event.currentTarget.value !== '' && Number.isFinite(event.currentTarget.valueAsNumber)) onChange(event.currentTarget.valueAsNumber);
              } else onChange(event.currentTarget.value);
            }}
          />
        )}
      </div>
      {issue?.message && <InlineIssue issue={issue} />}
    </div>
  );
}

function TextField({
  label,
  path,
  value,
  disabled,
  issue,
  onChange,
  className = '',
  type = 'text',
}: {
  label: string;
  path: RecipePath;
  value: string;
  disabled: boolean;
  issue?: { id: string; message: string };
  onChange: (value: string) => void;
  className?: string;
  type?: 'text' | 'number';
}) {
  if (disabled) return <ReadOnlyOrField label={label} value={value} className={className} />;
  const id = issue?.id ?? recipeFieldId([...path]);
  return (
    <div className={className}>
      <label htmlFor={id} className="mb-1 block text-xs font-medium text-ink">{label}</label>
      <input
        id={id}
        type={type}
        className={controlClass}
        value={value}
        aria-invalid={issue?.message ? true : undefined}
        aria-describedby={issue?.message ? id + '-error' : undefined}
        onChange={(event) => onChange(event.currentTarget.value)}
      />
      {issue?.message && <InlineIssue issue={issue} />}
    </div>
  );
}

type KeyFieldProps = {
  label: string;
  path: RecipePath;
  value: string;
  disabled: boolean;
  onCommit: (value: string) => void;
};

// Keyed by the stored name so the draft restarts when the key is renamed from elsewhere.
function KeyField(props: KeyFieldProps) {
  return <KeyFieldInput key={props.value} {...props} />;
}

function KeyFieldInput({ label, path, value, disabled, onCommit }: KeyFieldProps) {
  const [draft, setDraft] = useState(value);
  if (disabled) return <ReadOnlyOrField label={label} value={value} />;
  const id = recipeFieldId([...path]);
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs font-medium text-ink">{label}</label>
      <input
        id={id}
        type="text"
        className={controlClass}
        value={draft}
        onChange={(event) => setDraft(event.currentTarget.value)}
        onBlur={() => onCommit(draft)}
      />
    </div>
  );
}

function SelectField({
  label,
  path,
  value,
  options,
  disabled,
  issue,
  onChange,
  optionLabels,
  includeEmpty = false,
  emptyLabel,
}: {
  label: string;
  path: RecipePath;
  value: string;
  options: readonly string[];
  disabled: boolean;
  issue?: { id: string; message: string };
  onChange: (value: string) => void;
  optionLabels?: Record<string, string>;
  includeEmpty?: boolean;
  emptyLabel?: string;
}) {
  const { t } = useTranslation();
  const selected = value;
  if (disabled) return <ReadOnlyOrField label={label} value={optionLabels?.[selected] ?? selected} />;
  const id = issue?.id ?? recipeFieldId([...path]);
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-xs font-medium text-ink">{label}</label>
      <select
        id={id}
        className={controlClass}
        value={selected}
        aria-invalid={issue?.message ? true : undefined}
        aria-describedby={issue?.message ? id + '-error' : undefined}
        onChange={(event) => onChange(event.currentTarget.value)}
      >
        {includeEmpty && <option value="">{emptyLabel ?? t('connectors.recipeBuilder.none')}</option>}
        {selected && !options.includes(selected) && <option value={selected}>{selected}</option>}
        {options.map((option) => <option key={option} value={option}>{optionLabels?.[option] ?? option}</option>)}
      </select>
      {issue?.message && <InlineIssue issue={issue} />}
    </div>
  );
}

function CheckboxField({
  label,
  path,
  checked,
  disabled,
  issue,
  onChange,
}: {
  label: string;
  path: RecipePath;
  checked: boolean;
  disabled: boolean;
  issue?: { id: string; message: string };
  onChange: (checked: boolean) => void;
}) {
  const { t } = useTranslation();
  if (disabled) return <ReadOnlyOrField label={label} value={checked ? t('connectors.recipeBuilder.true') : t('connectors.recipeBuilder.false')} />;
  const id = issue?.id ?? recipeFieldId([...path]);
  return (
    <div>
      <label htmlFor={id} className="inline-flex items-center gap-2 text-xs font-medium text-ink">
        <input id={id} type="checkbox" checked={checked} className="accent-[var(--color-accent-primary)]" aria-invalid={issue?.message ? true : undefined} aria-describedby={issue?.message ? id + '-error' : undefined} onChange={(event) => onChange(event.currentTarget.checked)} />
        {label}
      </label>
      {issue?.message && <InlineIssue issue={issue} />}
    </div>
  );
}

function ReadOnlyOrField({ label, value, className = '' }: { label: string; value: string; className?: string }) {
  return (
    <div className={className}>
      <span className="mb-1 block text-xs font-medium text-ink">{label}</span>
      <p className="min-h-9 whitespace-pre-wrap break-words rounded-sm bg-canvas-sunken px-3 py-2 text-xs text-ink-muted">{value || '—'}</p>
    </div>
  );
}

function InlineIssue({ issue }: { issue: { id: string; message: string } }) {
  return <p id={issue.id + '-error'} className="mt-1 text-2xs text-err" role="alert">{issue.message}</p>;
}

function UnknownMarker({ keys }: { keys: string[] }) {
  const { t } = useTranslation();
  return (
    <span
      className="inline-flex rounded-sm bg-warn/10 px-2 py-1 text-2xs text-warn"
      title={keys.join(', ')}
    >
      {t('connectors.recipeBuilder.yamlOnly', { keys: keys.join(', ') })}
    </span>
  );
}

const controlClass =
  'min-h-9 w-full rounded-sm border border-line-strong bg-canvas px-3 py-2 text-xs text-ink outline-none focus:border-accent-primary disabled:cursor-not-allowed disabled:opacity-60';

function startRecipe(value: string): string {
  const source = value.startsWith('\uFEFF') ? value.slice(1) : value;
  const started = editRecipeDocument(source, {
    type: 'set',
    path: [],
    value: { version: 1, category: 'other', auth: { mode: 'none' }, endpoints: [] },
  });
  return value.startsWith('\uFEFF') ? '\uFEFF' + started : started;
}

function newEndpoint(endpoints: unknown[]): Values {
  const name = uniqueName(Object.fromEntries(endpoints.map((value) => [String(record(value).name ?? ''), true])), 'endpoint');
  return {
    name,
    path: '',
    method: 'GET',
    items: '',
    entity: { kind: '', name: '', external_id: '' },
  };
}

function uniqueName(values: Values, base: string): string {
  if (!(base in values)) return base;
  let index = 2;
  while ((base + '_' + index) in values) index += 1;
  return base + '_' + index;
}

function samePath(left: RecipePath, right: RecipePath): boolean {
  return left.length === right.length && left.every((part, index) => part === right[index]);
}

function isPrefix(prefix: RecipePath, path: RecipePath): boolean {
  return prefix.length <= path.length && prefix.every((part, index) => part === path[index]);
}

function valueAtPath(value: unknown, path: DocumentPath): unknown {
  const parts: (string | number)[] = typeof path === 'string'
    ? Array.from(path.matchAll(/(?:^|\.)([^.[\]]+)|\[(\d+)\]/g), (match) => match[2] === undefined ? match[1] : Number(match[2]))
    : [...path];
  return parts.reduce<unknown>((current, part) => {
    if (!current || typeof current !== 'object') return undefined;
    return (current as Values)[String(part)];
  }, value);
}

function record(value: unknown): Values {
  return value !== null && typeof value === 'object' && !Array.isArray(value) ? value as Values : {};
}

function list(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}

function formatBlock(value: unknown): string {
  if (value === undefined) return '';
  const json = JSON.stringify(value, null, 2);
  return json ?? String(value);
}

function formatScalar(value: unknown): string {
  if (value === undefined) return '';
  if (value === null) return 'null';
  if (typeof value === 'string') return value;
  return String(value);
}

function scalarType(value: unknown): string {
  if (value === null) return 'null';
  if (typeof value === 'number') return 'number';
  if (typeof value === 'boolean') return 'boolean';
  return 'string';
}

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}
