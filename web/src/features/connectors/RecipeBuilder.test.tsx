import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fireEvent, render, screen, within } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { RecipeBuilder } from './RecipeBuilder';
import { parseRecipeDocument } from './recipeDocument';

function Harness({
  initial = '',
  disabled = false,
}: {
  initial?: string;
  disabled?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <RecipeBuilder value={value} onChange={setValue} disabled={disabled} />
      <output data-testid="recipe">{value}</output>
    </>
  );
}

function renderBuilder(initial = '', disabled = false) {
  return render(<Harness initial={initial} disabled={disabled} />);
}

function recipeText() {
  return screen.getByTestId('recipe').textContent ?? '';
}

function recipeValue() {
  return parseRecipeDocument(recipeText()).document?.toJS() as Record<string, unknown>;
}

function change(label: string, value: string, index = 0) {
  fireEvent.change(screen.getAllByLabelText(label)[index], { target: { value } });
}

function changeWithin(container: HTMLElement, label: string, value: string, index = 0) {
  fireEvent.change(within(container).getAllByLabelText(label)[index], { target: { value } });
}

function rowByLegend(label: string): HTMLElement {
  return screen.getByText(label, { selector: 'legend' }).closest('fieldset') as HTMLElement;
}

function rename(label: string, value: string, index = 0) {
  const input = screen.getAllByLabelText(label)[index];
  fireEvent.change(input, { target: { value } });
  fireEvent.blur(input);
}

function renameWithin(container: HTMLElement, label: string, value: string) {
  const input = within(container).getByLabelText(label);
  fireEvent.change(input, { target: { value } });
  fireEvent.blur(input);
}

const baseRecipe = [
  'version: 1',
  'category: media',
  'auth:',
  '  mode: none',
  'endpoints:',
  '  - name: items',
  '    path: /api/items',
  '    method: GET',
  '    items: items',
  '    entity:',
  '      kind: media_item',
  '      name: name',
  '      external_id: id',
  '',
].join('\n');

const repositoryRoot = resolve(process.cwd(), '..');
const sonarrRecipe = readFileSync(`${repositoryRoot}/docs/connectors/recipes/sonarr.yaml`, 'utf8');

