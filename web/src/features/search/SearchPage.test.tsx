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
    HTMLElement.prototype.scrollIntoView = vi.fn();
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
    const panel = screen.getByText('Runbook viewer r').parentElement!;
    expect(panel).toHaveFocus();
    expect(panel.scrollIntoView).toHaveBeenCalledWith({ block: 'start' });
    expect(
      panel.compareDocumentPosition(screen.getByRole('region', { name: 'Docs' })) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
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
  it('debounces and lowercases the kind filter', async () => {
    mount();
    fireEvent.change(screen.getByLabelText('Entity kind'), { target: { value: 'DEVICE' } });
    expect(get.mock.lastCall?.[1]).toEqual({ query: { enabled: false } });
    expect(get.mock.lastCall?.[0]).toHaveProperty('kind', undefined);
    await waitFor(() => expect(get.mock.lastCall?.[0]).toHaveProperty('kind', 'device'));
    expect(get.mock.lastCall?.[1]).toEqual({ query: { enabled: true } });
  });
  it('focuses and scrolls a runbook selected by the URL, including a changed selection', () => {
    mount('/search?q=router&runbook=other');
    const initialPanel = screen.getByText('Runbook viewer other').parentElement!;
    expect(initialPanel).toHaveFocus();
    expect(initialPanel.scrollIntoView).toHaveBeenCalledOnce();
    fireEvent.click(screen.getByRole('link', { name: 'Recovery' }));
    expect(screen.getByText('Runbook viewer r').parentElement).toHaveFocus();
    expect(initialPanel.scrollIntoView).toHaveBeenCalledTimes(2);
  });
  it('keeps its live region mounted when results become empty', () => {
    const rendered = mount();
    const live = screen.getByRole('region', { name: 'Docs' }).closest('[aria-live]');
    expect(live).toHaveAttribute('aria-live', 'polite');
    view.mockReturnValue({ data: { docs: [], runbooks: [], entities: [] } });
    fireEvent.change(screen.getByLabelText('Type'), { target: { value: 'doc' } });
    expect(screen.getByText('No results found').closest('[aria-live]')).toBe(live);
    rendered.unmount();
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
