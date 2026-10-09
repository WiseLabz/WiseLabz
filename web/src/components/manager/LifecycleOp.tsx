/**
 * Shared dry-run-preview + elevation-confirm dialogs for a connector
 * lifecycle mutation (restart/start/stop) — driven by {@link useMutatingOp}
 * (`useMutatingOp.ts`), reused by ServiceDetailPage and RunbookPanel (#282).
 */
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import type { ActionRequest } from '../../api/model';
import { Button } from '../ui/Button';
import { Dialog } from '../ui/Dialog';
import { SkeletonRows } from '../ui/states';
import { ElevationConfirm } from './ElevationConfirm';
import type { MutatingOp } from './useMutatingOp';

/** Every string the dialogs render, pre-translated by the caller (each
 * consumer has its own i18n keys — ServiceDetailPage's verb-specific copy vs.
 * RunbookPanel's step copy). */
export interface MutatingOpMessages {
  previewTitle: string;
  previewNotice: string;
  previewError: string;
  targetLabel: string;
  downtimeLabel: string;
  downtimeSeconds: (count: number) => string;
  downtimeIndefinite: string;
  dependenciesLabel: string;
  noDependencies: string;
  failed: string;
  retry: string;
  confirmTitle: string;
  confirmDescription: string;
  confirmLabel: string;
}

export interface MutatingOpExtraMessages {
  userDefined: string;
  actionLabel: string;
  actionDescription: string;
  request: string;
  method: string;
  url: string;
  headers: string;
  body: string;
  entityRequired: string;
  resultTitle: string;
  status: string;
  excerpt: string;
  resultClose: string;
  noDowntime: string;
}

