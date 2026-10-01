import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  useGetMeMfa,
  getGetMeMfaQueryKey,
  postMeMfaTotp,
  postMeMfaTotpFactorIdConfirm,
  postMeMfaWebauthnRegisterBegin,
  postMeMfaWebauthnRegisterFinish,
  postMeMfaRecoveryCodes,
  deleteMeMfaFactorsFactorId,
} from '../../api/generated/me/me';
import type { MfaFactor } from '../../api/model';
import { setAccessToken } from '../../api/axios-instance';
import { registerWebAuthn, isWebAuthnCancel } from '../../lib/webauthn';
import type { WebAuthnResponse } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { SkeletonRows, ErrorState, EmptyState } from '../../components/ui/states';
import { Dialog } from '../../components/ui/Dialog';
import { ElevationConfirm } from '../../components/manager/ElevationConfirm';
import { TimeAgo } from '../../components/ui/TimeAgo';
import { ToneTag } from '../../components/ui/ToneTag';
import { copyText } from '../../lib/clipboard';
import { toast } from '../../lib/toast';
import { Section, Field, TextInput } from './parts';
import { CopyIcon, ShieldIcon, DownloadIcon } from '../../components/icons';
import { StepUp } from '../../components/manager/StepUp';
import { elevationOptions, isElevationRequired } from '../../components/manager/useStepUpMutation';

/**
 * Starting enrollment needs a `mfa.manage` elevation token (#523), except in a
 * forced-enrollment session, where the server skips it and /auth/elevate is
 * off-limits. The web can't tell the two apart, so each start step is tried
 * bare first and only prompts for step-up on the server's `elevation_required`.
 */

/**
 * Settings → Profile → Two-factor authentication (#279, local accounts only).
 * Covers enrollment (QR + manual secret, confirm-with-code, recovery codes
 * shown once), the factor list with a step-up-gated remove action, and a
 * step-up-gated recovery-code regeneration.
 */
