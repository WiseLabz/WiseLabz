/** Nested scope/doc navigation; native drag/drop moves docs within one scope. */
import { NavLink } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { cn } from '../../lib/cn';
import { FileTextIcon, LayersIcon, ShareIcon } from '../icons';
import { useConnectorRole, useIsInstanceAdmin } from '../../hooks/useRole';
import { IconButton } from '../ui/Button';
import type { DocNode } from '../../api/model';

interface DocTreeProps {
  tree: DocNode;
  onShare?: (node: { docId: string; title: string; connectorId?: string }) => void;
  onReparent?: (docId: string, parentId: string) => void;
}

export function DocTree(props: DocTreeProps) {
  return (
    <nav aria-label="Documentation">
      <DocTreeItem {...props} root />
    </nav>
  );
}

function DocTreeItem({
  tree: node,
  onShare,
  onReparent,
  root = false,
  scope = '',
}: DocTreeProps & { root?: boolean; scope?: string }) {
  const { t } = useTranslation();
  // Older tree fixtures omit branch metadata; connector groups have no origin.
  const branch = node.branch ?? (root || (node.kind === 'service' && !node.origin));
  const serviceId = node.serviceId ?? (branch && node.kind === 'service' ? node.docId : scope);
  const admin = useIsInstanceAdmin();
  const role = useConnectorRole(serviceId || undefined);
  const canEdit = serviceId ? role === 'operator' : admin;
  const canShare = canEdit && node.docId !== 'lab';
  const draggable = !branch && canEdit && !!onReparent;
  const targetParent = branch ? '' : node.docId;
  return (
    <div>
      <div
        className="group flex items-center gap-1"
        draggable={draggable}
        onDragStart={(e) => {
          e.stopPropagation();
          e.dataTransfer.setData(
            'application/x-wiselabz-doc',
            JSON.stringify({ id: node.docId, scope: serviceId })
          );
          e.dataTransfer.effectAllowed = 'move';
        }}
        onDragOver={(e) => {
          if (canEdit && !root && onReparent) {
            e.preventDefault();
            e.stopPropagation();
            e.dataTransfer.dropEffect = 'move';
          }
        }}
        onDrop={(e) => {
          e.preventDefault();
          e.stopPropagation();
          if (!canEdit || root || !onReparent) return;
          try {
            const source = JSON.parse(e.dataTransfer.getData('application/x-wiselabz-doc')) as {
              id: string;
              scope: string;
            };
            if (
              typeof source.id === 'string' &&
              source.scope === serviceId &&
              source.id !== node.docId
            )
              onReparent(source.id, targetParent);
          } catch {
            /* External drags have no doc payload. */
          }
        }}
      >
        <NavLink
          to={`/docs/${node.docId}`}
          end
          className={({ isActive }) =>
            cn(
              'flex min-w-0 flex-1 items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm transition-colors',
              isActive
                ? 'bg-accent-primary-tint text-ink'
                : 'text-ink-muted hover:bg-surface-raised hover:text-ink'
            )
          }
        >
          {root ? (
            <LayersIcon size={16} />
          ) : (
            <FileTextIcon size={15} className="shrink-0 opacity-70" />
          )}
          <span className="truncate">{node.title.split(' — ')[0]}</span>
          {node.title.includes(' — ') && (
            <span className="ml-auto truncate font-mono text-2xs text-ink-faint">
              {node.title.split(' — ')[1]}
            </span>
          )}
        </NavLink>
        {onShare && canShare && (
          <IconButton
            label={t('docs.share.action', { defaultValue: 'Share' })}
            onClick={() =>
              onShare({
                docId: node.docId,
                title: node.title,
                ...(serviceId ? { connectorId: serviceId } : {}),
              })
            }
            className="shrink-0 opacity-0 group-hover:opacity-100 focus:opacity-100"
          >
            <ShareIcon size={14} />
          </IconButton>
        )}
      </div>
      {!!node.children?.length && (
        <div className="ml-3 flex flex-col gap-0.5 border-l border-line-soft pl-2">
          {node.children.map((child) => (
            <DocTreeItem
              key={child.docId}
              tree={child}
              onShare={onShare}
              onReparent={onReparent}
              scope={serviceId}
            />
          ))}
        </div>
      )}
    </div>
  );
}
