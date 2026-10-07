import { useCallback, useEffect, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import {
  getGetCompliancePacksQueryKey,
  getGetComplianceRulesQueryKey,
  postCompliancePacksIdInstall,
  useGetCompliancePacks,
} from '../../api/generated/compliance/compliance';
import { useGetMe } from '../../api/generated/me/me';
import { Button } from '../../components/ui/Button';
import { Dialog } from '../../components/ui/Dialog';

const PACK_ID = 'certificate-expiry';
const DISMISSED_KEY = 'wiselabz.certificate-expiry-pack-dismissed';

function readDismissed(): boolean {
  try {
    return window.localStorage.getItem(DISMISSED_KEY) === 'true';
  } catch {
    return false;
  }
}

function rememberDismissal() {
  try {
    window.localStorage.setItem(DISMISSED_KEY, 'true');
  } catch {
    // The offer remains usable when browser storage is unavailable.
  }
}

/** Shared opt-in offer used after creating a TLS probe and enabling its widget. */
export function CertificateExpiryPackOffer({
  requested,
  onFinished,
}: {
  requested: boolean;
  onFinished: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const me = useGetMe();
  const mayInstall = (me.data as { role?: string } | undefined)?.role === 'admin';
  const [dismissed, setDismissed] = useState(readDismissed);
  const completed = useRef(false);
  const packs = useGetCompliancePacks({
    query: { enabled: requested && !me.isLoading && mayInstall && !dismissed },
  });
  const pack = packs.data?.items.find((item) => item.id === PACK_ID);

  const finish = useCallback(() => {
    if (completed.current) return;
    completed.current = true;
    onFinished();
  }, [onFinished]);

  useEffect(() => {
    if (!requested) {
      completed.current = false;
      return;
    }
    if (me.isLoading) return;
    if (!mayInstall || dismissed || packs.isError) {
      finish();
    } else if (packs.isSuccess && (!pack || pack.installed)) {
      finish();
    }
  }, [requested, me.isLoading, mayInstall, dismissed, packs.isError, packs.isSuccess, pack, finish]);

  const install = useMutation({
    mutationFn: () => postCompliancePacksIdInstall(PACK_ID),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: getGetCompliancePacksQueryKey() });
      void queryClient.invalidateQueries({ queryKey: getGetComplianceRulesQueryKey() });
      finish();
    },
  });

  const decline = () => {
    rememberDismissal();
    setDismissed(true);
    finish();
  };

  const open = requested && mayInstall && !dismissed && pack?.installed === false;
  return (
    <Dialog open={open} onClose={decline} title={t('compliance.certificateExpiryOffer.title')}>
      <div className="space-y-4">
        <p className="text-sm text-ink-muted">{t('compliance.certificateExpiryOffer.description')}</p>
        {install.isError && (
          <p className="text-sm text-err" role="alert">
            {t('compliance.certificateExpiryOffer.installError')}
          </p>
        )}
        <div className="flex justify-end gap-2">
          <Button variant="ghost" size="md" onClick={decline} disabled={install.isPending}>
            {t('compliance.certificateExpiryOffer.notNow')}
          </Button>
          <Button variant="primary" size="md" onClick={() => install.mutate()} disabled={install.isPending}>
            {install.isPending
              ? t('compliance.certificateExpiryOffer.installing')
              : t('compliance.certificateExpiryOffer.install')}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}
