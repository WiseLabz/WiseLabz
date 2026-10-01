/**
 * Confirm-with-blast-radius dialog for the one destructive op in v1 (connector
 * removal). Machine-honest: fetches the server-computed blast radius and states
 * the concrete dependents the action destroys, requires type-to-confirm (the
 * resource name), and — when step-up is enabled — a fresh elevation token before
 * the delete fires.
 *
 * A modal is the right affordance here: a destructive, focus-demanding decision
 * that must interrupt. Built on ui/Dialog (native <dialog>), so focus moves in,
 * is trapped, and is restored on close; Escape and backdrop-click cancel.
 */
import {useState} from 'react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {
  deleteConnectorsConnectorId,
  getGetConnectorsQueryKey,
  useGetConnectorsConnectorIdRemovalImpact,
} from '../../api/generated/connectors/connectors';
import {useGetAuthConfig} from '../../api/generated/settings/settings';
import {Button} from '../ui/Button';
import {Dialog} from '../ui/Dialog';
import {StepUp} from './StepUp';
import {AlertTriangleIcon} from '../icons';

export function ConfirmDestructive({
                                       open,
                                       connectorId,
                                       connectorName,
                                       onClose,
                                       onConfirmed,
                                   }: {
    open: boolean;
    connectorId: string;
    connectorName: string;
    onClose: () => void;
    onConfirmed: () => void;
}) {
    const queryClient = useQueryClient();
    const [typed, setTyped] = useState('');
    const [token, setToken] = useState<string | null>(null);

    const impact = useGetConnectorsConnectorIdRemovalImpact(connectorId, {
        query: {enabled: open},
    });
    const authConfig = useGetAuthConfig({query: {enabled: open}});
    const stepUpRequired = authConfig.data?.stepUpForDestructive ?? true;

    // Transient input is cleared on every close path, so each open starts fresh.
    const reset = () => {
        setTyped('');
        setToken(null);
    };
    const close = () => {
        reset();
        onClose();
    };

    const remove = useMutation({
        mutationFn: () =>
            deleteConnectorsConnectorId(
                connectorId,
                token ? {headers: {'X-Elevation-Token': token}} : undefined,
            ),
        onSuccess: () => {
            queryClient.invalidateQueries({queryKey: getGetConnectorsQueryKey()});
            reset();
            onConfirmed();
        },
    });

    const nameMatches = typed === connectorName;
    const elevationOk = !stepUpRequired || !!token;
    const canConfirm = nameMatches && elevationOk && !remove.isPending;

    return (
        <Dialog
            open={open}
            onClose={close}
            title={
                <span className="flex items-center gap-2.5">
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-err-tint text-err">
                        <AlertTriangleIcon size={16}/>
                    </span>
                    Remove connector “{connectorName}”
                </span>
            }
        >
            <div className="space-y-4">
                <p className="text-xs text-ink-muted">This cascades and cannot be undone.</p>

                {/* Blast radius */}
                <div>
                    <p className="mb-2 text-2xs text-ink-faint">What this destroys</p>
                    {impact.isLoading ? (
                        <p className="text-xs text-ink-faint">Computing blast radius…</p>
                    ) : impact.data ? (
                        <ul className="space-y-1">
                            <BlastLine n={impact.data.trackedServices} unit="tracked service"/>
                            <BlastLine n={impact.data.docSections} unit="generated doc section"/>
                            <BlastLine n={impact.data.snapshots} unit="stored snapshot"/>
                        </ul>
                    ) : (
                        <p className="text-xs text-err">
                            Couldn’t compute the blast radius — refusing to proceed blind.
                        </p>
                    )}
                </div>

                {/* Type-to-confirm */}
                <div>
                    <label htmlFor="confirm-name" className="mb-1.5 block text-2xs text-ink-faint">
                        Type <span className="font-mono text-ink">{connectorName}</span> to confirm
                    </label>
                    <input
                        id="confirm-name"
                        value={typed}
                        onChange={(e) => setTyped(e.target.value)}
                        autoComplete="off"
                        className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 font-mono text-sm text-ink outline-none focus-visible:border-accent-primary-soft"
                    />
                </div>

                {/* Step-up (only once the name matches, to keep focus ordered) */}
                {stepUpRequired && nameMatches && !token && (
                    <StepUp action="connector.delete" onElevated={setToken}/>
                )}
                {stepUpRequired && token && (
                    <p className="text-2xs text-ok">Re-authenticated — ready to remove.</p>
                )}

                {remove.isError && (
                    <p role="alert" className="text-2xs text-err">
                        Removal failed. The elevation token may have expired — re-authenticate and retry.
                    </p>
                )}

                <div className="flex items-center justify-end gap-2 pt-1">
                    <Button variant="ghost" size="md" onClick={close}>
                        Cancel
                    </Button>
                    <Button
                        variant="danger"
                        size="md"
                        disabled={!canConfirm}
                        onClick={() => remove.mutate()}
                    >
                        {remove.isPending ? 'Removing…' : 'Remove connector'}
                    </Button>
                </div>
            </div>
        </Dialog>
    );
}

function BlastLine({n, unit}: { n: number; unit: string }) {
    return (
        <li className="flex items-baseline gap-2 text-sm text-ink">
            <span className="nums font-mono font-semibold text-err">{n}</span>
            <span className="text-ink-muted">
        {unit}
                {n === 1 ? '' : 's'}
      </span>
        </li>
    );
}