export function SecuritySection() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { data, isLoading, isError, refetch } = useGetMeMfa();

  const [enrolling, setEnrolling] = useState(false);
  const [addingKey, setAddingKey] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [keyNeedsStepUp, setKeyNeedsStepUp] = useState(false);
  const [toRemove, setToRemove] = useState<MfaFactor | null>(null);
  const [regenerating, setRegenerating] = useState(false);
  const [newCodes, setNewCodes] = useState<string[] | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: getGetMeMfaQueryKey() });

  const removeFactor = useMutation({
    mutationFn: ({ id, token }: { id: string; token: string | null }) =>
      deleteMeMfaFactorsFactorId(
        id,
        token ? { headers: { 'X-Elevation-Token': token } } : undefined
      ),
    onSuccess: () => {
      invalidate();
      toast.success(t('settings.security.factorRemoved', { defaultValue: 'Factor removed.' }));
      setToRemove(null);
    },
    onError: () =>
      toast.error(
        t('settings.security.factorRemoveError', { defaultValue: 'Could not remove that factor.' })
      ),
  });

  const regenerateCodes = useMutation({
    mutationFn: (token: string | null) =>
      postMeMfaRecoveryCodes(token ? { headers: { 'X-Elevation-Token': token } } : undefined),
    onSuccess: (res) => {
      invalidate();
      setNewCodes(res.recoveryCodes);
      setRegenerating(false);
    },
    onError: () =>
      toast.error(
        t('settings.security.recoveryError', {
          defaultValue: 'Could not regenerate recovery codes.',
        })
      ),
  });

  const registerKey = useMutation({
    mutationFn: async (token: string | null) => {
      const options = await postMeMfaWebauthnRegisterBegin(
        { name: keyName.trim() },
        elevationOptions(token)
      );
      const credential = await registerWebAuthn(options);
      return postMeMfaWebauthnRegisterFinish(credential as unknown as WebAuthnResponse);
    },
    onSuccess: (res) => {
      if (res.accessToken) setAccessToken(res.accessToken);
      invalidate();
      setAddingKey(false);
      setKeyName('');
      if (res.recoveryCodes) setNewCodes(res.recoveryCodes);
    },
    onError: (err) => {
      if (isElevationRequired(err)) setKeyNeedsStepUp(true);
      else if (!isWebAuthnCancel(err))
        toast.error(
          t('settings.security.keyError', { defaultValue: 'Could not add the security key.' })
        );
    },
  });

  return (
    <Section
      title={t('settings.security.title', { defaultValue: 'Two-factor authentication' })}
      description={t('settings.security.subtitle', {
        defaultValue: 'Require a second factor when you sign in and before sensitive actions.',
      })}
    >
      {isLoading ? (
        <SkeletonRows rows={2} className="p-0" />
      ) : isError || !data ? (
        <ErrorState
          description={t('settings.security.loadError', {
            defaultValue: 'Could not load two-factor settings.',
          })}
          onRetry={() => refetch()}
        />
      ) : (
        <>
          {data.required && data.factors.length === 0 && (
            <p className="mb-3 rounded-md border border-line-soft bg-canvas-sunken px-3 py-2 text-2xs text-ink-muted">
              {t('settings.security.requiredNotice', {
                defaultValue: 'Your instance requires two-factor authentication for your account.',
              })}
            </p>
          )}

          {data.factors.length === 0 ? (
            <EmptyState
              icon={<ShieldIcon size={18} />}
              title={t('settings.security.empty', {
                defaultValue: 'No authenticator app enrolled',
              })}
              action={
                <Button variant="primary" size="sm" onClick={() => setEnrolling(true)}>
                  {t('settings.security.enroll', { defaultValue: 'Set up authenticator app' })}
                </Button>
              }
            />
          ) : (
            <div className="space-y-3">
              <ul className="divide-y divide-line-soft">
                {data.factors.map((f) => (
                  <li key={f.id} className="flex items-center gap-4 py-3 first:pt-0 last:pb-0">
                    <div className="min-w-0 flex-1">
                      <p className="flex items-center gap-2 text-sm text-ink">
                        <span className="truncate">{f.name}</span>
                        <ToneTag
                          tone="ok"
                          label={
                            f.type === 'webauthn'
                              ? t('settings.security.webauthn', {
                                  defaultValue: 'Security key / passkey',
                                })
                              : t('settings.security.totp', { defaultValue: 'Authenticator app' })
                          }
                        />
                      </p>
                      <p className="mt-0.5 font-mono text-2xs text-ink-faint">
                        {t('settings.security.addedOn', { defaultValue: 'Added' })}{' '}
                        <TimeAgo at={f.createdAt} />
                      </p>
                    </div>
                    <Button variant="danger" size="sm" onClick={() => setToRemove(f)}>
                      {t('common.remove', { defaultValue: 'Remove' })}
                    </Button>
                  </li>
                ))}
              </ul>

              <div className="flex items-center justify-between border-t border-line-soft pt-3">
                <p className="font-mono text-2xs text-ink-faint">
                  {t('settings.security.recoveryCodesRemaining', {
                    defaultValue: '{{count}} recovery codes remaining',
                    count: data.recoveryCodesRemaining,
                  })}
                </p>
                <Button variant="secondary" size="sm" onClick={() => setRegenerating(true)}>
                  {t('settings.security.regenerate', { defaultValue: 'Regenerate recovery codes' })}
                </Button>
              </div>
            </div>
          )}

          <div className="mt-4 border-t border-line-soft pt-4">
            {data.webauthnAvailable && window.isSecureContext ? (
              <Button variant="secondary" size="sm" onClick={() => setAddingKey(true)}>
                {t('settings.security.addKey', { defaultValue: 'Add security key / passkey' })}
              </Button>
            ) : (
              <p className="text-2xs text-ink-muted">
                {t('settings.security.keyUnavailable', {
                  defaultValue:
                    'Security keys are unavailable. Use HTTPS (or localhost) and configure a server origin.',
                })}
              </p>
            )}
          </div>
        </>
      )}

      <Dialog
        open={addingKey}
        onClose={() => {
          setAddingKey(false);
          setKeyNeedsStepUp(false);
        }}
        title={t('settings.security.addKey', { defaultValue: 'Add security key / passkey' })}
        size="sm"
      >
        {keyNeedsStepUp ? (
          <StepUp
            action="mfa.manage"
            onElevated={(token) => {
              setKeyNeedsStepUp(false);
              registerKey.mutate(token);
            }}
          />
        ) : (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              registerKey.mutate(null);
            }}
          >
            <Field
              label={t('settings.security.keyName', { defaultValue: 'Name' })}
              htmlFor="security-key-name"
            >
              <TextInput
                id="security-key-name"
                value={keyName}
                onChange={(event) => setKeyName(event.target.value)}
                placeholder="Security key"
              />
            </Field>
            <Button type="submit" variant="primary" size="sm" disabled={registerKey.isPending}>
              {t('settings.security.registerKey', { defaultValue: 'Register key' })}
            </Button>
          </form>
        )}
      </Dialog>

      <MfaEnrollDialog
        open={enrolling}
        onClose={() => setEnrolling(false)}
        onEnrolled={invalidate}
      />

      <ElevationConfirm
        open={toRemove !== null}
        onClose={() => setToRemove(null)}
        resourceName={toRemove?.name ?? ''}
        action="mfa.manage"
        title={t('settings.security.removeTitle', { defaultValue: 'Remove authenticator factor' })}
        description={t('settings.security.removeConfirm', {
          defaultValue: 'You will no longer be able to use this factor to sign in or step up.',
        })}
        confirmLabel={t('common.remove', { defaultValue: 'Remove' })}
        isPending={removeFactor.isPending}
        onConfirm={(token) => {
          if (toRemove) return removeFactor.mutateAsync({ id: toRemove.id, token });
        }}
      />

      <RegenerateRecoveryCodesDialog
        open={regenerating}
        onClose={() => setRegenerating(false)}
        onRegenerate={(token) => regenerateCodes.mutate(token)}
        isPending={regenerateCodes.isPending}
      />

      <RecoveryCodesDialog codes={newCodes} onClose={() => setNewCodes(null)} />
    </Section>
  );
}

