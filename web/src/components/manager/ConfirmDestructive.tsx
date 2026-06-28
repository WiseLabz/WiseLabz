/**
 * Confirm-with-blast-radius dialog for the one destructive op in v1 (connector
 * removal). Machine-honest: fetches the server-computed blast radius and states
 * the concrete dependents the action destroys, requires type-to-confirm (the
 * resource name), and — when step-up is enabled — a fresh elevation token before
 * the delete fires.
 *
 * A modal is the right affordance here: a destructive, focus-demanding decision
 * that must interrupt. Escape and backdrop-click cancel; focus moves into it.
 */
import {useEffect, useState} from 'react';
import {AnimatePresence, motion} from 'motion/react';
import {useMutation, useQueryClient} from '@tanstack/react-query';
import {
  deleteConnectorsConnectorId,
  getGetConnectorsQueryKey,
  useGetConnectorsConnectorIdRemovalImpact,
} from '../../api/generated/connectors/connectors';
import {useGetAuthConfig} from '../../api/generated/settings/settings';
import {Button} from '../ui/Button';
import {StepUp} from './StepUp';
import {AlertTriangleIcon, XIcon} from '../icons';

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

    // Escape to cancel.
    useEffect(() => {
        if (!open) return;
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') close();
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [open]);

    return (
        <AnimatePresence>
            {open && (
                <motion.div
                    className="fixed inset-0 z-(--z-modal) flex items-center justify-center p-4"
                    initial={{opacity: 0}}
                    animate={{opacity: 1}}
                    exit={{opacity: 0}}
                >
                    <div
                        className="absolute inset-0 bg-black/55 backdrop-blur-sm"
                        onClick={close}
                        aria-hidden="true"
                    />
                    <motion.div
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="confirm-destructive-title"
                        initial={{opacity: 0, y: 10, scale: 0.98}}
                        animate={{opacity: 1, y: 0, scale: 1}}
                        exit={{opacity: 0, y: 10, scale: 0.98}}
                        transition={{duration: 0.18, ease: [0.16, 1, 0.3, 1]}}
                        className="relative z-(--z-modal) w-full max-w-md overflow-hidden rounded-lg border border-line bg-surface-overlay shadow-(--shadow-pop)"
                    >
                        <div className="flex items-start gap-3 border-b border-line-soft px-5 py-4">
              <span
                  className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-err-tint text-err">
                <AlertTriangleIcon size={17}/>
              </span>
                            <div className="flex-1">
                                <h2
                                    id="confirm-destructive-title"
                                    className="text-sm font-semibold text-ink"
                                >
                                    Remove connector “{connectorName}”
                                </h2>
                                <p className="mt-0.5 text-xs text-ink-muted">
                                    This cascades and cannot be undone.
                                </p>
                            </div>
                            <button
                                onClick={close}
                                aria-label="Cancel"
                                className="text-ink-faint transition-colors hover:text-ink"
                            >
                                <XIcon size={16}/>
                            </button>
                        </div>

                        <div className="space-y-4 px-5 py-4">
                            {/* Blast radius */}
                            <div>
                                <p className="mb-2 text-2xs uppercase tracking-wider text-ink-faint">
                                    What this destroys
                                </p>
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
                                <label
                                    htmlFor="confirm-name"
                                    className="mb-1.5 block text-2xs uppercase tracking-wider text-ink-faint"
                                >
                                    Type <span className="font-mono text-ink">{connectorName}</span> to
                                    confirm
                                </label>
                                <input
                                    id="confirm-name"
                                    value={typed}
                                    onChange={(e) => setTyped(e.target.value)}
                                    autoComplete="off"
                                    className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 font-mono text-sm text-ink outline-none focus-visible:border-signal-soft"
                                />
                            </div>

                            {/* Step-up (only once the name matches, to keep focus ordered) */}
                            {stepUpRequired && nameMatches && !token && (
                                <StepUp onElevated={setToken}/>
                            )}
                            {stepUpRequired && token && (
                                <p className="text-2xs text-ok">Re-authenticated — ready to remove.</p>
                            )}

                            {remove.isError && (
                                <p className="text-2xs text-err">
                                    Removal failed. The elevation token may have expired — re-authenticate and retry.
                                </p>
                            )}
                        </div>

                        <div
                            className="flex items-center justify-end gap-2 border-t border-line-soft px-5 py-3.5">
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
                    </motion.div>
                </motion.div>
            )}
        </AnimatePresence>
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
