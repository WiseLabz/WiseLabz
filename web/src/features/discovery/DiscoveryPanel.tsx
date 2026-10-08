/**
 * Network scan panel: pick a private range, start a scan, watch it, and select
 * what it found. Starting goes through step-up (the server decides whether it is
 * needed); progress and candidates arrive over the WebSocket into the discovery
 * store, and the scan is read once on mount (and again after a reconnect or a
 * completion) so a reload or a missed frame cannot leave the panel stale.
 *
 * The panel only selects; turning a selection into connectors is ConnectQueue's job.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { isAxiosError } from 'axios';
import {
  cancelDiscoveryScan,
  startDiscoveryScan,
  useGetDiscoverySuggestions,
  useGetDiscoveryScan,
} from '../../api/generated/discovery/discovery';
import type { DiscoveryCandidate, DiscoveryScan } from '../../api/model';
import { elevationOptions, useStepUpMutation } from '../../components/manager/useStepUpMutation';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { ToneTag } from '../../components/ui/ToneTag';
import { NetworkIcon, XIcon } from '../../components/icons';
import { useDiscovery } from '../../store/discovery';
import { useLive } from '../../store/live';
import { byAddress, candidateKey } from './candidates';

type StartError = { kind: 'field' | 'conflict' | 'rateLimited' | 'other'; message?: string; retryMinutes?: number };

function startErrorFrom(err: unknown): StartError {
  if (!isAxiosError(err)) return { kind: 'other' };
  const data = err.response?.data as
    | { code?: string; message?: string; details?: unknown; retryAfterSeconds?: number }
    | undefined;
  switch (err.response?.status) {
    case 400: {
      const details = Array.isArray(data?.details) ? (data.details as { msg?: string }[]) : [];
      return { kind: 'field', message: details[0]?.msg ?? data?.message };
    }
    case 409:
      return { kind: 'conflict' };
    case 429: {
      const secs = Number(data?.retryAfterSeconds ?? err.response?.headers?.['retry-after']);
      return { kind: 'rateLimited', retryMinutes: Number.isFinite(secs) && secs > 0 ? Math.ceil(secs / 60) : undefined };
    }
    default:
      return { kind: 'other' };
  }
}

const POLL_MS = 3000;

export function DiscoveryPanel({
  onConnect,
}: {
  /** Called with the selected, not yet connected candidates. */
  onConnect: (selected: DiscoveryCandidate[]) => void;
}) {
  const { t } = useTranslation();
  const scan = useDiscovery((s) => s.scan);
  const setScan = useDiscovery((s) => s.setScan);

  // Hydrate from the server on mount and whenever the WebSocket layer invalidates
  // the query (reconnect, completion). No scan is a 404, which means "none".
  // While a scan runs the read is polled: live frames only reach the admin who started
  // it, and a frame can be missed (socket down, or a tiny scan ending before the start
  // response lands), which would leave the panel "running" for good.
  const running = scan?.state === 'running';
  const read = useGetDiscoveryScan({
    query: { retry: false, refetchOnWindowFocus: false, refetchInterval: running ? POLL_MS : false },
  });
  const notFound = isAxiosError(read.error) && read.error.response?.status === 404;
  useEffect(() => {
    // Only a 404 means "no scan"; a 5xx or network error keeps what is on screen.
    // A stale 404 from before the scan started must not wipe the scan just started.
    if (read.isError) {
      if (notFound && !read.isFetching) setScan(null);
    } else if (read.data) setScan(read.data.scan);
  }, [read.data, read.isError, read.isFetching, notFound, setScan]);

  // Frames sent while the socket was down are gone for good: read the scan again
  // whenever the socket comes back.
  const socket = useLive((s) => s.ws);
  const wasOpen = useRef(socket === 'open');
  useEffect(() => {
    if (socket === 'open' && !wasOpen.current) void read.refetch();
    wasOpen.current = socket === 'open';
  }, [socket, read]);

  const suggestions = useGetDiscoverySuggestions({ query: { retry: false, refetchOnWindowFocus: false } });
  const suggested = useMemo(() => suggestions.data?.suggestions ?? [], [suggestions.data]);

  // Null until the admin types or picks a chip; until then the field shows their
  // own range (empty when the server found no private one).
  const [typed, setTyped] = useState<string | null>(null);
  const cidr = typed ?? suggested.find((s) => s.source === 'client')?.cidr ?? '';

  const [startError, setStartError] = useState<StartError | null>(null);
  const start = useStepUpMutation({
    action: 'discovery.scan',
    mutationFn: (range: string, token) => startDiscoveryScan({ cidr: range }, elevationOptions(token)),
    onSuccess: (res) => {
      setStartError(null);
      setScan(res.scan);
    },
    onError: (err) => {
      const e = startErrorFrom(err);
      setStartError(e);
      // Someone else's scan is running: show it instead of an empty panel.
      if (e.kind === 'conflict') void read.refetch();
    },
  });

  const [cancelling, setCancelling] = useState(false);
  const cancel = async () => {
    setCancelling(true);
    try {
      const res = await cancelDiscoveryScan();
      setScan(res.scan);
    } catch {
      void read.refetch();
    } finally {
      setCancelling(false);
    }
  };

  const candidates = useMemo(() => [...(scan?.candidates ?? [])].sort(byAddress), [scan?.candidates]);
  const selectable = candidates.filter((c) => !c.connectorId);

  // Selection belongs to one scan; a new scan starts with nothing selected.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [selectionScan, setSelectionScan] = useState<string | undefined>(undefined);
  if (scan?.id !== selectionScan) {
    setSelectionScan(scan?.id);
    setSelected(new Set());
  }
  const chosen = selectable.filter((c) => selected.has(candidateKey(c)));
  const toggle = (c: DiscoveryCandidate) =>
    setSelected((prev) => {
      const next = new Set(prev);
      const k = candidateKey(c);
      if (next.has(k)) next.delete(k);
      else next.add(k);
      return next;
    });

  const submit = () => {
    setStartError(null);
    start.mutate(cidr.trim());
  };

  return (
    <Panel className="p-5">
      <div className="flex items-start gap-2.5">
        <NetworkIcon size={18} className="mt-0.5 text-accent-primary" />
        <div>
          <h2 className="text-sm font-semibold text-ink">{t('discovery.title')}</h2>
          <p className="mt-0.5 text-2xs text-ink-faint">{t('discovery.lead')}</p>
          <p className="mt-1 text-2xs text-ink-faint">{t('discovery.note')}</p>
        </div>
      </div>

      <form
        className="mt-4"
        onSubmit={(e) => {
          e.preventDefault();
          if (!running && cidr.trim()) submit();
        }}
      >
        <label className="block" htmlFor="discovery-range">
          <span className="mb-1 block text-2xs text-ink-faint">{t('discovery.rangeLabel')}</span>
        </label>
        <div className="flex gap-2">
          <input
            id="discovery-range"
            value={cidr}
            onChange={(e) => setTyped(e.target.value)}
            placeholder="192.168.1.0/24"
            autoComplete="off"
            spellCheck={false}
            disabled={running}
            aria-invalid={startError?.kind === 'field'}
            aria-describedby={startError?.kind === 'field' ? 'discovery-range-error' : undefined}
            className="h-9 min-w-0 flex-1 rounded-sm border border-line bg-surface px-2.5 font-mono text-sm text-ink outline-none placeholder:text-ink-faint focus-visible:border-accent-primary-soft disabled:opacity-50"
          />
          <Button type="submit" variant="primary" size="md" disabled={running || !cidr.trim() || start.isPending}>
            {scan && !running ? t('discovery.scanAgain') : t('discovery.start')}
          </Button>
        </div>
        {startError?.kind === 'field' && (
          <p id="discovery-range-error" className="mt-1 text-2xs text-err" role="alert">
            {startError.message ?? t('discovery.errors.invalidRange')}
          </p>
        )}
        {suggested.length > 0 && !running && (
          <div className="mt-2 flex flex-wrap items-center gap-1.5" role="group" aria-label={t('discovery.suggestions')}>
            {suggested.map((s) => (
              <button
                key={s.cidr}
                type="button"
                onClick={() => setTyped(s.cidr)}
                className="rounded-sm border border-line-soft px-2 py-0.5 font-mono text-2xs text-ink-muted transition-colors hover:border-line-strong hover:text-ink"
              >
                {s.cidr}
                <span className="ml-1.5 text-ink-faint">
                  {s.source === 'client' ? t('discovery.source.client') : t('discovery.source.server')}
                </span>
              </button>
            ))}
          </div>
        )}
      </form>

      {startError?.kind === 'conflict' && (
        <p className="mt-3 text-2xs text-warn" role="alert">{t('discovery.errors.conflict')}</p>
      )}
      {startError?.kind === 'rateLimited' && (
        <p className="mt-3 text-2xs text-warn" role="alert">
          {startError.retryMinutes
            ? t('discovery.errors.rateLimited', { minutes: startError.retryMinutes })
            : t('discovery.errors.rateLimitedSoon')}
        </p>
      )}
      {startError?.kind === 'other' && (
        <p className="mt-3 text-2xs text-err" role="alert">{t('discovery.errors.generic')}</p>
      )}

      {scan && (
        <div className="mt-4 border-t border-line-soft pt-4">
          <ScanStatus
            scan={scan}
            empty={candidates.length === 0}
            cancelling={cancelling}
            onCancel={() => void cancel()}
          />

          {candidates.length > 0 && (
            <ul className="mt-3 space-y-1.5">
              {candidates.map((c) => (
                <li key={candidateKey(c)}>
                  <CandidateRow
                    candidate={c}
                    checked={selected.has(candidateKey(c))}
                    onToggle={() => toggle(c)}
                  />
                </li>
              ))}
            </ul>
          )}

          {candidates.length > 0 && (
            <div className="mt-3 flex items-center justify-end">
              <Button variant="primary" size="md" disabled={chosen.length === 0} onClick={() => onConnect(chosen)}>
                {t('discovery.connectSelected', { count: chosen.length })}
              </Button>
            </div>
          )}
        </div>
      )}
      {start.dialog}
    </Panel>
  );
}

