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
import type {
  DiscoveryAddOn,
  DiscoveryCandidate,
  DiscoveryScan,
  DiscoveryScanRequest,
  DiscoveryTLSTarget,
} from '../../api/model';
import { elevationOptions, useStepUpMutation } from '../../components/manager/useStepUpMutation';
import { Button } from '../../components/ui/Button';
import { Panel } from '../../components/ui/Panel';
import { ToneTag } from '../../components/ui/ToneTag';
import { NetworkIcon, XIcon } from '../../components/icons';
import { useDiscovery } from '../../store/discovery';
import { useLive } from '../../store/live';
import { addOnKey, byAddress, candidateKey, candidateSource, compareAddresses, endpointLabel, targetKey } from './candidates';
import { calculateRangeBudget } from './rangeBudget';

type StartError = {
  kind: 'field' | 'conflict' | 'rateLimited' | 'other';
  message?: string;
  rangeErrors?: Record<number, string>;
  retryMinutes?: number;
};

function startErrorFrom(err: unknown): StartError {
  if (!isAxiosError(err)) return { kind: 'other' };
  const data = err.response?.data as
    | { code?: string; message?: string; details?: unknown; retryAfterSeconds?: number }
    | undefined;
  switch (err.response?.status) {
    case 400: {
      const details = Array.isArray(data?.details) ? (data.details as { field?: string; msg?: string }[]) : [];
      const rangeErrors: Record<number, string> = {};
      for (const detail of details) {
        const match = detail.field?.match(/^ranges\[(\d+)\]$/);
        const index = match ? Number(match[1]) : detail.field === 'cidr' ? 0 : undefined;
        if (index !== undefined && detail.msg) rangeErrors[index] = detail.msg;
      }
      return {
        kind: 'field',
        message: details.find((detail) => !detail.field?.match(/^ranges\[\d+\]$/) && detail.field !== 'cidr')?.msg ?? data?.message,
        rangeErrors,
      };
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
const tlsTargetKey = (target: DiscoveryTLSTarget) => targetKey(target.address, target.port);
type TLSSelection = { scanId?: string; seen: Set<string>; selected: Set<string> };

export function DiscoveryPanel({
  onConnect,
}: {
  /** Called with the selected, not yet connected candidates. */
  onConnect: (
    selected: DiscoveryCandidate[],
    tlsTargets?: DiscoveryTLSTarget[],
    addOns?: { candidate: DiscoveryCandidate; addOn: DiscoveryAddOn }[],
  ) => void;
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

  // The caller's network is the only suggestion prefilled. Other suggestions are
  // added explicitly, and removing the prefilled chip stays removed.
  const [ranges, setRanges] = useState<string[] | null>(null);
  const clientSuggestion = suggested.find((s) => s.source === 'client')?.cidr;
  const selectedRanges = useMemo(() => ranges ?? (clientSuggestion ? [clientSuggestion] : []), [ranges, clientSuggestion]);
  const [rangeDraft, setRangeDraft] = useState('');
  const [includeInventory, setIncludeInventory] = useState(true);
  const [askNetwork, setAskNetwork] = useState(true);
  const knownHosts = suggestions.data?.knownHosts ?? 0;
  // With no known hosts the option is disabled, which means effectively off.
  const effectiveInventory = includeInventory && knownHosts > 0;
  const budget = useMemo(
    () => calculateRangeBudget(selectedRanges, knownHosts, effectiveInventory),
    [selectedRanges, knownHosts, effectiveInventory],
  );
  const budgetProgress = Math.min(100, Math.round((budget.used / 1024) * 100));

  const [startError, setStartError] = useState<StartError | null>(null);
  const start = useStepUpMutation({
    action: 'discovery.scan',
    mutationFn: (request: DiscoveryScanRequest, token) => startDiscoveryScan(request, elevationOptions(token)),
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
  const tlsTargets = useMemo(
    () => [...(scan?.tlsTargets ?? [])].sort((a, b) => compareAddresses(a.address, b.address) || a.port - b.port),
    [scan?.tlsTargets],
  );

  // Selection belongs to one scan; a new scan starts with nothing selected.
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [addOnOverrides, setAddOnOverrides] = useState<Map<string, boolean>>(new Map());
  const [tlsQueueSelected, setTLSQueueSelected] = useState(false);
  const [selectionScan, setSelectionScan] = useState<string | undefined>(undefined);
  if (scan?.id !== selectionScan) {
    setSelectionScan(scan?.id);
    setSelected(new Set());
    setAddOnOverrides(new Map());
    setTLSQueueSelected(false);
  }
  const [tlsSelection, setTLSSelection] = useState<TLSSelection>({ seen: new Set(), selected: new Set() });
  const currentTLSSelection = tlsSelection.scanId === scan?.id
    ? tlsSelection
    : { scanId: scan?.id, seen: new Set<string>(), selected: new Set<string>() };
  const tlsSelected = new Set(currentTLSSelection.selected);
  for (const target of tlsTargets) {
    const key = tlsTargetKey(target);
    if (!target.monitoredBy && !currentTLSSelection.seen.has(key)) tlsSelected.add(key);
  }
  const chosen = selectable.filter((c) => selected.has(candidateKey(c)));
  const chosenAddOns = candidates.flatMap((candidate) =>
    (candidate.addOns ?? [])
      .filter((addOn) => !addOn.connectorId && (addOnOverrides.get(addOnKey(candidate, addOn)) ?? selected.has(candidateKey(candidate))))
      .map((addOn) => ({ candidate, addOn })),
  );
  const availableTLSTargets = tlsTargets.filter((target) => !target.monitoredBy);
  const chosenTLSTargets = tlsQueueSelected
    ? availableTLSTargets.filter((target) => tlsSelected.has(tlsTargetKey(target)))
    : [];
  const totalSelected = chosen.length + chosenAddOns.length + (chosenTLSTargets.length > 0 ? 1 : 0);
  const hasAddOnRows = candidates.some((candidate) => (candidate.addOns?.length ?? 0) > 0);
  const toggle = (c: DiscoveryCandidate) =>
    setSelected((prev) => {
      const next = new Set(prev);
      const k = candidateKey(c);
      if (next.has(k)) next.delete(k);
      else next.add(k);
      return next;
    });
  const toggleAddOn = (candidate: DiscoveryCandidate, addOn: DiscoveryAddOn) =>
    setAddOnOverrides((previous) => new Map(previous).set(addOnKey(candidate, addOn),
      !(previous.get(addOnKey(candidate, addOn)) ?? selected.has(candidateKey(candidate)))));
  const toggleTLSQueue = () => setTLSQueueSelected((previous) => !previous);
  const toggleAllTLS = () =>
    setTLSSelection((previous) => {
      const state = previous.scanId === scan?.id ? previous : currentTLSSelection;
      const selected = new Set(state.selected);
      const seen = new Set(state.seen);
      const allSelected = availableTLSTargets.every((target) => tlsSelected.has(tlsTargetKey(target)));
      for (const target of availableTLSTargets) {
        const key = tlsTargetKey(target);
        if (allSelected) selected.delete(key);
        else selected.add(key);
        seen.add(key);
      }
      return { scanId: scan?.id, seen, selected };
    });
  const toggleTLS = (target: DiscoveryTLSTarget) =>
    setTLSSelection((previous) => {
      const state = previous.scanId === scan?.id ? previous : currentTLSSelection;
      const selected = new Set(state.selected);
      const seen = new Set(state.seen);
      const key = tlsTargetKey(target);
      if (tlsSelected.has(key)) selected.delete(key);
      else selected.add(key);
      seen.add(key);
      return { scanId: scan?.id, seen, selected };
    });
  const connectSelected = () => {
    if (hasAddOnRows) {
      onConnect(chosen, chosenTLSTargets.length > 0 ? chosenTLSTargets : undefined, chosenAddOns);
    } else if (chosenTLSTargets.length > 0) {
      onConnect(chosen, chosenTLSTargets);
    } else {
      onConnect(chosen);
    }
  };

  const submit = () => {
    const nextRanges = [...new Set([
      ...selectedRanges,
      ...rangeDraft.split(/[\n,]/).map((range) => range.trim()).filter(Boolean),
    ])];
    setRanges(nextRanges);
    setRangeDraft('');
    setStartError(null);
    start.mutate({ ranges: nextRanges, includeInventory: effectiveInventory, askNetwork });
  };

  const addRanges = (values: string[]) => {
    const additions = values.map((range) => range.trim()).filter(Boolean);
    if (!additions.length) return;
    setRanges([...new Set([...selectedRanges, ...additions])]);
    setStartError(null);
  };

  const removeRange = (range: string) => {
    setRanges(selectedRanges.filter((value) => value !== range));
    setStartError(null);
  };

  const canStart = selectedRanges.length > 0 || effectiveInventory;

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
          if (!running && canStart) submit();
        }}
      >
        <label className="block" htmlFor="discovery-range">
          <span className="mb-1 block text-2xs text-ink-faint">{t('discovery.rangeLabel')}</span>
        </label>
        <div className="flex gap-2">
          <div
            className="flex min-h-9 min-w-0 flex-1 flex-wrap items-center gap-1.5 rounded-sm border border-line bg-surface px-2 py-1 focus-within:border-accent-primary-soft"
            role="group"
            aria-label={t('discovery.rangeLabel')}
          >
            {selectedRanges.map((range, index) => {
              const error = startError?.rangeErrors?.[index];
              const errorId = error ? `discovery-range-error-${index}` : undefined;
              return (
                <span
                  key={range}
                  className={`inline-flex items-center gap-1 rounded-sm border px-1.5 py-0.5 font-mono text-2xs ${error ? 'border-err text-err' : 'border-line-soft text-ink'}`}
                  aria-invalid={error ? true : undefined}
                  aria-describedby={errorId}
                >
                  {range}
                  {!running && (
                    <button
                      type="button"
                      aria-label={t('discovery.removeRange', { range })}
                      onClick={() => removeRange(range)}
                      className="rounded-sm text-ink-faint hover:text-ink"
                    >
                      <XIcon size={11} />
                    </button>
                  )}
                </span>
              );
            })}
            <input
              id="discovery-range"
              value={rangeDraft}
              onChange={(e) => {
                const value = e.target.value;
                if (value.includes(',')) {
                  const parts = value.split(',');
                  addRanges(parts.slice(0, -1));
                  setRangeDraft(parts[parts.length - 1] ?? '');
                } else {
                  setRangeDraft(value);
                }
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ',') {
                  e.preventDefault();
                  addRanges(rangeDraft.split(/[\n,]/));
                  setRangeDraft('');
                }
              }}
              placeholder="192.168.1.0/24"
              autoComplete="off"
              spellCheck={false}
              disabled={running}
              aria-invalid={startError?.kind === 'field' && Object.keys(startError.rangeErrors ?? {}).length === 0}
              aria-describedby={startError?.kind === 'field' && Object.keys(startError.rangeErrors ?? {}).length === 0 ? 'discovery-range-error' : undefined}
              className="h-7 min-w-36 flex-1 bg-transparent font-mono text-sm text-ink outline-none placeholder:text-ink-faint disabled:opacity-50"
            />
          </div>
          <Button type="submit" variant="primary" size="md" disabled={running || !canStart || start.isPending}>
            {scan && !running ? t('discovery.scanAgain') : t('discovery.start')}
          </Button>
        </div>
        {selectedRanges.map((range, index) => {
          const message = startError?.rangeErrors?.[index];
          return message ? (
            <p key={range} id={`discovery-range-error-${index}`} className="mt-1 text-2xs text-err" role="alert">
              {message}
            </p>
          ) : null;
        })}
        {startError?.kind === 'field' && Object.keys(startError.rangeErrors ?? {}).length === 0 && (
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
                onClick={() => addRanges([s.cidr])}
                className="rounded-sm border border-line-soft px-2 py-0.5 font-mono text-2xs text-ink-muted transition-colors hover:border-line-strong hover:text-ink"
              >
                {s.cidr}
                <span className="ml-1.5 text-ink-faint">
                  {t(`discovery.source.${s.source}`)}
                </span>
              </button>
            ))}
          </div>
        )}
        <p className="mt-1 text-2xs text-ink-faint">{t('discovery.rangeHelp')}</p>
        <div className="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-2xs text-ink-muted">
          <label className="inline-flex items-center gap-2">
            <input
              type="checkbox"
              checked={effectiveInventory}
              onChange={(e) => setIncludeInventory(e.target.checked)}
              disabled={running || knownHosts === 0}
              className="h-3.5 w-3.5 accent-[var(--color-accent-primary)] disabled:opacity-40"
            />
            {t('discovery.includeInventory', { count: knownHosts })}
          </label>
          <label className="inline-flex items-center gap-2">
            <input
              type="checkbox"
              checked={askNetwork}
              onChange={(e) => setAskNetwork(e.target.checked)}
              disabled={running}
              className="h-3.5 w-3.5 accent-[var(--color-accent-primary)] disabled:opacity-40"
            />
            {t('discovery.askNetwork')}
          </label>
        </div>
        <p className="mt-3 text-2xs text-ink-faint">
          <span className="mr-1">{t('discovery.budgetLabel')}:</span>
          <span>{t('discovery.budget', { used: budget.used })}</span>
          <span className="ml-2">{t('discovery.budgetHint')}</span>
        </p>
        <div
          className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-canvas-sunken"
          role="meter"
          aria-label={t('discovery.budgetLabel')}
          aria-valuenow={Math.min(1024, budget.used)}
          aria-valuemin={0}
          aria-valuemax={1024}
        >
          <div className="h-full rounded-full bg-accent-primary" style={{ width: `${budgetProgress}%` }} />
        </div>
        {budget.skippedInventory > 0 && (
          <p className="mt-1 text-2xs text-warn">{t('discovery.skippedInventory', { count: budget.skippedInventory })}</p>
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
                  {(c.addOns ?? []).map((addOn) => (
                    <AddOnRow
                      key={addOnKey(c, addOn)}
                      addOn={addOn}
                      checked={addOn.connectorId ? false : (addOnOverrides.get(addOnKey(c, addOn)) ?? selected.has(candidateKey(c)))}
                      onToggle={() => toggleAddOn(c, addOn)}
                    />
                  ))}
                </li>
              ))}
            </ul>
          )}

          {tlsTargets.length > 0 && (
            <CertificateTargets
              targets={tlsTargets}
              available={availableTLSTargets}
              selected={tlsSelected}
              queueSelected={tlsQueueSelected}
              onToggleQueue={toggleTLSQueue}
              onToggleAll={toggleAllTLS}
              onToggle={toggleTLS}
            />
          )}

          {(candidates.length > 0 || tlsTargets.length > 0) && (
            <div className="mt-3 flex items-center justify-end">
              <Button variant="primary" size="md" disabled={totalSelected === 0} onClick={connectSelected}>
                {t('discovery.connectSelected', { count: totalSelected })}
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
  const scanLabel = (scan.ranges ?? [scan.cidr]).filter(Boolean).join(', ') || t('discovery.origin.inventory');
  return (
    <>
      {running && (
        <div>
          <div className="flex items-center justify-between gap-3">
            <p className="text-sm text-ink">
              {t('discovery.running', { range: scanLabel })}
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
              {t('discovery.summary', { range: scanLabel, done: scan.done, total: scan.total, answered: scan.answered })}
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
        {!running && (scan.skipped?.inventory ?? 0) > 0 && (
          <p className="mt-2 text-2xs text-warn">{t('discovery.skippedInventory', { count: scan.skipped?.inventory })}</p>
        )}
        {!running && (scan.skipped?.passive ?? 0) > 0 && (
          <p className="mt-1 text-2xs text-warn">{t('discovery.skippedPassive', { count: scan.skipped?.passive })}</p>
        )}
        {!running && scan.askNetwork && (
          <div className="mt-2">
            <p className="text-2xs text-ink-faint">{t('discovery.passiveAnswered', { count: scan.passiveAnswered ?? 0 })}</p>
            {(scan.passiveAnswered ?? 0) === 0 && (
              <p className="mt-1 text-2xs text-ink-faint">{t('discovery.hostNetworkingHint')}</p>
            )}
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
          {endpointLabel(candidate.address, candidate.port)}
        </span>
        <span className="ml-2 rounded-sm bg-canvas-sunken px-1.5 py-0.5 text-2xs text-ink-faint">
          {t(`discovery.origin.${candidateSource(candidate)}`)}
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

function AddOnRow({
  addOn,
  checked,
  onToggle,
}: {
  addOn: DiscoveryAddOn;
  checked: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const name = addOn.type === 'dnsresolver' ? t('discovery.dnsResolverAddon') : addOn.type;
  return (
    <div className="ml-7 mt-1 flex items-center gap-3 rounded-sm border border-line-soft px-3 py-1.5 text-2xs">
      {addOn.connectorId ? (
        <>
          <span className="min-w-0 flex-1 text-ink-muted">{name}</span>
          <ToneTag tone="ok" label={t('discovery.alreadyConnected')} />
          <Link to={`/services/${addOn.connectorId}`} className="text-accent-primary hover:underline">
            {t('discovery.viewConnector')}
          </Link>
        </>
      ) : (
        <label className="flex min-w-0 flex-1 items-center gap-2 text-ink">
          <input
            type="checkbox"
            checked={checked}
            onChange={onToggle}
            aria-label={`${name} ${addOn.url}`}
            className="h-3.5 w-3.5 accent-[var(--color-accent-primary)]"
          />
          <span>{name}</span>
          <span className="truncate font-mono text-ink-faint">{addOn.url}</span>
        </label>
      )}
    </div>
  );
}

function CertificateTargets({
  targets,
  available,
  selected,
  queueSelected,
  onToggleQueue,
  onToggleAll,
  onToggle,
}: {
  targets: DiscoveryTLSTarget[];
  available: DiscoveryTLSTarget[];
  selected: Set<string>;
  queueSelected: boolean;
  onToggleQueue: () => void;
  onToggleAll: () => void;
  onToggle: (target: DiscoveryTLSTarget) => void;
}) {
  const { t } = useTranslation();
  const allSelected = available.length > 0 && available.every((target) => selected.has(tlsTargetKey(target)));
  return (
    <section className="mt-3 rounded-sm border border-line-soft p-3" aria-label={t('discovery.certificates')}>
      <label className="flex items-center gap-2 text-sm font-medium text-ink">
        <input
          type="checkbox"
          checked={queueSelected}
          onChange={onToggleQueue}
          disabled={available.length === 0}
          className="h-4 w-4 accent-[var(--color-accent-primary)] disabled:opacity-40"
        />
        {t('discovery.monitorCertificates', { count: available.length })}
      </label>
      {available.length > 0 && (
        <div className="mt-2 border-t border-line-soft pt-2">
          <label className="flex items-center gap-2 text-2xs text-ink-muted">
            <input
              type="checkbox"
              checked={allSelected}
              onChange={onToggleAll}
              className="h-3.5 w-3.5 accent-[var(--color-accent-primary)]"
            />
            {t('discovery.selectAllCertificates')}
          </label>
          <ul className="mt-1 space-y-1">
            {available.map((target) => (
              <li key={tlsTargetKey(target)}>
                <label className="flex items-center gap-2 py-0.5 text-2xs text-ink-muted">
                  <input
                    type="checkbox"
                    aria-label={endpointLabel(target.address, target.port)}
                    checked={selected.has(tlsTargetKey(target))}
                    onChange={() => onToggle(target)}
                    className="h-3.5 w-3.5 accent-[var(--color-accent-primary)]"
                  />
                  <span className="font-mono">{endpointLabel(target.address, target.port)}</span>
                </label>
              </li>
            ))}
          </ul>
        </div>
      )}
      {targets.some((target) => target.monitoredBy) && (
        <ul className={`${available.length > 0 ? 'mt-2 border-t border-line-soft pt-2' : 'mt-2'} space-y-1`}>
          {targets.filter((target) => target.monitoredBy).map((target) => (
            <li key={tlsTargetKey(target)} className="flex items-center justify-between gap-3 text-2xs">
              <span className="flex items-center gap-2">
                <input type="checkbox" aria-label={endpointLabel(target.address, target.port)} checked disabled />
                <span className="font-mono text-ink-muted">{endpointLabel(target.address, target.port)}</span>
              </span>
              <span className="flex items-center gap-2">
                <ToneTag tone="ok" label={t('discovery.alreadyMonitored')} />
                <Link to={`/services/${target.monitoredBy}`} className="text-accent-primary hover:underline">
                  {t('discovery.viewConnector')}
                </Link>
              </span>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