/**
 * A minimal step-up dialog for the "regenerate recovery codes" action, which
 * (unlike factor removal) has no natural "resource name" to type-to-confirm,
 * so it reuses StepUp directly instead of the type-to-confirm ElevationConfirm.
 */
function RegenerateRecoveryCodesDialog({
  open,
  onClose,
  onRegenerate,
  isPending,
}: {
  open: boolean;
  onClose: () => void;
  onRegenerate: (token: string | null) => void;
  isPending: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={t('settings.security.regenerate', { defaultValue: 'Regenerate recovery codes' })}
      size="sm"
    >
      <p className="text-sm leading-relaxed text-ink-muted">
        {t('settings.security.regenerateDesc', {
          defaultValue: 'Your existing recovery codes will stop working. New codes are shown once.',
        })}
      </p>
      <div className="mt-4">
        <StepUpInline action="mfa.manage" onElevated={onRegenerate} isPending={isPending} />
      </div>
    </Dialog>
  );
}

function MfaEnrollDialog({
  open,
  onClose,
  onEnrolled,
}: {
  open: boolean;
  onClose: () => void;
  onEnrolled: () => void;
}) {
  const { t } = useTranslation();
  const [factor, setFactor] = useState<{
    factorId: string;
    secret: string;
    otpauthUrl: string;
  } | null>(null);
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [savedCodes, setSavedCodes] = useState<string[] | null>(null);
  const [needsStepUp, setNeedsStepUp] = useState(false);

  const begin = useMutation({
    mutationFn: (token: string | null) => postMeMfaTotp({}, elevationOptions(token)),
    onSuccess: async (res) => {
      setFactor(res);
      const { toDataURL } = await import('qrcode');
      setQrDataUrl(await toDataURL(res.otpauthUrl));
    },
    onError: (err) => {
      if (isElevationRequired(err)) setNeedsStepUp(true);
      else
        toast.error(
          t('settings.security.enrollError', { defaultValue: 'Could not start enrollment.' })
        );
    },
  });

  const confirm = useMutation({
    mutationFn: () => postMeMfaTotpFactorIdConfirm(factor!.factorId, { code }),
    onSuccess: (res) => {
      // Confirming enrollment on an enrollment-only session also returns a
      // fresh, fully-privileged token pair (#279) — apply it immediately so
      // the caller isn't stuck behind the enrollment allowlist a moment longer.
      if (res.accessToken) setAccessToken(res.accessToken);
      if (res.recoveryCodes) {
        setSavedCodes(res.recoveryCodes);
      } else {
        onEnrolled();
        close();
      }
    },
    onError: () =>
      toast.error(t('settings.security.confirmError', { defaultValue: 'That code is not valid.' })),
  });

  function close() {
    setFactor(null);
    setQrDataUrl(null);
    setCode('');
    setSavedCodes(null);
    setNeedsStepUp(false);
    begin.reset();
    onClose();
  }

  function done() {
    onEnrolled();
    close();
  }

  // Kick off enrollment as soon as the dialog opens.
  if (open && !factor && !begin.isPending && !begin.isError) {
    begin.mutate(null);
  }

  return (
    <Dialog
      open={open}
      onClose={savedCodes ? () => {} : close}
      title={t('settings.security.enroll', { defaultValue: 'Set up authenticator app' })}
      size="sm"
    >
      {savedCodes ? (
        <SavedRecoveryCodes codes={savedCodes} onDone={done} />
      ) : !factor ? (
        needsStepUp ? (
          <StepUp
            action="mfa.manage"
            onElevated={(token) => {
              setNeedsStepUp(false);
              begin.mutate(token);
            }}
          />
        ) : (
          <SkeletonRows rows={3} className="p-0" />
        )
      ) : (
        <form
          className="space-y-4"
          onSubmit={(e) => {
            e.preventDefault();
            if (code) confirm.mutate();
          }}
        >
          <p className="text-sm leading-relaxed text-ink-muted">
            {t('settings.security.scanPrompt', {
              defaultValue:
                'Scan this QR code with your authenticator app, or enter the secret manually.',
            })}
          </p>
          {qrDataUrl && (
            <img
              src={qrDataUrl}
              alt={t('settings.security.qrAlt', {
                defaultValue: 'QR code for authenticator enrollment',
              })}
              className="mx-auto h-40 w-40 rounded-md border border-line-soft bg-white p-2"
            />
          )}
          <Field
            label={t('settings.security.manualSecret', { defaultValue: 'Manual entry secret' })}
            hint={t('settings.security.manualSecretHint', {
              defaultValue: 'Use this if you cannot scan the QR code.',
            })}
          >
            <code className="block break-all rounded-sm border border-line-soft bg-canvas-sunken px-2.5 py-2 font-mono text-xs text-ink">
              {factor.secret}
            </code>
          </Field>
          <Field
            label={t('settings.security.enterCode', { defaultValue: 'Enter the 6-digit code' })}
            htmlFor="mfa-confirm-code"
          >
            <TextInput
              id="mfa-confirm-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              autoFocus
            />
          </Field>
          <div className="flex justify-end gap-2 pt-1">
            <Button type="button" variant="ghost" size="sm" onClick={close}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" size="sm" disabled={!code || confirm.isPending}>
              {confirm.isPending
                ? t('settings.security.confirming', { defaultValue: 'Confirming…' })
                : t('settings.security.confirm', { defaultValue: 'Confirm' })}
            </Button>
          </div>
        </form>
      )}
    </Dialog>
  );
}

