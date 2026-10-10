/**
 * Confirm-with-step-up dialog for destructive/sensitive user & template ops
 * (user delete, user reset-password, template delete). Smaller sibling of
 * ConfirmDestructive: no blast-radius fetch, since users/templates have no
 * removal-impact endpoint. Requires type-to-confirm (the resource name), plus
 * — when step-up is enabled — a fresh elevation token before `onConfirm` fires.
 *
 * A modal is the right affordance here: a destructive, focus-demanding decision
 * that must interrupt. Built on ui/Dialog (native <dialog>), so focus moves in,
 * is trapped, and is restored on close; Escape and backdrop-click cancel.
 */
import {useState} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {useGetAuthConfig} from '../../api/generated/settings/settings';
import {Button} from '../ui/Button';
import {Dialog} from '../ui/Dialog';
import {StepUp} from './StepUp';
import {AlertTriangleIcon} from '../icons';

export function ElevationConfirm({
                                      open,
                                      resourceName,
                                      action,
                                      target,
                                      title,
                                      description,
                                      confirmLabel,
                                      onClose,
                                      onConfirm,
                                      isPending = false,
                                      alwaysElevate = false,
                                  }: {
    open: boolean;
    resourceName: string;
    action: string;
    /** Resource the elevation token is bound to (the user for user.delete). */
    target?: string;
    title: string;
    description?: string;
    confirmLabel?: string;
    onClose: () => void;
    onConfirm: (elevationToken: string | null) => Promise<void> | void;
    isPending?: boolean;
    alwaysElevate?: boolean;
}) {
    const {t} = useTranslation();
    const [typed, setTyped] = useState('');
    const [token, setToken] = useState<string | null>(null);

    const authConfig = useGetAuthConfig({query: {enabled: open && !alwaysElevate}});
    const stepUpRequired = alwaysElevate || (authConfig.data?.stepUpForDestructive ?? true);

    // Transient input is cleared on every close path, so each open starts fresh.
    const close = () => {
        setTyped('');
        setToken(null);
        onClose();
    };

    const nameMatches = typed === resourceName;
    const elevationOk = !stepUpRequired || !!token;
    const canConfirm = nameMatches && elevationOk && !isPending;

    return (
        <Dialog
            open={open}
            onClose={close}
            title={
                <span className="flex items-center gap-2.5">
                    <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-err-tint text-err">
                        <AlertTriangleIcon size={16}/>
                    </span>
                    {title}
                </span>
            }
        >
            <div className="space-y-4">
                {description && <p className="text-xs text-ink-muted">{description}</p>}

                {/* Type-to-confirm */}
                <div>
                    <label htmlFor="elevation-confirm-name" className="mb-1.5 block text-2xs text-ink-faint">
                        <Trans
                            i18nKey="elevationConfirm.typeName"
                            values={{name: resourceName}}
                            components={{name: <span className="font-mono text-ink"/>}}
                        />
                    </label>
                    <input
                        id="elevation-confirm-name"
                        value={typed}
                        onChange={(e) => setTyped(e.target.value)}
                        autoComplete="off"
                        className="h-9 w-full rounded-sm border border-line bg-surface px-2.5 font-mono text-sm text-ink outline-none focus-visible:border-accent-primary-soft"
                    />
                </div>

                {/* Step-up (only once the name matches, to keep focus ordered) */}
                {stepUpRequired && nameMatches && !token && (
                    <StepUp action={action} target={target} onElevated={setToken}/>
                )}
                {stepUpRequired && token && (
                    <p className="text-2xs text-ok">{t('elevationConfirm.ready')}</p>
                )}

                <div className="flex items-center justify-end gap-2 pt-1">
                    <Button variant="ghost" size="md" onClick={close}>
                        {t('common.cancel')}
                    </Button>
                    <Button
                        variant="danger"
                        size="md"
                        disabled={!canConfirm}
                        onClick={() => onConfirm(token)}
                    >
                        {isPending ? t('elevationConfirm.working') : (confirmLabel ?? t('elevationConfirm.confirm'))}
                    </Button>
                </div>
            </div>
        </Dialog>
    );
}
