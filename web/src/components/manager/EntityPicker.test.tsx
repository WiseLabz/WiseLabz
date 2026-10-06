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
});
