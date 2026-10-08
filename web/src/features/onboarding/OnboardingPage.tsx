/**
 * First-run stepper. Shown when the install has zero connectors (RequireOnboarded
 * routes here, and lives OUTSIDE that gate so it never loops). Four steps —
 * welcome → connect → first sync → done — but the only hard gate is the genuine
 * one: the app is useless with zero connectors, so the connector must be created
 * to leave. Everything after is guidance the user can skip.
 *
 * Reuses the real <ConnectorForm/> (no dumbed-down onboarding variant) and drives
 * the first sync over the mock WebSocket timeline, reading progress from the live
 * store exactly as the dashboard will.
 */
import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useLive, type SyncJob } from '../../store/live';
import { postConnectorsConnectorIdSync } from '../../api/generated/connectors/connectors';
import { useIsInstanceAdmin } from '../../hooks/useRole';
import { ConnectorForm } from '../connectors/ConnectorForm';
import { DiscoveryFlow } from '../discovery/DiscoveryFlow';
import { Button } from '../../components/ui/Button';
import { CheckIcon, SyncIcon, ArrowRightIcon } from '../../components/icons';
import type { Connector } from '../../api/model';

type Step = 'welcome' | 'connect' | 'sync' | 'done';
const ORDER: Step[] = ['welcome', 'connect', 'sync', 'done'];

export function OnboardingPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const isInstanceAdmin = useIsInstanceAdmin();
  const [step, setStep] = useState<Step>('welcome');
  // Every connector created on the way to the sync step: one when added by hand,
  // any number when connected from a network scan.
  const [connectors, setConnectors] = useState<Connector[]>([]);
  // The queue reports a save and its end in one event, before state has flushed.
  const createdRef = useRef<Connector[]>([]);
  const addConnector = (c: Connector) => {
    createdRef.current = [...createdRef.current, c];
    setConnectors(createdRef.current);
  };

  return (
    <div className="min-h-dvh bg-canvas px-6 py-10">
      <div className="mx-auto w-full max-w-2xl">
        <header className="mb-8 flex items-center justify-between">
          <span className="font-mono text-sm font-semibold tracking-tight text-ink">{t('app.name')}</span>
          <Stepper current={step} />
        </header>

        {step === 'welcome' && (
          <WelcomeStep
            onStart={() => setStep('connect')}
          />
        )}

        {step === 'connect' && (
          <section aria-labelledby="ob-connect-title">
            <h1 id="ob-connect-title" className="text-xl font-semibold tracking-tight text-ink">
              {t('onboarding.connect.title')}
            </h1>
            <p className="mb-5 mt-1 text-sm text-ink-muted">{t('onboarding.connect.body')}</p>
            {isInstanceAdmin && (
              <div className="mb-6 space-y-3">
                <DiscoveryFlow
                  onConnectorCreated={addConnector}
                  onQueueFinished={() => {
                    // Nothing created (everything skipped): stay on this step.
                    if (createdRef.current.length > 0) setStep('sync');
                  }}
                />
                {connectors.length > 0 && (
                  <Button variant="primary" size="md" onClick={() => setStep('sync')}>
                    {t('onboarding.connect.continueWithConnected', { count: connectors.length })}
                    <ArrowRightIcon size={14} />
                  </Button>
                )}
                <h2 className="pt-2 text-sm font-semibold text-ink">{t('onboarding.connect.manualTitle')}</h2>
              </div>
            )}
            <ConnectorForm
              onCreated={(c) => {
                addConnector(c);
                setStep('sync');
              }}
            />
          </section>
        )}

        {step === 'sync' && connectors.length > 0 && (
          <SyncStep
            connectors={connectors}
            onContinue={() => setStep('done')}
            onSkip={() => navigate('/dashboard')}
          />
        )}

        {step === 'done' && (
          <DoneStep onDashboard={() => navigate('/dashboard')} onAddAnother={() => navigate('/services/new')} />
        )}
      </div>
    </div>
  );
}

function Stepper({ current }: { current: Step }) {
  const { t } = useTranslation();
  const idx = ORDER.indexOf(current);
  return (
    <ol className="flex items-center gap-2" aria-label={t('onboarding.welcomeKicker')}>
      {ORDER.map((s, i) => {
        const state = i < idx ? 'done' : i === idx ? 'current' : 'upcoming';
        return (
          <li key={s} className="flex items-center gap-2">
            <span
              className={
                'flex h-5 items-center gap-1.5 rounded-full px-2 text-2xs font-medium ' +
                (state === 'current'
                  ? 'bg-accent-primary-tint text-accent-primary'
                  : state === 'done'
                    ? 'text-ok'
                    : 'text-ink-faint')
              }
            >
              {state === 'done' ? <CheckIcon size={12} /> : <span aria-hidden>{i + 1}</span>}
              {t(`onboarding.steps.${s}`)}
            </span>
            {i < ORDER.length - 1 && <span className="h-px w-4 bg-line-strong" aria-hidden />}
          </li>
        );
      })}
    </ol>
  );
}

