/**
 * First-run entry. Shown when the install has zero connectors (RequireOnboarded
 * routes here). Minimal for now — Phase 4 replaces this with the full stepper
 * (welcome → add connector → first sync → done). Lives OUTSIDE RequireOnboarded so
 * it never loops.
 */
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { PublicLayout } from '../auth/PublicLayout';
import { Button } from '../../components/ui/Button';

export function OnboardingPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  return (
    <PublicLayout>
      <div className="overflow-hidden rounded-lg border border-[var(--color-line)] bg-[var(--color-surface)] p-6 shadow-[var(--shadow-pop)]">
        <p className="font-mono text-2xs uppercase tracking-wider text-[var(--color-signal)]">
          {t('onboarding.welcomeKicker')}
        </p>
        <h1 className="mt-2 text-xl font-semibold tracking-tight text-[var(--color-ink)]">
          {t('onboarding.title')}
        </h1>
        <p className="mt-2 text-sm text-[var(--color-ink-muted)]">{t('onboarding.body')}</p>
        <Button
          variant="primary"
          size="md"
          className="mt-5 w-full justify-center"
          onClick={() => navigate('/services/new')}
        >
          {t('onboarding.addFirst')}
        </Button>
      </div>
    </PublicLayout>
  );
}
