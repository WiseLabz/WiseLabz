import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';
import { fireEvent, render, screen, cleanup, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { setupServer } from 'msw/node';
import { http, HttpResponse } from 'msw';
import '../../i18n';
import { RulesPage } from './RulesPage';

const rule = { id: 'r1', name: 'No privileged containers', connectorType: 'docker', entityKind: 'container', conditions: [{ attribute: 'privileged', op: 'eq', value: true }], severity: 'critical', title: 'Privileged container', remediationLink: '', enabled: false, createdAt: '', updatedAt: '' };
const clause = { mode: 'forbids' as const, connectorType: 'pbs', entityKind: 'backup', join: { sourceField: 'external_id', relatedField: 'external_id' }, conditions: [] };
const ruleWithClause = { id: 'r2', name: 'VM without backup', connectorType: 'proxmox', entityKind: 'vm', conditions: [{ attribute: 'template', op: 'eq', value: false }], severity: 'warning', title: 'Unbacked VM', remediationLink: '', enabled: true, createdAt: '', updatedAt: '', related: [clause] };
const schema = {
  attributes: {
    docker: { container: [{ name: 'privileged', type: 'boolean', description: '' }] },
    proxmox: { vm: [{ name: 'template', type: 'boolean', description: '' }] },
    pbs: { backup: [{ name: 'last_backup_age_days', type: 'number', description: '' }] },
    tlsprobe: { certificate: [{ name: 'not_after', type: 'string', description: '' }] },
  },
  joinFields: ['external_id', 'name', 'ip', 'hostname', 'mac'],
};
const server = setupServer(
  http.get('/api/compliance/rules', () => HttpResponse.json({ items: [rule] })),
  http.get('/api/compliance/schema', () => HttpResponse.json(schema)),
  http.get('/api/findings', () => HttpResponse.json({ items: [], total: 0, page: 1, pageSize: 500 })),
);
beforeAll(() => server.listen({ onUnhandledRequest: 'error' }));
afterEach(() => { server.resetHandlers(); cleanup(); });
afterAll(() => server.close());

function renderPage() { return render(<QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><RulesPage /></QueryClientProvider>); }

// Fills the rule's own fields: docker container with a privileged=true condition.
async function fillBaseRule() {
  fireEvent.click(await screen.findByRole('button', { name: 'New rule' }));
  fireEvent.change(await screen.findByLabelText('Rule name'), { target: { value: 'Rule under test' } });
  fireEvent.change(screen.getByLabelText('Finding title'), { target: { value: 'Finding under test' } });
  await screen.findByRole('option', { name: 'docker' });
  fireEvent.change(screen.getByLabelText('Connector type'), { target: { value: 'docker' } });
  fireEvent.change(screen.getByLabelText('Entity kind'), { target: { value: 'container' } });
  fireEvent.click(screen.getByRole('button', { name: 'Add condition' }));
  fireEvent.change(screen.getByLabelText('Attribute'), { target: { value: 'privileged' } });
  fireEvent.change(screen.getByLabelText('Value'), { target: { value: 'true' } });
}

describe('RulesPage', () => {
  it('renders a rule and toggles it through the API', async () => {
    let enabled = false;
    server.use(http.put('/api/compliance/rules/r1', async ({ request }) => { enabled = (await request.json() as typeof rule).enabled; return HttpResponse.json({ ...rule, enabled }); }));
    renderPage();
    expect(await screen.findByText('No privileged containers')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('switch', { name: 'Enable No privileged containers' }));
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(enabled).toBe(true);
  });

  it('creates a days-left rule with a whole-number threshold', async () => {
    let body: Record<string, unknown> | undefined;
    server.use(http.post('/api/compliance/rules', async ({ request }) => {
      body = await request.json() as Record<string, unknown>;
      return HttpResponse.json({ ...body, id: 'new', createdAt: '', updatedAt: '' });
    }));
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'New rule' }));
    fireEvent.change(await screen.findByLabelText('Rule name'), { target: { value: 'Certificate expiring soon' } });
    fireEvent.change(screen.getByLabelText('Finding title'), { target: { value: 'Certificate expires soon' } });
    await screen.findByRole('option', { name: 'tlsprobe' });
    fireEvent.change(screen.getByLabelText('Connector type'), { target: { value: 'tlsprobe' } });
    fireEvent.change(screen.getByLabelText('Entity kind'), { target: { value: 'certificate' } });
    fireEvent.click(screen.getByRole('button', { name: 'Add condition' }));
    fireEvent.change(screen.getByLabelText('Attribute'), { target: { value: 'not_after' } });
    fireEvent.change(screen.getByLabelText('Operator'), { target: { value: 'days_left_lt' } });
    expect(screen.getByLabelText('Value')).toHaveAttribute('type', 'number');
    expect(screen.getByLabelText('Value')).toHaveAttribute('step', '1');
    fireEvent.change(screen.getByLabelText('Value'), { target: { value: '8' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(body).toBeDefined());
    expect(body?.conditions).toEqual([{ attribute: 'not_after', op: 'days_left_lt', value: 8 }]);
  });

  it('installs the recommended pack from the empty state', async () => {
    let installed = false;
    server.use(
      http.get('/api/compliance/rules', () => HttpResponse.json({ items: installed ? [rule] : [] })),
      http.post('/api/compliance/packs/recommended/install', () => { installed = true; return HttpResponse.json({ installed: 1, skipped: 0 }); }),
    );
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Install recommended rules' }));
    expect(await screen.findByText('No privileged containers')).toBeInTheDocument();
    expect(installed).toBe(true);
  });

  it('creates a rule with a related clause and a clause condition', async () => {
    let body: Record<string, unknown> | undefined;
    server.use(http.post('/api/compliance/rules', async ({ request }) => { body = await request.json() as Record<string, unknown>; return HttpResponse.json({ ...body, id: 'new', createdAt: '', updatedAt: '' }); }));
    renderPage();
    await fillBaseRule();

    fireEvent.click(screen.getByRole('button', { name: 'Add related entity check' }));
    fireEvent.change(screen.getByLabelText('Mode'), { target: { value: 'requires' } });
    fireEvent.change(screen.getByLabelText('Related service type'), { target: { value: 'pbs' } });
    fireEvent.change(screen.getByLabelText('Related entity kind'), { target: { value: 'backup' } });
    fireEvent.change(screen.getByLabelText('Source field'), { target: { value: 'external_id' } });
    fireEvent.change(screen.getByLabelText('Related field'), { target: { value: 'external_id' } });
    fireEvent.click(screen.getAllByRole('button', { name: 'Add condition' })[1]);
    // The first "Attribute" select belongs to the rule, the second to the clause.
    fireEvent.change(screen.getAllByLabelText('Attribute')[1], { target: { value: 'last_backup_age_days' } });
    fireEvent.change(screen.getAllByLabelText('Operator')[1], { target: { value: 'lt' } });
    fireEvent.change(screen.getAllByLabelText('Value')[1], { target: { value: '7' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => expect(body).toBeDefined());
    expect(body?.conditions).toEqual([{ attribute: 'privileged', op: 'eq', value: true }]);
    expect(body?.related).toEqual([{
      mode: 'requires', connectorType: 'pbs', entityKind: 'backup',
      join: { sourceField: 'external_id', relatedField: 'external_id' },
      conditions: [{ attribute: 'last_backup_age_days', op: 'lt', value: 7 }],
    }]);
  });

  it('loads a saved clause into the editor and clears it on removal', async () => {
    let body: Record<string, unknown> | undefined;
    server.use(
      http.get('/api/compliance/rules', () => HttpResponse.json({ items: [ruleWithClause] })),
      http.put('/api/compliance/rules/r2', async ({ request }) => { body = await request.json() as Record<string, unknown>; return HttpResponse.json(body); }),
    );
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Edit rule' }));
    expect(await screen.findByLabelText('Mode')).toHaveValue('forbids');
    expect(screen.getByLabelText('Related service type')).toHaveValue('pbs');
    expect(screen.getByLabelText('Related entity kind')).toHaveValue('backup');
    expect(screen.getByLabelText('Source field')).toHaveValue('external_id');

    fireEvent.click(screen.getByRole('button', { name: 'Remove clause' }));
    expect(screen.queryByLabelText('Mode')).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(body).toBeDefined());
    expect(body?.related).toEqual([]);
  });

  it('stops offering clauses at the limit of five', async () => {
    const fourClauses = { ...ruleWithClause, related: Array.from({ length: 4 }, () => clause) };
    server.use(http.get('/api/compliance/rules', () => HttpResponse.json({ items: [fourClauses] })));
    renderPage();
    fireEvent.click(await screen.findByRole('button', { name: 'Edit rule' }));
    const add = await screen.findByRole('button', { name: 'Add related entity check' });
    expect(add).toBeEnabled();
    fireEvent.click(add);
    expect(screen.getAllByLabelText('Mode')).toHaveLength(5);
    expect(screen.getByRole('button', { name: 'Add related entity check' })).toBeDisabled();
  });

  it('resets the related entity kind and join field when the clause service type changes', async () => {
    renderPage();
    await fillBaseRule();
    fireEvent.click(screen.getByRole('button', { name: 'Add related entity check' }));
    fireEvent.change(screen.getByLabelText('Related service type'), { target: { value: 'pbs' } });
    fireEvent.change(screen.getByLabelText('Related entity kind'), { target: { value: 'backup' } });
    fireEvent.change(screen.getByLabelText('Related field'), { target: { value: 'attributes.last_backup_age_days' } });
    fireEvent.change(screen.getByLabelText('Related service type'), { target: { value: 'proxmox' } });
    expect(screen.getByLabelText('Related entity kind')).toHaveValue('');
    expect(screen.getByLabelText('Related field')).toHaveValue('');
  });

  it('refuses to save a clause without join fields and sends nothing', async () => {
    let posted = false;
    server.use(http.post('/api/compliance/rules', () => { posted = true; return HttpResponse.json({}); }));
    renderPage();
    await fillBaseRule();
    fireEvent.click(screen.getByRole('button', { name: 'Add related entity check' }));
    fireEvent.change(screen.getByLabelText('Related service type'), { target: { value: 'pbs' } });
    fireEvent.change(screen.getByLabelText('Related entity kind'), { target: { value: 'backup' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(await screen.findByRole('alert')).toBeInTheDocument();
    expect(posted).toBe(false);

    fireEvent.change(screen.getByLabelText('Source field'), { target: { value: 'external_id' } });
    fireEvent.change(screen.getByLabelText('Related field'), { target: { value: 'external_id' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(posted).toBe(true));
  });
});
