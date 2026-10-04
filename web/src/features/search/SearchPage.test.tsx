import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { SearchPage } from './SearchPage';
import type { SearchResults } from '../../api/model';

const { get, view } = vi.hoisted(() => ({ get: vi.fn(), view: vi.fn() }));
vi.mock('../../api/generated/search/search', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api/generated/search/search')>()),
  useGetSearch: (params: unknown, options: unknown) => {
    get(params, options);
    return view();
  },
}));
vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectors: () => ({ data: [{ id: 'c', name: 'Gateway' }] }),
}));
vi.mock('../../components/runbook/RunbookPanel', () => ({
  RunbookPanel: ({ runbookId }: { runbookId: string }) => <p>Runbook viewer {runbookId}</p>,
}));
const results: SearchResults = {
  docs: [{ type: 'doc', id: 'd', title: 'Router notes', snippet: 'Restore gateway', score: 1 }],
  runbooks: [{ type: 'runbook', id: 'r', title: 'Recovery', snippet: 'Restart safely', score: 1 }],
  entities: [
    {
      connectorId: 'c',
      connectorName: 'Gateway',
      docId: 'd',
      kind: 'device',
      name: 'router',
      externalId: '100',
      ip: '10.0.0.1',
      hostname: 'gateway.lab',
      mac: 'aa:bb:cc:dd:ee:ff',
      aliases: ['dns.lab'],
    },
  ],
};
function Location() {
  return <output data-testid="location">{useLocation().search}</output>;
}
function mount(path = '/search?q=router') {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter initialEntries={[path]}>
        <SearchPage />
        <Location />
      </MemoryRouter>
    </QueryClientProvider>
  );
}
describe('SearchPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    view.mockReturnValue({ data: results, isLoading: false, isError: false, refetch: vi.fn() });
  });
  it('renders all groups and links entities to the generated doc', () => {
    mount();
    expect(screen.getByRole('region', { name: 'Docs' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Runbooks' })).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Entities' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'router' })).toHaveAttribute('href', '/docs/d');
    expect(screen.getByText(/aa:bb:cc:dd:ee:ff/)).toHaveTextContent('dns.lab');
    fireEvent.click(screen.getByRole('link', { name: 'Recovery' }));
    expect(screen.getByText('Runbook viewer r')).toBeInTheDocument();
  });
  it('initializes filters from the URL and keeps edits in it', async () => {
    mount('/search?q=router&type=entity&connector=c&kind=device');
    expect(get).toHaveBeenLastCalledWith(
      { q: 'router', type: 'entity', connector: 'c', kind: 'device', limit: 100 },
      { query: { enabled: true } }
    );
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'doc' } });
    expect(screen.getByTestId('location')).toHaveTextContent('type=doc');
    fireEvent.change(screen.getByLabelText('Connector'), { target: { value: '' } });
    expect(screen.getByTestId('location')).not.toHaveTextContent('connector=');
    fireEvent.change(screen.getByLabelText('Entity kind'), { target: { value: 'vm' } });
    expect(screen.getByTestId('location')).toHaveTextContent('kind=vm');
    fireEvent.change(screen.getByLabelText('Search'), { target: { value: '10.0.0.1' } });
    expect(screen.getByTestId('location')).toHaveTextContent('q=10.0.0.1');
    await waitFor(() =>
      expect(get).toHaveBeenLastCalledWith(
        { q: '10.0.0.1', type: 'doc', connector: undefined, kind: 'vm', limit: 100 },
        { query: { enabled: true } }
      )
    );
  });
  it('does not search fewer than two characters and debounces edits', async () => {
    mount('/search');
    expect(screen.getByText('Enter at least two characters to search')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Search'), { target: { value: 'x' } });
    expect(get.mock.lastCall?.[1]).toEqual({ query: { enabled: false } });
    fireEvent.change(screen.getByLabelText('Search'), { target: { value: 'host' } });
    expect(get.mock.lastCall?.[1]).toEqual({ query: { enabled: false } });
    await waitFor(() => expect(get.mock.lastCall?.[0]).toHaveProperty('q', 'host'));
    expect(get.mock.lastCall?.[1]).toEqual({ query: { enabled: true } });
  });
  it('shows empty, loading, and error states', () => {
    view.mockReturnValue({ data: { docs: [], runbooks: [], entities: [] } });
    const rendered = mount();
    expect(screen.getByText('No results found')).toBeInTheDocument();
    rendered.unmount();
    view.mockReturnValue({ isLoading: true });
    const loading = mount();
    expect(screen.getByRole('status', { name: 'Loading' })).toHaveAttribute('aria-busy', 'true');
    loading.unmount();
    view.mockReturnValue({ isError: true, refetch: vi.fn() });
    mount();
    expect(screen.getByRole('alert')).toHaveTextContent('Could not search the lab');
  });
});
