/**
 * Works through the candidates the admin selected, one at a time: each opens the
 * standard connector form with its type chosen and its address filled in. Saving
 * creates the connector and moves on; skipping moves on without creating; a
 * failed save stays on the candidate (the form shows the error); stopping hands
 * control back to the result list.
 */
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { Connector, DiscoveryCandidate } from '../../api/model';
import { Button } from '../../components/ui/Button';
import { ConnectorForm } from '../connectors/ConnectorForm';
import { candidateKey } from './candidates';

export function ConnectQueue({
  candidates,
  onCreated,
  onFinished,
  onStop,
}: {
  candidates: DiscoveryCandidate[];
  /** A connector was created for a candidate. */
  onCreated: (created: Connector, candidate: DiscoveryCandidate) => void;
  /** Every candidate was saved or skipped. */
  onFinished: () => void;
  /** The admin left the queue early. */
  onStop: () => void;
}) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  const candidate = candidates[index];
  if (!candidate) return null;

  const advance = () => {
    if (index + 1 >= candidates.length) onFinished();
    else setIndex(index + 1);
  };

  return (
    <section aria-labelledby="connect-queue-title" className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h2 id="connect-queue-title" className="text-sm font-semibold text-ink">
            {t('discovery.queue.heading', { name: candidate.name, address: `${candidate.address}:${candidate.port}` })}
          </h2>
          <p className="text-2xs text-ink-faint" aria-live="polite">
            {t('discovery.queue.position', { index: index + 1, total: candidates.length })}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="secondary" size="sm" onClick={advance}>
            {t('discovery.queue.skip')}
          </Button>
          <Button variant="ghost" size="sm" onClick={onStop}>
            {t('discovery.queue.stop')}
          </Button>
        </div>
      </div>

      {/* Saving is the form's own button; remount per candidate so each starts clean. */}
      <ConnectorForm
        key={candidateKey(candidate)}
        initialType={candidate.type}
        initialValues={{
          [candidate.urlField]: candidate.url,
          name: `${candidate.name} (${candidate.address})`,
        }}
        onCreated={(created) => {
          onCreated(created, candidate);
          advance();
        }}
      />
    </section>
  );
}
