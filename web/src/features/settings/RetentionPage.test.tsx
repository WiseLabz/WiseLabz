import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { RetentionPage } from './RetentionPage';

const { putSystemSettingsRetention } = vi.hoisted(() => ({
  putSystemSettingsRetention: vi.fn().mockResolvedValue({}),
}));

let retentionData = {
  snapshotDays: 90,
  docVersionDays: 365,
  alertDays: 180,
  syncRunDays: 90,
  auditDays: 180,
  healthCheckDays: 90,
  reportDays: 90,
  deletedDocsDays: 30,
  runbookOpenRunHours: 24,
  runbookRunDays: 90,
  cronExpr: '0 0 * * *',
};

vi.mock('../../api/generated/system/system', () => ({
  useGetSystemSettingsRetention: () => ({
    data: retentionData,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  getGetSystemSettingsRetentionQueryKey: () => ['retentionSettings'],
  putSystemSettingsRetention,
}));

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <RetentionPage />
    </QueryClientProvider>
  );
}

describe('RetentionPage', () => {
  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders retention settings with runbook open run hours and runbook run days', () => {
    renderPage();

    expect(screen.getByLabelText('Open runbook runs (hours)')).toHaveValue(24);
    expect(screen.getByLabelText('Runbook run history (days)', { exact: false })).toHaveValue(90);
    expect(screen.getByText('0 days keeps history forever.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled();
  });

  it('allows updating runbookOpenRunHours and runbookRunDays and saving', async () => {
    renderPage();

    const hoursInput = screen.getByLabelText('Open runbook runs (hours)');
    const daysInput = screen.getByLabelText('Runbook run history (days)', { exact: false });
    const saveButton = screen.getByRole('button', { name: 'Save' });

    fireEvent.change(hoursInput, { target: { value: '48' } });
    fireEvent.change(daysInput, { target: { value: '180' } });

    expect(saveButton).toBeEnabled();
    fireEvent.click(saveButton);

    await waitFor(() =>
      expect(putSystemSettingsRetention).toHaveBeenCalledWith(
        expect.objectContaining({
          runbookOpenRunHours: 48,
          runbookRunDays: 180,
        })
      )
    );
  });

  it('allows runbookRunDays to be 0 to keep history forever', async () => {
    renderPage();

    const daysInput = screen.getByLabelText('Runbook run history (days)', { exact: false });
    const saveButton = screen.getByRole('button', { name: 'Save' });

    fireEvent.change(daysInput, { target: { value: '0' } });
    expect(saveButton).toBeEnabled();

    fireEvent.click(saveButton);

    await waitFor(() =>
      expect(putSystemSettingsRetention).toHaveBeenCalledWith(
        expect.objectContaining({
          runbookRunDays: 0,
        })
      )
    );
  });

  it('disables save when runbookOpenRunHours is less than 1 or exceeds 8760', () => {
    renderPage();

    const hoursInput = screen.getByLabelText('Open runbook runs (hours)');
    const saveButton = screen.getByRole('button', { name: 'Save' });

    fireEvent.change(hoursInput, { target: { value: '0' } });
    expect(saveButton).toBeDisabled();

    fireEvent.change(hoursInput, { target: { value: '-5' } });
    expect(saveButton).toBeDisabled();

    fireEvent.change(hoursInput, { target: { value: '8761' } });
    expect(saveButton).toBeDisabled();

    fireEvent.change(hoursInput, { target: { value: '8760' } });
    expect(saveButton).toBeEnabled();
  });

  it('disables save when runbookRunDays is negative or exceeds 3650', () => {
    renderPage();

    const daysInput = screen.getByLabelText('Runbook run history (days)', { exact: false });
    const saveButton = screen.getByRole('button', { name: 'Save' });

    fireEvent.change(daysInput, { target: { value: '-1' } });
    expect(saveButton).toBeDisabled();

    fireEvent.change(daysInput, { target: { value: '3651' } });
    expect(saveButton).toBeDisabled();

    fireEvent.change(daysInput, { target: { value: '3650' } });
    expect(saveButton).toBeEnabled();
  });
});
