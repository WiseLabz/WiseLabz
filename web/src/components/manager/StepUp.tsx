/**
 * Step-up re-authentication for a single destructive action. Collects a password
 * (or TOTP, when configured), calls POST /auth/elevate, and hands the short-lived
 * elevation token back to the caller, which replays it on the destructive request
 * Self-contained: owns its own input and error state, never persists the secret.
 */
import {useState} from 'react';
import {useMutation} from '@tanstack/react-query';
import {postAuthElevate} from '../../api/generated/auth/auth';
import {Button} from '../ui/Button';

export function StepUp({
                           onElevated,
                           mode = 'password',
                       }: {
    onElevated: (token: string) => void;
    mode?: 'password' | 'totp';
}) {
    const [value, setValue] = useState('');
    const elevate = useMutation({
        mutationFn: () =>
            postAuthElevate(mode === 'totp' ? {totp: value} : {password: value}),
        onSuccess: (res) => onElevated(res.token),
    });

    const label = mode === 'totp' ? 'Authenticator code' : 'Confirm your password';
    return (
        <form
            onSubmit={(e) => {
                e.preventDefault();
                if (value) elevate.mutate();
            }}
            className="rounded-md border border-line-soft bg-canvas-sunken p-3"
        >
            <label className="mb-1.5 block text-2xs uppercase tracking-wider text-ink-faint">
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
                    className="h-9 flex-1 rounded-sm border border-line bg-surface px-2.5 text-sm text-ink outline-none focus-visible:border-signal-soft"
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