/**
 * Progress while running (the progressbar carries it; the changing counts are not
 * announced), and the final outcome in a status region that stays mounted so a
 * screen reader announces it when it appears.
 */
function ScanStatus({
  scan,
  empty,
  cancelling,
  onCancel,
}: {
  scan: DiscoveryScan;
  /** No candidates were found (or have arrived yet). */
  empty: boolean;
  cancelling: boolean;
  onCancel: () => void;
}) {
  const { t } = useTranslation();
  const percent = scan.total > 0 ? Math.min(100, Math.round((scan.done / scan.total) * 100)) : 0;
  const running = scan.state === 'running';
  return (
    <>
      {running && (
        <div>
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm text-ink">
              {t('discovery.running', { range: scan.cidr })}
              <span className="ml-2 text-2xs text-ink-faint">
                {t('discovery.progress', { done: scan.done, total: scan.total })}
              </span>
            </p>
            <Button variant="ghost" size="sm" onClick={onCancel} disabled={cancelling}>
              <XIcon size={12} />
              {t('discovery.cancel')}
            </Button>
          </div>
          <div
            className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-canvas-sunken"
            role="progressbar"
            aria-label={t('discovery.title')}
            aria-valuenow={percent}
            aria-valuemin={0}
            aria-valuemax={100}
          >
            <div className="h-full rounded-full bg-accent-primary transition-[width] duration-300" style={{ width: `${percent}%` }} />
          </div>
        </div>
      )}
      <div role="status">
        {!running && (
          <div className="flex flex-wrap items-center gap-2">
            <ToneTag
              tone={scan.state === 'failed' ? 'err' : scan.state === 'cancelled' ? 'warn' : 'ok'}
              label={t(`discovery.state.${scan.state}`)}
            />
            <span className="text-2xs text-ink-faint">
              {t('discovery.summary', { range: scan.cidr, done: scan.done, total: scan.total, answered: scan.answered })}
            </span>
            {scan.partial && <span className="text-2xs text-warn">{t('discovery.partial')}</span>}
          </div>
        )}
        {!running && empty && scan.state !== 'failed' && (
          <div className="mt-3 rounded-sm border border-line-soft p-3">
            <p className="text-sm font-medium text-ink">{t('discovery.noResults')}</p>
            <p className="mt-1 text-2xs text-ink-faint">
              {t('discovery.noResultsDetail', { answered: scan.answered, total: scan.total })}
            </p>
          </div>
        )}
      </div>
    </>
  );
}

function CandidateRow({
  candidate,
  checked,
  onToggle,
}: {
  candidate: DiscoveryCandidate;
  checked: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const connected = !!candidate.connectorId;
  const id = `discovery-${candidateKey(candidate).replace(/[^a-z0-9]/gi, '-')}`;
  return (
    <div className="flex items-center gap-3 rounded-sm border border-line-soft px-3 py-2">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        disabled={connected}
        onChange={onToggle}
        className="h-4 w-4 accent-[var(--color-accent-primary)] disabled:opacity-40"
      />
      <label htmlFor={id} className="min-w-0 flex-1 text-sm text-ink">
        <span className="font-medium">{candidate.name}</span>
        <span className="ml-2 font-mono text-2xs text-ink-muted">
          {candidate.address}:{candidate.port}
        </span>
      </label>
      {connected && (
        <span className="flex items-center gap-2">
          <ToneTag tone="ok" label={t('discovery.alreadyConnected')} />
          <Link to={`/services/${candidate.connectorId}`} className="text-2xs text-accent-primary hover:underline">
            {t('discovery.viewConnector')}
          </Link>
        </span>
      )}
    </div>
  );
}
