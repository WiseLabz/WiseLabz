import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { ElevationConfirm } from './ElevationConfirm';

let mockStepUpForDestructive = false;

vi.mock('../../api/generated/settings/settings', () => ({
  useGetAuthConfig: () => ({ data: { stepUpForDestructive: mockStepUpForDestructive } }),
}));

vi.mock('./StepUp', () => ({
  StepUp: ({ onElevated }: { onElevated: (token: string) => void }) => (
    <button data-testid="mock-step-up" onClick={() => onElevated('mock-token')}>
      Step up now
    </button>
  ),
}));

function renderConfirm(props: {
  open?: boolean;
  resourceName?: string;
  action?: string;
  alwaysElevate?: boolean;
  onConfirm?: (token: string | null) => void;
  onClose?: () => void;
}) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <ElevationConfirm
        open={props.open ?? true}
        resourceName={props.resourceName ?? 'test-resource'}
        action={props.action ?? 'test.action'}
        title="Test elevation title"
        confirmLabel="Confirm"
        alwaysElevate={props.alwaysElevate}
        onConfirm={props.onConfirm ?? vi.fn()}
        onClose={props.onClose ?? vi.fn()}
      />
    </QueryClientProvider>,
  );
}

describe('ElevationConfirm', () => {
  afterEach(() => {
    cleanup();
    mockStepUpForDestructive = false;
  });

  it('skips step-up when stepUpForDestructive is false and alwaysElevate is false', () => {
    mockStepUpForDestructive = false;
    const onConfirm = vi.fn();
    renderConfirm({ onConfirm, alwaysElevate: false });

    // Type the resource name to match
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'test-resource' } });

    // StepUp component should not be rendered
    expect(screen.queryByTestId('mock-step-up')).not.toBeInTheDocument();

    // Confirm button should be enabled and call onConfirm(null)
    const confirmButton = screen.getByRole('button', { name: 'Confirm' });
    expect(confirmButton).not.toBeDisabled();
    fireEvent.click(confirmButton);
    expect(onConfirm).toHaveBeenCalledWith(null);
  });

  it('enforces step-up when alwaysElevate is true even if stepUpForDestructive is false', () => {
    mockStepUpForDestructive = false;
    const onConfirm = vi.fn();
    renderConfirm({ onConfirm, alwaysElevate: true });

    // Type the resource name to match
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'test-resource' } });

    // StepUp component MUST be rendered because alwaysElevate is true
    expect(screen.getByTestId('mock-step-up')).toBeInTheDocument();

    // Confirm button should be disabled until stepped up
    const confirmButton = screen.getByRole('button', { name: 'Confirm' });
    expect(confirmButton).toBeDisabled();

    // Perform step-up
    fireEvent.click(screen.getByTestId('mock-step-up'));
    expect(confirmButton).not.toBeDisabled();

    // Now confirm can be clicked and passes the token
    fireEvent.click(confirmButton);
    expect(onConfirm).toHaveBeenCalledWith('mock-token');
  });
});