/** Recovery codes shown once, with copy and download (.txt), gated by an "I saved these" checkbox. */
function SavedRecoveryCodes({ codes, onDone }: { codes: string[]; onDone: () => void }) {
  const { t } = useTranslation();
  const [saved, setSaved] = useState(false);
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await copyText(codes.join('\n'));
      setCopied(true);
    } catch {
      toast.error(
        t('common.copyError', {
          defaultValue: 'Could not copy. Select and copy the text manually.',
        })
      );
    }
  };

  function download() {
    const blob = new Blob([codes.join('\n') + '\n'], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'wiselabz-recovery-codes.txt';
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="space-y-4">
      <p className="text-sm leading-relaxed text-ink-muted">
        {t('settings.security.recoveryCodesDesc', {
          defaultValue:
            'Save these recovery codes somewhere safe. Each one can be used once if you lose access to your authenticator app. They will not be shown again.',
        })}
      </p>
      <div className="grid select-text grid-cols-2 gap-2 rounded-md border border-line-soft bg-canvas-sunken p-3 font-mono text-xs text-ink">
        {codes.map((c) => (
          <span key={c}>{c}</span>
        ))}
      </div>
      <div className="flex gap-2">
        <Button variant="ghost" size="sm" onClick={() => void copy()}>
          <CopyIcon size={14} />
          {copied
            ? t('settings.profile.apiKeys.copied')
            : t('settings.security.copyAll', { defaultValue: 'Copy all' })}
        </Button>
        <Button variant="ghost" size="sm" onClick={download}>
          <DownloadIcon size={14} />
          {t('settings.security.download', { defaultValue: 'Download .txt' })}
        </Button>
      </div>
      <label className="flex items-center gap-1.5 text-xs text-ink">
        <input type="checkbox" checked={saved} onChange={(e) => setSaved(e.target.checked)} />
        {t('settings.security.savedCheckbox', { defaultValue: "I've saved these recovery codes" })}
      </label>
      <div className="flex justify-end">
        <Button variant="primary" size="sm" disabled={!saved} onClick={onDone}>
          {t('common.done', { defaultValue: 'Done' })}
        </Button>
      </div>
    </div>
  );
}

/** Recovery codes shown once after a regeneration (outside the enrollment flow). */
function RecoveryCodesDialog({ codes, onClose }: { codes: string[] | null; onClose: () => void }) {
  const { t } = useTranslation();
  return (
    <Dialog
      open={codes !== null}
      onClose={() => {}}
      title={t('settings.security.newCodesTitle', { defaultValue: 'New recovery codes' })}
      size="sm"
    >
      {codes && <SavedRecoveryCodes codes={codes} onDone={onClose} />}
    </Dialog>
  );
}

/** Thin StepUp wrapper used where there's no type-to-confirm resource name. */
function StepUpInline({
  action,
  onElevated,
  isPending,
}: {
  action: string;
  onElevated: (token: string) => void;
  isPending: boolean;
}) {
  const [token, setToken] = useState<string | null>(null);
  if (token) {
    return (
      <div className="flex items-center justify-between">
        <p className="text-2xs text-ok">{isPending ? 'Applying…' : 'Re-authenticated.'}</p>
      </div>
    );
  }
  return (
    <StepUp
      action={action}
      onElevated={(t) => {
        setToken(t);
        onElevated(t);
      }}
    />
  );
}