describe('RecipeBuilder', () => {
  it('does not write an empty recipe until accepted, then builds the documented Sonarr shape through rows', () => {
    renderBuilder();
    expect(recipeText()).toBe('');

    fireEvent.click(screen.getByRole('button', { name: 'Start recipe' }));
    change('Category', 'media');
    change('Authentication mode', 'header');
    change('Authentication name', 'X-Api-Key');
    fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }));
    change('Endpoint name', 'series');
    change('Path', '/api/v3/series');
    change('Items path', '@this');

    change('Pagination style', 'offset');
    change('Parameter', 'page');
    change('Size parameter', 'pageSize');
    change('Page size', '100');
    change('Kind', 'media_series');
    change('Name mapping', 'title');
    change('External ID', 'id');

    fireEvent.click(screen.getByRole('button', { name: 'Add attribute' }));
    renameWithin(rowByLegend('new_attribute'), 'Attribute name', 'status');
    changeWithin(rowByLegend('status'), 'Path', 'status');
    changeWithin(rowByLegend('status'), 'Type', '');

    fireEvent.click(screen.getByRole('button', { name: 'Add map entry' }));
    rename('Key', 'continuing');
    change('Value', 'active');
    fireEvent.click(screen.getByRole('button', { name: 'Add map entry' }));
    rename('Key', 'ended', 1);
    change('Value', 'ended', 1);

    fireEvent.click(screen.getByRole('button', { name: 'Add attribute' }));
    renameWithin(rowByLegend('new_attribute'), 'Attribute name', 'monitored');
    changeWithin(rowByLegend('monitored'), 'Path', 'monitored');
    changeWithin(rowByLegend('monitored'), 'Type', 'bool');

    fireEvent.click(screen.getByRole('button', { name: 'Add attribute' }));
    renameWithin(rowByLegend('new_attribute'), 'Attribute name', 'year');
    changeWithin(rowByLegend('year'), 'Path', 'year');
    changeWithin(rowByLegend('year'), 'Type', 'number');

    fireEvent.click(screen.getAllByRole('button', { name: 'Add dependency' })[0]);
    change('Dependency kind', 'storage');
    changeWithin(rowByLegend('Dependency 1'), 'Path', '#.path');

    const expected = parseRecipeDocument(sonarrRecipe).document?.toJS() as {
      endpoints: Array<Record<string, unknown>>;
    };
    expected.endpoints[0].pagination = {
      type: 'offset',
      param: 'page',
      size_param: 'pageSize',
      size: 100,
    };
    expect(recipeValue()).toEqual(expected);
    const previewInput = {
      url: 'https://sonarr.example',
      verifyTls: true,
      config: { recipe: recipeText() },
    };
    expect(previewInput.config.recipe).toBe(recipeText());
  });

  it.each([
    ['page', ['Parameter', 'Size parameter', 'Page size', 'Start'], ['Cursor path', 'Next link path']],
    ['offset', ['Parameter', 'Size parameter', 'Page size', 'Start'], ['Cursor path', 'Next link path']],
    ['cursor', ['Parameter', 'Cursor path', 'Size parameter', 'Page size'], ['Start', 'Next link path']],
    ['next_link', ['Use Link header', 'Next link path'], ['Page size', 'Cursor path', 'Start']],
  ])('shows only the %s pagination fields', (style, visible, hidden) => {
    const initial = baseRecipe.replace(
      '    entity:\n',
      '    pagination:\n      type: offset\n      param: offset\n      size_param: limit\n      size: 20\n      start: 4\n      cursor_path: next\n      next_path: paging.next\n    entity:\n',
    );
    renderBuilder(initial);
    change('Pagination style', style);
    for (const label of visible) expect(screen.getByLabelText(label)).toBeInTheDocument();
    for (const label of hidden) expect(screen.queryByLabelText(label)).not.toBeInTheDocument();
  });

  it('keeps pagination values that the new style still allows when switching from page to offset', () => {
    const initial = baseRecipe.replace(
      '    entity:\n',
      '    pagination:\n      type: page\n      param: page\n      size_param: pageSize\n      size: 50\n      start: 1\n    entity:\n',
    );
    renderBuilder(initial);
    change('Pagination style', 'offset');
    expect(recipeValue().endpoints).toMatchObject([{
      pagination: { type: 'offset', param: 'page', size_param: 'pageSize', size: 50, start: 1 },
    }]);
    expect(screen.getByLabelText('Parameter')).toHaveValue('page');
    expect(screen.getByLabelText('Start')).toHaveValue(1);
  });

  it('drops start but keeps param, size_param and size when switching from offset to cursor', () => {
    const initial = baseRecipe.replace(
      '    entity:\n',
      '    pagination:\n      type: offset\n      param: offset\n      size_param: limit\n      size: 20\n      start: 4\n    entity:\n',
    );
    renderBuilder(initial);
    change('Pagination style', 'cursor');
    expect(recipeValue().endpoints).toMatchObject([{
      pagination: { type: 'cursor', param: 'offset', size_param: 'limit', size: 20 },
    }]);
    expect((recipeValue().endpoints as Array<{ pagination: Record<string, unknown> }>)[0].pagination).not.toHaveProperty('start');
    expect(screen.queryByLabelText('Start')).not.toBeInTheDocument();
  });

  it('drops every field the next_link style does not allow when switching from page', () => {
    const initial = baseRecipe.replace(
      '    entity:\n',
      '    pagination:\n      type: page\n      param: page\n      size_param: pageSize\n      size: 50\n      start: 1\n    entity:\n',
    );
    renderBuilder(initial);
    change('Pagination style', 'next_link');
    expect((recipeValue().endpoints as Array<{ pagination: unknown }>)[0].pagination).toEqual({ type: 'next_link' });
  });

  it('edits request maps and body, then adds, reorders and removes only the selected endpoint', () => {
    renderBuilder(baseRecipe);
    const query = screen.getByText('Query parameters').closest('section');
    const headers = screen.getByText('Headers').closest('section');
    expect(query).not.toBeNull();
    expect(headers).not.toBeNull();

    fireEvent.click(within(query as HTMLElement).getByRole('button', { name: 'Add entry' }));
    rename('Key', 'include');
    change('Value', 'series');
    fireEvent.click(within(headers as HTMLElement).getByRole('button', { name: 'Add entry' }));
    rename('Key', 'X-Mode', 1);
    change('Value', 'full', 1);

    const body = screen.getByLabelText('Request body');
    fireEvent.change(body, { target: { value: '{\"query\":\"series\"}' } });
    fireEvent.blur(body);
    expect(recipeText()).toContain('body:');
    expect(recipeValue()).toMatchObject({
      endpoints: [{ query: { include: 'series' }, headers: { 'X-Mode': 'full' }, body: { query: 'series' } }],
    });
    fireEvent.click(within(query as HTMLElement).getByRole('button', { name: 'Remove entry' }));
    fireEvent.click(within(headers as HTMLElement).getByRole('button', { name: 'Remove entry' }));
    expect((recipeValue().endpoints as Array<Record<string, unknown>>)[0].query).toBeUndefined();
    expect((recipeValue().endpoints as Array<Record<string, unknown>>)[0].headers).toBeUndefined();

    fireEvent.click(screen.getByRole('button', { name: 'Add endpoint' }));
    expect((recipeValue().endpoints as unknown[])).toHaveLength(2);
    change('Method', 'POST');
    fireEvent.click(screen.getAllByRole('button', { name: 'Move endpoint down' })[0]);
    expect((recipeValue().endpoints as Array<{ name: string }>)[1].name).toBe('items');
    fireEvent.click(screen.getAllByRole('button', { name: 'Remove endpoint' })[0]);
    expect((recipeValue().endpoints as unknown[])).toHaveLength(1);
    expect((recipeValue().endpoints as Array<{ name: string }>)[0].name).toBe('items');
  });

  it('keeps unknown endpoint and entity keys during edits and confirms before removing their row', () => {
    const initial = baseRecipe.replace('      external_id: id\n', '      external_id: id\n      entity_extra:\n        keep: true\n') + [
      '    jobs:',
      '      - name: restart',
      '        method: POST',
      '        path: /restart',
      '',
    ].join('\n');
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderBuilder(initial);

    expect(screen.getByTitle('jobs')).toBeInTheDocument();
    expect(screen.getByTitle('entity_extra')).toBeInTheDocument();
    change('Path', '/api/changed');
    expect(recipeText()).toContain('jobs:');
    expect(recipeText()).toContain('entity_extra:');
    expect(recipeText()).toContain('path: /restart');

    fireEvent.click(screen.getByRole('button', { name: 'Remove endpoint' }));
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(recipeText()).toContain('jobs:');

    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: 'Remove endpoint' }));
    expect(recipeValue().endpoints).toEqual([]);
    confirm.mockRestore();
  });

  it('keeps scalar types for constants, value maps and defaults', () => {
    const initial = baseRecipe.replace('      external_id: id\n', [
      '      external_id: id',
      '      attributes:',
      '        code:',
      '          const: 7',
      '          type: number',
      '          map:',
      '            enabled: true',
      '          default: false',
      '',
    ].join('\n'));
    renderBuilder(initial);
    expect(screen.getAllByLabelText('Value type').map((field) => (field as HTMLSelectElement).value)).toEqual([
      'number',
      'boolean',
      'boolean',
    ]);
    change('Constant', '9');
    change('Value type', 'string', 1);
    change('Value type', 'number', 2);

    expect(recipeValue()).toMatchObject({
      endpoints: [{
        entity: {
          attributes: {
            code: { const: 9, map: { enabled: 'true' }, default: 0 },
          },
        },
      }],
    });
    expect(recipeText()).toContain('const: 9');
    expect(recipeText()).toContain('enabled: \"true\"');
    expect(recipeText()).toContain('default: 0');
  });

  it('shows authentication fields for the selected mode and removes only fields the mode no longer uses', () => {
    renderBuilder(baseRecipe);
    change('Category', 'dns');
    change('Authentication mode', 'header');
    change('Authentication name', 'Authorization');
    change('Token prefix', 'Bearer ');
    expect(recipeValue().category).toBe('dns');
    expect(recipeValue().auth).toEqual({ mode: 'header', name: 'Authorization', prefix: 'Bearer ' });

    change('Authentication mode', 'query');
    expect(screen.getByLabelText('Authentication name')).toBeInTheDocument();
    expect(screen.queryByLabelText('Token prefix')).not.toBeInTheDocument();
    expect(recipeValue().auth).toEqual({ mode: 'query', name: 'Authorization' });

    change('Authentication mode', 'basic');
    expect(screen.queryByLabelText('Authentication name')).not.toBeInTheDocument();
    expect(recipeValue().auth).toEqual({ mode: 'basic' });

    change('Authentication mode', 'none');
    expect(recipeValue().auth).toEqual({ mode: 'none' });
  });

  it('marks unknown auth keys and keeps them when the authentication name changes', () => {
    const initial = baseRecipe.replace('  mode: none', '  mode: header\n  name: X-Api-Key\n  foo: bar');
    renderBuilder(initial);
    expect(screen.getByTitle('foo')).toBeInTheDocument();
    expect(screen.getByText(/More content editable in YAML only: foo/)).toBeInTheDocument();
    change('Authentication name', 'X-Other');
    expect(recipeText()).toContain('foo: bar');
    expect(recipeValue().auth).toEqual({ mode: 'header', name: 'X-Other', foo: 'bar' });
  });

  it('edits every entity mapping and attribute source, type, map entry, default and row', () => {
    renderBuilder(baseRecipe);
    change('Kind', 'asset');
    change('Name mapping', 'displayName');
    change('External ID', 'assetId');
    change('Hostname', 'host.name');
    change('IP address', 'network.ip');
    change('MAC address', 'network.mac');
    change('Aliases', 'name, displayName');
    fireEvent.click(screen.getByRole('button', { name: 'Add attribute' }));
    rename('Attribute name', 'status');
    changeWithin(rowByLegend('status'), 'Path', 'state');
    changeWithin(rowByLegend('status'), 'Type', 'string');
    fireEvent.click(screen.getByRole('button', { name: 'Add map entry' }));
    rename('Key', 'ready');
    change('Value', 'available');
    fireEvent.click(screen.getByRole('button', { name: 'Add map entry' }));
    rename('Key', 'stopped', 1);
    change('Value', 'offline', 1);
    fireEvent.click(screen.getByRole('button', { name: 'Add default' }));
    change('Default', 'unknown');

    expect(recipeValue().endpoints).toMatchObject([{
      entity: {
        kind: 'asset',
        name: 'displayName',
        external_id: 'assetId',
        hostname: 'host.name',
        ip: 'network.ip',
        mac: 'network.mac',
        aliases: 'name, displayName',
        attributes: { status: { path: 'state', type: 'string', map: { ready: 'available', stopped: 'offline' }, default: 'unknown' } },
      },
    }]);

    fireEvent.click(screen.getAllByRole('button', { name: 'Remove entry' })[0]);
    expect(recipeValue().endpoints).toMatchObject([{ entity: { attributes: { status: { map: { stopped: 'offline' } } } } }]);
    fireEvent.click(screen.getByRole('button', { name: 'Remove attribute' }));
    expect(recipeValue().endpoints).toMatchObject([{ entity: { attributes: {} } }]);
  });

  it('edits root and endpoint dependencies with path or constant sources and removes selected entries', () => {
    renderBuilder(baseRecipe);
    const recipeDependencies = screen.getByText('Recipe dependencies').closest('section') as HTMLElement;
    const endpointDependencies = screen.getByText('Endpoint dependencies').closest('section') as HTMLElement;

    fireEvent.click(within(recipeDependencies).getByRole('button', { name: 'Add dependency' }));
    changeWithin(recipeDependencies, 'Dependency kind', 'network');
    changeWithin(recipeDependencies, 'Path', 'network.gateway');
    fireEvent.click(within(recipeDependencies).getByRole('button', { name: 'Add dependency' }));
    const secondDependency = rowByLegend('Dependency 2');
    changeWithin(secondDependency, 'Dependency kind', 'upstream_service');
    changeWithin(secondDependency, 'Dependency source', 'const');
    expect(secondDependency).toHaveTextContent('Constant');
    changeWithin(secondDependency, 'Constant', 'dns');

    fireEvent.click(within(endpointDependencies).getByRole('button', { name: 'Add dependency' }));
    changeWithin(endpointDependencies, 'Dependency kind', 'storage');
    changeWithin(endpointDependencies, 'Path', '#.path');
    expect(recipeValue()).toMatchObject({
      dependencies: [
        { kind: 'network', path: 'network.gateway' },
        { kind: 'upstream_service', const: 'dns' },
      ],
      endpoints: [{ dependencies: [{ kind: 'storage', path: '#.path' }] }],
    });

    fireEvent.click(within(recipeDependencies).getAllByRole('button', { name: 'Remove dependency' })[0]);
    fireEvent.click(within(endpointDependencies).getByRole('button', { name: 'Remove dependency' }));
    expect(recipeValue()).toMatchObject({ dependencies: [{ kind: 'upstream_service', const: 'dns' }], endpoints: [{ dependencies: [] }] });
  });

  it('marks root, endpoint, entity and attribute unknown keys, preserving them through row edits and rename', () => {
    const initial = [
      'root_extra: {keep: true}',
      ...baseRecipe.trimEnd().split('\n'),
      '      attributes:',
      '        custom:',
      '          path: custom.path',
      '          custom_attr: { quoted: "keep me" } # keep comment',
      '      entity_extra: true',
      '    jobs:',
      '      - name: restart',
      '        method: POST',
      '        path: /restart',
      '',
    ].join('\n');
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
    renderBuilder(initial);

    for (const key of ['root_extra', 'jobs', 'entity_extra', 'custom_attr']) {
      expect(screen.getByTitle(key)).toBeInTheDocument();
    }
    change('Path', '/api/changed');
    expect(recipeText()).toContain('root_extra: {keep: true}');
    expect(recipeText()).toContain('custom_attr: { quoted: "keep me" } # keep comment');
    expect(recipeText()).toContain('jobs:');

    rename('Attribute name', 'renamed');
    expect(recipeText()).toContain('renamed:\n          path: custom.path\n          custom_attr: { quoted: "keep me" } # keep comment');

    fireEvent.click(screen.getByRole('button', { name: 'Remove attribute' }));
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(recipeText()).toContain('custom_attr:');
    confirm.mockReturnValue(true);
    fireEvent.click(screen.getByRole('button', { name: 'Remove attribute' }));
    expect(recipeText()).not.toContain('custom_attr:');
    expect(recipeText()).toContain('jobs:');
    confirm.mockRestore();
  });

  it('renders values without controls when read-only', () => {
    renderBuilder(baseRecipe, true);
    expect(screen.getByText('/api/items')).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
  });

  describe('hand-written text', () => {
    const jellyfinRecipe = readFileSync(`${repositoryRoot}/docs/connectors/recipes/jellyfin.yaml`, 'utf8');
    const commented = [
      '# my media server',
      'version: 1',
      'category: media   # shown on the dashboard',
      '',
      'auth: {mode: header, name: X-Api-Key}',
      'endpoints:',
      '  # the main list',
      '  - name: items',
      "    path: '/api/items'",
      '    method: GET',
      '    items: items',
      '    entity: {kind: media_item, name: name, external_id: id}',
      '',
    ].join('\r\n');

    it.each([['sonarr', sonarrRecipe], ['jellyfin', jellyfinRecipe], ['commented CRLF', commented]])(
      'never writes text when %s is only opened, focused and left',
      (_name, text) => {
        const onChange = vi.fn();
        render(<RecipeBuilder value={text} onChange={onChange} />);
        for (const control of screen.getAllByRole('textbox')) {
          fireEvent.focus(control);
          fireEvent.blur(control);
        }
        expect(onChange).not.toHaveBeenCalled();
      },
    );

    it('turns one field edit into a one-line change and keeps comments, quoting and line endings', () => {
      renderBuilder(commented);
      change('Path', '/api/new');
      const before = commented.split('\r\n');
      const after = recipeText().split('\r\n');
      expect(after).toHaveLength(before.length);
      expect(after.filter((line, index) => line !== before[index])).toEqual(["    path: /api/new"]);
    });
  });

  describe('action rows', () => {
    const entitySection = () => screen.getByText('Entity mapping', { selector: 'h4' }).closest('section') as HTMLElement;
    const serviceSection = () => screen.getByText('Service actions', { selector: 'h4' }).closest('section') as HTMLElement;
    const sectionOf = (container: HTMLElement, label: string) =>
      within(container).getByText(label, { selector: 'h4' }).closest('section') as HTMLElement;
    const withServiceActions = (actions: string[]) => baseRecipe + ['actions:', ...actions, ''].join('\n');

    it('adds an entity action through the form and keeps block style, quoting and the rest of the text', () => {
      renderBuilder(baseRecipe);
      fireEvent.click(within(entitySection()).getByRole('button', { name: 'Add action' }));
      expect(recipeValue().endpoints).toMatchObject([{ entity: { actions: { restart: { method: 'POST', path: '' } } } }]);

      changeWithin(rowByLegend('restart'), 'Action path', '/api/containers/{external_id}/restart');
      expect(recipeText()).toContain('      actions:\n        restart:\n          method: POST\n          path: /api/containers/{external_id}/restart\n');
      expect(recipeText().startsWith(baseRecipe)).toBe(true);
      expect(recipeValue().endpoints).toMatchObject([{
        entity: { kind: 'media_item', external_id: 'id', actions: { restart: { method: 'POST', path: '/api/containers/{external_id}/restart' } } },
      }]);
    });

    it('adds a service action at the root of a recipe without actions', () => {
      renderBuilder(baseRecipe);
      fireEvent.click(within(serviceSection()).getByRole('button', { name: 'Add action' }));
      changeWithin(rowByLegend('restart'), 'Action path', '/api/system/restart');
      changeWithin(rowByLegend('restart'), 'Action label', 'Restart service');
      changeWithin(rowByLegend('restart'), 'Downtime seconds', '30');

      expect(recipeText()).toContain('\nactions:\n  restart:\n    method: POST\n    path: /api/system/restart\n    label: Restart service\n    downtime_seconds: 30\n');
      expect(recipeText().startsWith(baseRecipe)).toBe(true);
      expect(recipeValue()).toMatchObject({
        actions: { restart: { method: 'POST', path: '/api/system/restart', label: 'Restart service', downtime_seconds: 30 } },
      });
    });

    it('names new actions restart, start and stop before a generated name', () => {
      renderBuilder(baseRecipe);
      for (let added = 0; added < 4; added += 1) {
        fireEvent.click(within(serviceSection()).getByRole('button', { name: 'Add action' }));
      }
      expect(Object.keys(recipeValue().actions as object)).toEqual(['restart', 'start', 'stop', 'action']);
    });

    it('keeps flow style when a root action path with braces is written, quoting it and parsing back to the same value', () => {
      renderBuilder(baseRecipe + 'actions: {rescan: {method: POST, path: /a}}\n');
      changeWithin(rowByLegend('rescan'), 'Action path', '/items/{external_id}/rescan');
      expect(recipeText()).toContain('path: "/items/{external_id}/rescan"');
      expect(recipeText().startsWith(baseRecipe)).toBe(true);
      expect(recipeValue()).toEqual({
        ...parseRecipeDocument(baseRecipe).document?.toJS() as object,
        actions: { rescan: { method: 'POST', path: '/items/{external_id}/rescan' } },
      });
    });

    it('keeps flow style when an entity action path with braces is written inside a flow entity', () => {
      const initial = [
        'version: 1',
        'category: media',
        'auth: {mode: none}',
        'endpoints:',
        '  - name: items',
        '    path: /api/items',
        '    method: GET',
        '    items: items',
        '    entity: {kind: media_item, name: name, external_id: id, actions: {rescan: {method: POST, path: /a}}}',
        '',
      ].join('\n');
      renderBuilder(initial);
      changeWithin(rowByLegend('rescan'), 'Action path', '/items/{external_id}/rescan');
      expect(recipeText()).toContain('path: "/items/{external_id}/rescan"');
      expect(recipeValue().endpoints).toEqual([{
        name: 'items',
        path: '/api/items',
        method: 'GET',
        items: 'items',
        entity: { kind: 'media_item', name: 'name', external_id: 'id', actions: { rescan: { method: 'POST', path: '/items/{external_id}/rescan' } } },
      }]);
    });

    it('edits method, label, description, downtime, query, headers and body of an action', () => {
      renderBuilder(withServiceActions(['  restart:', '    method: POST', '    path: /api/system/restart']));
      const row = rowByLegend('restart');

      changeWithin(row, 'Action method', 'PUT');
      changeWithin(row, 'Action label', 'Restart now');
      changeWithin(row, 'Action description', 'Restarts the service.');
      changeWithin(row, 'Downtime seconds', '45');
      expect(recipeValue().actions).toEqual({
        restart: { method: 'PUT', path: '/api/system/restart', label: 'Restart now', description: 'Restarts the service.', downtime_seconds: 45 },
      });

      changeWithin(row, 'Downtime seconds', '');
      changeWithin(row, 'Action label', '');
      expect(recipeValue().actions).toEqual({
        restart: { method: 'PUT', path: '/api/system/restart', description: 'Restarts the service.' },
      });

      const query = sectionOf(row, 'Query parameters');
      fireEvent.click(within(query).getByRole('button', { name: 'Add entry' }));
      renameWithin(query, 'Key', 'source');
      changeWithin(query, 'Value', 'operator');
      const headers = sectionOf(row, 'Headers');
      fireEvent.click(within(headers).getByRole('button', { name: 'Add entry' }));
      renameWithin(headers, 'Key', 'X-Mode');
      changeWithin(headers, 'Value', 'full');
      expect(recipeValue().actions).toMatchObject({ restart: { query: { source: 'operator' }, headers: { 'X-Mode': 'full' } } });

      const body = within(row).getByLabelText('Request body');
      fireEvent.change(body, { target: { value: '{"reason":"ops"}' } });
      fireEvent.blur(body);
      expect(recipeValue().actions).toMatchObject({ restart: { body: { reason: 'ops' } } });

      fireEvent.click(within(query).getByRole('button', { name: 'Remove entry' }));
      expect((recipeValue().actions as { restart: Record<string, unknown> }).restart.query).toBeUndefined();
    });

    it('renames an action, ignores a duplicate name and removes an action', () => {
      renderBuilder(withServiceActions([
        '  restart:', '    method: POST', '    path: /restart',
        '  start:', '    method: POST', '    path: /start',
      ]));
      renameWithin(rowByLegend('restart'), 'Action name', 'stop');
      expect(recipeValue().actions).toEqual({
        stop: { method: 'POST', path: '/restart' },
        start: { method: 'POST', path: '/start' },
      });

      renameWithin(rowByLegend('stop'), 'Action name', 'start');
      expect(recipeValue().actions).toHaveProperty('stop');

      fireEvent.click(within(rowByLegend('start')).getByRole('button', { name: 'Remove action' }));
      expect(recipeValue().actions).toEqual({ stop: { method: 'POST', path: '/restart' } });
    });

    it('disables adding at ten actions and shows the limit hint', () => {
      const names = Array.from({ length: 10 }, (_, index) => `action${index + 1}`);
      renderBuilder(withServiceActions(names.flatMap((name) => [`  ${name}:`, '    method: POST', `    path: /${name}`])));
      expect(within(serviceSection()).getByRole('button', { name: 'Add action' })).toBeDisabled();
      expect(screen.getByText('An action list can hold at most 10 actions.')).toBeInTheDocument();
    });

    it('marks an unknown key inside an action, keeps it through edits and confirms before removing the row', () => {
      const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
      renderBuilder(withServiceActions(['  restart:', '    method: POST', '    path: /restart', '    retry: 3']));
      expect(screen.getByTitle('retry')).toBeInTheDocument();
      expect(screen.getByText(/editable in YAML only: retry/)).toBeInTheDocument();
      expect(screen.queryByTitle('actions')).not.toBeInTheDocument();

      changeWithin(rowByLegend('restart'), 'Action path', '/api/restart');
      expect(recipeText()).toContain('    retry: 3');

      fireEvent.click(within(rowByLegend('restart')).getByRole('button', { name: 'Remove action' }));
      expect(confirm).toHaveBeenCalledTimes(1);
      expect(recipeText()).toContain('retry: 3');
      confirm.mockRestore();
    });

    it('shows no unknown-key marker for known root and entity action containers', () => {
      const initial = baseRecipe.replace('      external_id: id\n', '      external_id: id\n      actions:\n        rescan:\n          method: POST\n          path: /rescan\n')
        + ['actions:', '  restart:', '    method: POST', '    path: /restart', '    retry: 3', ''].join('\n');
      renderBuilder(initial);
      expect(screen.getAllByText(/editable in YAML only/)).toHaveLength(1);
      expect(screen.queryByTitle('actions')).not.toBeInTheDocument();
    });

    it('shows values without controls when read-only', () => {
      renderBuilder(baseRecipe.replace('      external_id: id\n', '      external_id: id\n      actions:\n        rescan:\n          method: POST\n          path: /rescan\n')
        + ['actions:', '  restart:', '    method: POST', '    path: /restart', ''].join('\n'), true);
      expect(screen.getByText('Entity actions')).toBeInTheDocument();
      expect(screen.getByText('Service actions')).toBeInTheDocument();
      expect(screen.getByText('/restart')).toBeInTheDocument();
      expect(screen.getByText('/rescan')).toBeInTheDocument();
      expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
      expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('marks located server errors on the matching action fields, rows and containers', () => {
      const initial = withServiceActions(['  restart:', '    method: POST', '    path: /restart'])
        .replace('      external_id: id\n', [
          '      external_id: id',
          '      actions:',
          '        rescan:',
          '          method: POST',
          '          path: /a',
          '          label: Rescan',
          '          downtime_seconds: 0',
          '          query: {source: operator}',
          '          headers: {X-Mode: full}',
          '          body: {reason: ok}',
          '',
        ].join('\n'));
      const errors = [
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.method', msg: 'Entity method rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.path', msg: 'Entity path rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.label', msg: 'Entity label rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.downtime_seconds', msg: 'Entity downtime rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.query.source', msg: 'Entity query rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.headers.X-Mode', msg: 'Entity header rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan.body.reason', msg: 'Entity body rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions.rescan', msg: 'Entity action rejected' },
        { field: 'config.recipe.endpoints[0].entity.actions', msg: 'Entity actions rejected' },
        { field: 'config.recipe.actions.restart.method', msg: 'Service method rejected' },
        { field: 'config.recipe.actions', msg: 'Service actions rejected' },
      ];
      render(<RecipeBuilder value={initial} onChange={vi.fn()} errors={errors} errorRecipe={initial} />);

      const rescan = rowByLegend('rescan');
      const entityActions = sectionOf(entitySection(), 'Entity actions');
      const queryField = within(sectionOf(rescan, 'Query parameters')).getByLabelText('Value');
      const headerField = within(sectionOf(rescan, 'Headers')).getByLabelText('Value');
      const invalid: Array<[string, HTMLElement, string]> = [
        ['method', within(rescan).getByLabelText('Action method'), 'Entity method rejected'],
        ['path', within(rescan).getByLabelText('Action path'), 'Entity path rejected'],
        ['label', within(rescan).getByLabelText('Action label'), 'Entity label rejected'],
        ['downtime', within(rescan).getByLabelText('Downtime seconds'), 'Entity downtime rejected'],
        ['query key', queryField, 'Entity query rejected'],
        ['header', headerField, 'Entity header rejected'],
        ['body', within(rescan).getByLabelText('Request body'), 'Entity body rejected'],
        ['action row', rescan, 'Entity action rejected'],
        ['entity actions', entityActions, 'Entity actions rejected'],
        ['service method', within(rowByLegend('restart')).getByLabelText('Action method'), 'Service method rejected'],
        ['service actions', serviceSection(), 'Service actions rejected'],
      ];
      for (const [name, element, message] of invalid) {
        expect(element, name).toHaveAttribute('aria-invalid', 'true');
        expect(screen.getByText(message), name).toBeInTheDocument();
      }
    });

    it('opens the recipe-actions example from the format docs with no markers and writes nothing on focus and blur', () => {
      const docs = readFileSync(`${repositoryRoot}/docs/connectors/RECIPE_FORMAT.md`, 'utf8');
      const example = /<!-- recipe-actions-example-start -->\s*```yaml\n([\s\S]*?)```\s*<!-- recipe-actions-example-end -->/.exec(docs)?.[1];
      expect(example).toBeDefined();
      const onChange = vi.fn();
      render(<RecipeBuilder value={example ?? ''} onChange={onChange} />);

      expect(screen.queryByText(/editable in YAML only/)).not.toBeInTheDocument();
      expect(screen.getByText('Service actions')).toBeInTheDocument();
      expect(screen.getAllByLabelText('Action path')).toHaveLength(2);
      for (const control of [...screen.getAllByRole('textbox'), ...screen.getAllByRole('spinbutton')]) {
        fireEvent.focus(control);
        fireEvent.blur(control);
      }
      expect(onChange).not.toHaveBeenCalled();
    });
  });
});
