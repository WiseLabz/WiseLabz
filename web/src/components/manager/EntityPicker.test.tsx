import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import '../../i18n';
import { EntityPicker } from './EntityPicker';

const snapshotListHook = vi.fn();
const snapshotDetailHook = vi.fn();

vi.mock('../../api/generated/connectors/connectors', () => ({
  useGetConnectorsConnectorIdSnapshots: (...args: unknown[]) => snapshotListHook(...args),
  useGetConnectorsConnectorIdSnapshotsSnapshotId: (...args: unknown[]) => snapshotDetailHook(...args),
}));

describe('EntityPicker', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    snapshotListHook.mockReturnValue({ data: [{ id: 'snap-1' }] });
  });

  afterEach(cleanup);

  it('filters out entities without externalId when kind is not specified', () => {
    snapshotDetailHook.mockReturnValue({
      data: {
        entities: [
          { externalId: 'vm-100', kind: 'vm', name: 'router' },
          { kind: 'vm', name: 'unmanaged-vm' },
        ],
      },
    });

    render(<EntityPicker connectorId="c1" value="" onChange={vi.fn()} />);

    expect(screen.getByRole('option', { name: 'router (vm-100)' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /unmanaged-vm/ })).not.toBeInTheDocument();
  });

  it('falls back to entity name when externalId is absent and kind is specified', () => {
    const onChange = vi.fn();
    const onEntityChange = vi.fn();
    const bareMetalEntity = { kind: 'vm', name: 'bare-metal' };

    snapshotDetailHook.mockReturnValue({
      data: {
        entities: [
          { externalId: 'vm-100', kind: 'vm', name: 'router' },
          bareMetalEntity,
          { externalId: 'ct-200', kind: 'container', name: 'docker-app' },
        ],
      },
    });

    render(
      <EntityPicker
        connectorId="c1"
        value=""
        kind="vm"
        onChange={onChange}
        onEntityChange={onEntityChange}
      />
    );

    expect(screen.getByRole('option', { name: 'router (vm-100)' })).toBeInTheDocument();
    expect(screen.getByRole('option', { name: 'bare-metal' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: /docker-app/ })).not.toBeInTheDocument();

    const select = screen.getByRole('combobox');
    fireEvent.change(select, { target: { value: 'bare-metal' } });

    expect(onChange).toHaveBeenCalledWith('bare-metal');
    expect(onEntityChange).toHaveBeenCalledWith(bareMetalEntity);
  });

  it('displays hint when no entities match the specified kind', () => {
    snapshotDetailHook.mockReturnValue({
      data: {
        entities: [
          { externalId: 'ct-200', kind: 'container', name: 'docker-app' },
        ],
      },
    });

    render(<EntityPicker connectorId="c1" value="" kind="vm" onChange={vi.fn()} />);

    expect(screen.getByText('No entities of this kind found on this connector.')).toBeInTheDocument();
  });

  it('collapses same-ref entities of a kind into one option', () => {
    const onEntityChange = vi.fn();
    snapshotDetailHook.mockReturnValue({
      data: {
        entities: [
          { kind: 'vm', name: 'dup' },
          { kind: 'vm', name: 'dup' },
          { externalId: 'dup', kind: 'vm', name: 'other' },
        ],
      },
    });

    render(
      <EntityPicker connectorId="c1" value="" kind="vm" onChange={vi.fn()} onEntityChange={onEntityChange} />
    );

    expect(screen.getAllByRole('option', { name: /dup|other/ })).toHaveLength(1);
    fireEvent.change(screen.getByRole('combobox'), { target: { value: 'dup' } });
    expect(onEntityChange).toHaveBeenCalledWith(expect.objectContaining({ name: 'dup' }));
  });

  it('shows loading, no-snapshot and error states', () => {
    snapshotListHook.mockReturnValue({ data: undefined, isLoading: true });
    snapshotDetailHook.mockReturnValue({ data: undefined });
    const { rerender } = render(<EntityPicker connectorId="c1" value="" onChange={vi.fn()} />);
    expect(screen.getByText('Loading entities…')).toBeInTheDocument();

    snapshotListHook.mockReturnValue({ data: [] });
    rerender(<EntityPicker connectorId="c1" value="" onChange={vi.fn()} />);
    expect(screen.getByText(/No snapshot yet/)).toBeInTheDocument();

    snapshotListHook.mockReturnValue({ data: undefined, isError: true });
    rerender(<EntityPicker connectorId="c1" value="" onChange={vi.fn()} />);
    expect(screen.getByText('Could not load entities for this connector.')).toBeInTheDocument();
  });
});
