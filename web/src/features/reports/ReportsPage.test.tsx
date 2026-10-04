import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { expect, it, vi } from 'vitest';
import '../../i18n';
import { ReportsPage } from './ReportsPage';

const { create } = vi.hoisted(() => ({ create: vi.fn() }));
vi.mock('../../api/generated/reports/reports', () => ({
  useGetReportsDefinitions: () => ({ data: [], isLoading: false, isError: false }),
  useGetReports: () => ({ data: { items: [] }, isLoading: false, isError: false }),
  getGetReportsDefinitionsQueryKey: () => ['definitions'],
  getGetReportsQueryKey: () => ['reports'],
  postReportsDefinitions: create,
}));

it('selects email and persists the Lab Book flag in a report definition', async () => {
  create.mockResolvedValue({ id: 'report' });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <MemoryRouter><ReportsPage /></MemoryRouter>
    </QueryClientProvider>
  );
  fireEvent.click(screen.getByRole('button', { name: 'New definition' }));
  fireEvent.change(screen.getByLabelText('name'), { target: { value: 'Weekly' } });
  fireEvent.change(screen.getByLabelText('slug'), { target: { value: 'weekly' } });
  fireEvent.click(screen.getByLabelText('Attach offline Lab Book (email and Discord)'));
  fireEvent.click(screen.getByLabelText('email'));
  fireEvent.click(screen.getByRole('button', { name: 'Save' }));
  await waitFor(() => expect(create).toHaveBeenCalledWith(expect.objectContaining({ attachLabBook: true, channels: ['email'] })));
});