function WelcomeStep({ onStart }: { onStart: () => void }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ob-welcome-title" className="rounded-lg border border-line bg-surface p-6">
      <p className="font-mono text-2xs text-accent-primary">{t('onboarding.welcomeKicker')}</p>
      <h1 id="ob-welcome-title" className="mt-2 text-2xl font-semibold tracking-tight text-ink">
        {t('onboarding.title')}
      </h1>
      <p className="mt-2 text-sm text-ink-muted">{t('onboarding.welcome.lead')}</p>
      <ul className="mt-5 space-y-2.5">
        {['point1', 'point2', 'point3'].map((p) => (
          <li key={p} className="flex items-start gap-2.5 text-sm text-ink">
            <CheckIcon size={15} className="mt-0.5 shrink-0 text-accent-primary" />
            {t(`onboarding.welcome.${p}`)}
          </li>
        ))}
      </ul>
      <div className="mt-6 flex items-center gap-2">
        <Button variant="primary" size="md" onClick={onStart}>
          {t('onboarding.welcome.start')}
          <ArrowRightIcon size={14} />
        </Button>
        <Button variant="ghost" size="md" onClick={onStart}>
          {t('onboarding.welcome.skip')}
        </Button>
      </div>
    </section>
  );
}

function SyncStep({
  connectors,
  onContinue,
  onSkip,
}: {
  connectors: Connector[];
  onContinue: () => void;
  onSkip: () => void;
}) {
  const { t } = useTranslation();
  const jobs = useLive((s) => s.jobs);
  const jobOf = (c: Connector) => jobs[c.id] ?? jobs.global;

  // Trigger a first sync for each connector on mount - each returns 202, progress
  // streams over WS.
  const ids = connectors.map((c) => c.id).join(',');
  useEffect(() => {
    ids.split(',').forEach((id) => {
      postConnectorsConnectorIdSync(id).catch(() => {
        // Sync initiation failed, but user can still skip — WS events handle progress.
      });
    });
  }, [ids]);

  const done = connectors.every((c) => jobOf(c)?.phase === 'done');
  const failed = connectors.some((c) => jobOf(c)?.phase === 'error');

  return (
    <section aria-labelledby="ob-sync-title" className="rounded-lg border border-line bg-surface p-6">
      <div className="flex items-center gap-2.5">
        <SyncIcon size={18} className={done ? 'text-ok' : failed ? 'text-err' : 'animate-spin text-accent-primary motion-reduce:animate-none'} />
        <h1 id="ob-sync-title" className="text-lg font-semibold tracking-tight text-ink">
          {done
            ? t('onboarding.sync.complete')
            : failed
              ? t('onboarding.sync.failed')
              : connectors.length > 1
                ? t('onboarding.sync.titleMany', { count: connectors.length })
                : t('onboarding.sync.title')}
        </h1>
      </div>

      <ul className="mt-4 space-y-4">
        {connectors.map((connector) => (
          <li key={connector.id}>
            <SyncRow connector={connector} job={jobOf(connector)} named={connectors.length > 1} />
          </li>
        ))}
      </ul>

      <div className="mt-6 flex items-center gap-2">
        <Button variant="primary" size="md" disabled={!done} onClick={onContinue}>
          {t('onboarding.sync.continue')}
          <ArrowRightIcon size={14} />
        </Button>
        <Button variant="ghost" size="md" onClick={onSkip}>
          {t('onboarding.sync.skip')}
        </Button>
      </div>
    </section>
  );
}

/** One connector's sync progress. `named` prefixes the status with its name when several share the step. */
function SyncRow({ connector, job, named }: { connector: Connector; job: SyncJob | undefined; named: boolean }) {
  const { t } = useTranslation();
  const done = job?.phase === 'done';
  const failed = job?.phase === 'error';
  const percent = job?.percent ?? 0;
  const label = (text: string) => (named ? `${connector.name}: ${text}` : text);
  return (
    <div>
      <div className="h-1.5 w-full overflow-hidden rounded-full bg-canvas-sunken" role="progressbar" aria-label={connector.name} aria-valuenow={percent} aria-valuemin={0} aria-valuemax={100}>
        <div
          className="h-full rounded-full bg-accent-primary transition-[width] duration-300"
          style={{ width: `${done ? 100 : percent}%`, backgroundColor: done ? 'var(--color-ok)' : failed ? 'var(--color-err)' : undefined }}
        />
      </div>
      <p className="mt-2 text-sm text-ink-muted" aria-live="polite">
        {done
          ? label(t('onboarding.sync.completeDetail'))
          : failed
            ? label(t('onboarding.sync.errorDetail', { error: job?.message ?? '' }))
            : job
              ? t('onboarding.sync.running', { name: connector.name, phase: job.phase })
              : label(t('onboarding.sync.waiting'))}
      </p>
    </div>
  );
}

function DoneStep({ onDashboard, onAddAnother }: { onDashboard: () => void; onAddAnother: () => void }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ob-done-title" className="rounded-lg border border-line bg-surface p-6">
      <span className="flex h-9 w-9 items-center justify-center rounded-full bg-ok-tint text-ok">
        <CheckIcon size={18} />
      </span>
      <h1 id="ob-done-title" className="mt-3 text-xl font-semibold tracking-tight text-ink">
        {t('onboarding.done.title')}
      </h1>
      <p className="mt-2 text-sm text-ink-muted">{t('onboarding.done.body')}</p>
      <div className="mt-6 flex items-center gap-2">
        <Button variant="primary" size="md" onClick={onDashboard}>
          {t('onboarding.done.toDashboard')}
          <ArrowRightIcon size={14} />
        </Button>
        <Button variant="ghost" size="md" onClick={onAddAnother}>
          {t('onboarding.done.addAnother')}
        </Button>
      </div>
    </section>
  );
}
