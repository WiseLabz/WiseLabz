/**
 * Step-up re-authentication for a single destructive action. Collects a password
 * (or TOTP, when configured), calls POST /auth/elevate, and hands the short-lived
 * elevation token back to the caller, which replays it on the destructive request
 * Self-contained: owns its own input and error state, never persists the secret.
 *
 * `mode: 'oidc'` (#279 part 3) is for a user whose account signs in through an
 * IdP and has no password: it re-authenticates through a popup instead of a
 * form field. See OIDCStepUp below.
 */
import {useState} from 'react';
import {useMutation} from '@tanstack/react-query';
import {postAuthElevate} from '../../api/generated/auth/auth';
import {Button} from '../ui/Button';
import {OIDCStepUp} from './OIDCStepUp';

export function StepUp({
                           onElevated,
                           mode = 'password',
                           action,
                           providerName,
                       }: {
    onElevated: (token: string) => void;
    mode?: 'password' | 'totp' | 'oidc';
    action: string;
    /** Display name for the oidc mode's button, e.g. "Authentik". */
    providerName?: string;
}) {
    const [value, setValue] = useState('');
    const elevate = useMutation({
        mutationFn: () =>
            postAuthElevate(mode === 'totp' ? {totp: value, action} : {password: value, action}),
        onSuccess: (res) => onElevated(res.token),
    });

    if (mode === 'oidc') {
        return <OIDCStepUp action={action} providerName={providerName} onElevated={onElevated}/>;
    }

    const label = mode === 'totp' ? 'Authenticator code' : 'Confirm your password';
    return (
        <form
            onSubmit={(e) => {
                e.preventDefault();
                if (value) elevate.mutate();
            }}
            className="rounded-md border border-line-soft bg-canvas-sunken p-3"
        >
            <label className="mb-1.5 block text-2xs text-ink-faint">
                {label}
            </label>
            <div className="flex items-center gap-2">
                <input
                    type={mode === 'totp' ? 'text' : 'password'}
                    inputMode={mode === 'totp' ? 'numeric' : undefined}
                    autoComplete={mode === 'totp' ? 'one-time-code' : 'current-password'}
                    value={value}
                    onChange={(e) => setValue(e.target.value)}
                    autoFocus
                    className="h-9 flex-1 rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none focus-visible:border-accent-primary-soft"
                />
                <Button type="submit" variant="secondary" size="md" disabled={!value || elevate.isPending}>
                    {elevate.isPending ? 'Verifying…' : 'Verify'}
                </Button>
            </div>
            {elevate.isError && (
                <p className="mt-1.5 text-2xs text-err">
                    Re-authentication failed. Check your {mode === 'totp' ? 'code' : 'password'} and try again.
                </p>
            )}
        </form>
    );
}