export function MutatingOpDialogs({
  op,
  action,
  resourceName,
  messages,
  entityPicker,
  extraMessages,
  actionMetadata,
  target,
  requireEntity = false,
  selectedEntityRef = '',
}: {
  op: MutatingOp;
  /** Elevation action string, e.g. `connector.restart`. */
  action: string;
  resourceName: string;
  messages: MutatingOpMessages;
  extraMessages?: MutatingOpExtraMessages;
  actionMetadata?: { label?: string; description?: string };
  target?: string;
  requireEntity?: boolean;
  selectedEntityRef?: string;
  /** Optional entity picker rendered above the preview, for connectors that
   * manage more than one entity (VMs, containers, …). */
  entityPicker?: ReactNode;
}) {
  const { t } = useTranslation();
  return (
    <>
      <Dialog
        open={op.previewOpen}
        onClose={() => op.setPreviewOpen(false)}
        title={messages.previewTitle}
        size="sm"
      >
        <div className="space-y-4">
          {entityPicker}
          {op.preview.isPending ? (
            <SkeletonRows rows={3} />
          ) : requireEntity && !selectedEntityRef && !op.preview.data ? (
            <p className="text-sm text-ink-muted">{extraMessages?.entityRequired}</p>
          ) : op.preview.isError || !op.preview.data ? (
            <div className="space-y-3">
              <p className="text-sm text-err">{messages.previewError}</p>
              <Button size="sm" variant="secondary" onClick={() => op.rerunPreview()}>
                {messages.retry}
              </Button>
            </div>
          ) : (
            <div className="space-y-4">
              <p className="text-sm text-ink-muted">{messages.previewNotice}</p>
              {(op.preview.data.userDefined || op.preview.data.label || op.preview.data.description) && (
                <div className="space-y-1 rounded-md border border-line-soft bg-canvas-sunken p-3">
                  {op.preview.data.userDefined && (
                    <p className="text-2xs font-semibold uppercase tracking-wide text-ink-faint">
                      {extraMessages?.userDefined}
                    </p>
                  )}
                  {(op.preview.data.label || actionMetadata?.label) && (
                    <p className="text-sm text-ink">
                      <span className="text-2xs text-ink-faint">{extraMessages?.actionLabel}: </span>
                      <span className="font-semibold">
                        {op.preview.data.label || actionMetadata?.label}
                      </span>
                    </p>
                  )}
                  {(op.preview.data.description || actionMetadata?.description) && (
                    <p className="text-sm text-ink-muted">
                      <span className="text-2xs text-ink-faint">{extraMessages?.actionDescription}: </span>
                      {op.preview.data.description || actionMetadata?.description}
                    </p>
                  )}
                </div>
              )}
              {op.preview.data.request && extraMessages && (
                <ActionRequestBlock request={op.preview.data.request} messages={extraMessages} />
              )}
              <dl className="grid grid-cols-2 gap-x-4 gap-y-3 rounded-md border border-line-soft bg-canvas-sunken p-3 text-sm">
                <div>
                  <dt className="text-2xs text-ink-faint">{messages.targetLabel}</dt>
                  <dd className="mt-0.5 font-medium text-ink">{op.preview.data.targetService}</dd>
                </div>
                <div>
                  <dt className="text-2xs text-ink-faint">{messages.downtimeLabel}</dt>
                  <dd className="mt-0.5 font-medium text-ink">
                    {op.preview.data.estimatedDowntimeSeconds > 0
                      ? messages.downtimeSeconds(op.preview.data.estimatedDowntimeSeconds)
                      : op.preview.data.userDefined
                        ? extraMessages?.noDowntime
                        : messages.downtimeIndefinite}
                  </dd>
                </div>
              </dl>
              {(op.preview.data.affectedEntities?.length ?? 0) > 0 && (
                <div>
                  <h3 className="text-sm font-semibold text-ink">{t('services.detail.opAffected')}</h3>
                  <ul className="mt-2 divide-y divide-line-soft rounded-md border border-warn/40">
                    {op.preview.data.affectedEntities?.map((name) => (
                      <li key={name} className="px-3 py-2 text-sm text-ink">
                        {name}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
              <div>
                <h3 className="text-sm font-semibold text-ink">{messages.dependenciesLabel}</h3>
                {op.preview.data.dependentServices.length === 0 ? (
                  <p className="mt-1 text-sm text-ink-muted">{messages.noDependencies}</p>
                ) : (
                  <ul className="mt-2 divide-y divide-line-soft rounded-md border border-line-soft">
                    {op.preview.data.dependentServices.map((service) => (
                      <li key={`${service.kind}-${service.name}`} className="px-3 py-2 text-sm text-ink">
                        <span>{service.name}</span>
                        <span className="ml-2 font-mono text-2xs text-ink-faint">{service.kind}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
              {op.mutate.isError && (
                <ActionFailure
                  error={{ response: { data: op.failureData } }}
                  failed={messages.failed}
                  messages={extraMessages}
                />
              )}
              <div className="flex justify-end">
                <Button size="sm" variant="danger" onClick={() => op.setConfirmOpen(true)}>
                  {messages.confirmLabel}
                </Button>
              </div>
            </div>
          )}
        </div>
      </Dialog>

      <ElevationConfirm
        open={op.confirmOpen}
        resourceName={resourceName}
        action={action}
        target={target}
        title={messages.confirmTitle}
        description={messages.confirmDescription}
        confirmLabel={messages.confirmLabel}
        isPending={op.mutate.isPending}
        onClose={() => op.setConfirmOpen(false)}
        onConfirm={(token) => op.mutate.mutate(token)}
      />
      {extraMessages && <ActionResultDialog op={op} messages={extraMessages} />}
    </>
  );
}

/** The labels {@link ActionRequestBlock} renders, pre-translated by the caller. */
export type ActionRequestMessages = Pick<
  MutatingOpExtraMessages,
  'request' | 'method' | 'url' | 'headers' | 'body'
>;

/** Read-only view of the HTTP request a named action or lifecycle verb will send. */
export function ActionRequestBlock({
  request,
  messages,
}: {
  request: ActionRequest;
  messages: ActionRequestMessages;
}) {
  return (
    <section className="space-y-2 rounded-md border border-line-soft bg-canvas-sunken p-3 text-sm">
      <h3 className="font-semibold text-ink">{messages.request}</h3>
      <dl className="space-y-2">
        <div>
          <dt className="text-2xs text-ink-faint">{messages.method}</dt>
          <dd className="mt-0.5 font-mono text-xs text-ink">{request.method}</dd>
        </div>
        <div>
          <dt className="text-2xs text-ink-faint">{messages.url}</dt>
          <dd className="mt-0.5 break-all font-mono text-xs text-ink">{request.url}</dd>
        </div>
        {request.headers && Object.keys(request.headers).length > 0 && (
          <div>
            <dt className="text-2xs text-ink-faint">{messages.headers}</dt>
            <dd>
              <pre className="mt-0.5 overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs text-ink">
                {JSON.stringify(request.headers, null, 2)}
              </pre>
            </dd>
          </div>
        )}
        {request.body !== undefined && (
          <div>
            <dt className="text-2xs text-ink-faint">{messages.body}</dt>
            <dd>
              <pre className="mt-0.5 overflow-x-auto whitespace-pre-wrap break-all font-mono text-xs text-ink">
                {JSON.stringify(request.body, null, 2)}
              </pre>
            </dd>
          </div>
        )}
      </dl>
    </section>
  );
}

function ActionFailure({
  error,
  failed,
  messages,
}: {
  error: unknown;
  failed: string;
  messages?: MutatingOpExtraMessages;
}) {
  const details = responseDetails(errorFromResponse(error));
  return (
    <div role="alert" className="space-y-1 text-2xs text-err">
      <p>{failed}</p>
      {details.status != null && messages && (
        <p>
          {messages.status}: {details.status}
        </p>
      )}
      {details.excerpt && messages && (
        <p>
          {messages.excerpt}: <span className="whitespace-pre-wrap">{details.excerpt}</span>
        </p>
      )}
    </div>
  );
}

function ActionResultDialog({
  op,
  messages,
}: {
  op: MutatingOp;
  messages: MutatingOpExtraMessages;
}) {
  const details = responseDetails(op.resultData);
  if (details.status == null && !details.excerpt) return null;
  return (
    <Dialog
      open
      onClose={op.clearResult}
      title={messages.resultTitle}
      size="sm"
    >
      <div className="space-y-3">
        {details.status != null && (
          <p className="text-sm text-ink">
            {messages.status}: <span className="font-mono">{details.status}</span>
          </p>
        )}
        {details.excerpt && (
          <div>
            <p className="mb-1 text-2xs text-ink-faint">{messages.excerpt}</p>
            <pre className="whitespace-pre-wrap break-words rounded-md border border-line-soft bg-canvas-sunken p-3 font-mono text-xs text-ink">
              {details.excerpt}
            </pre>
          </div>
        )}
        <div className="flex justify-end">
          <Button size="sm" variant="secondary" onClick={op.clearResult}>
            {messages.resultClose}
          </Button>
        </div>
      </div>
    </Dialog>
  );
}

function errorFromResponse(error: unknown): unknown {
  if (!error || typeof error !== 'object') return undefined;
  const response = (error as { response?: { data?: unknown } }).response;
  return response?.data;
}

function responseDetails(value: unknown): { status?: number; excerpt?: string } {
  if (!value || typeof value !== 'object') return {};
  const result = value as { status?: unknown; statusCode?: unknown; excerpt?: unknown };
  const status = typeof result.statusCode === 'number'
    ? result.statusCode
    : typeof result.status === 'number'
      ? result.status
      : undefined;
  return {
    status,
    excerpt: typeof result.excerpt === 'string' ? result.excerpt : undefined,
  };
}
