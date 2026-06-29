/**
 * Deterministic, hand-authored template fixtures for the Templates CRUD surface
 * (Phase 7). NOT faker — believable homelab doc templates with real placeholder
 * grammar (`{{node.version}}`), so the editor, preview-generate, and diff all
 * render against stable data. The mock handlers in `mocks/templates.mock.ts`
 * mutate this array in place so create/edit/delete persist for the session.
 *
 * A template is a set of ordered sections whose bodies carry `{{placeholder}}`
 * tokens. Preview-generate resolves those tokens against a connector's last
 * synced snapshot (mocked here per connector type) to produce a DocVersion.
 */
import type { DocVersion, Template } from '../api/model';

/* ── Templates ─────────────────────────────────────────────────────────── */

export const templates: Template[] = [
  {
    id: 'tpl-proxmox-node',
    name: 'Proxmox Node',
    description:
      'Standard documentation for a Proxmox VE hypervisor: node specs and the virtual machine roster.',
    appliesTo: { category: 'virtualization', type: 'proxmox' },
    sections: [
      {
        title: 'Overview',
        order: 1,
        body: '**{{service.name}}** is a {{service.type}} node reachable at `{{service.url}}`.\nLast synced {{service.lastSync}}.',
      },
      {
        title: 'Node',
        order: 2,
        body: '| Field | Value |\n| --- | --- |\n| Version | {{node.version}} |\n| CPU | {{node.cpu}} |\n| Memory | {{node.memory}} |\n| Storage | {{node.storage}} |',
      },
      {
        title: 'Virtual machines',
        order: 3,
        body: 'A total of {{vm.count}} virtual machines are defined on this node.\n\n{{vm.table}}',
      },
    ],
  },
  {
    id: 'tpl-network-appliance',
    name: 'Network Appliance',
    description:
      'Firewall / router documentation: interface map and the active rule set, for any networking connector.',
    appliesTo: { category: 'networking' },
    sections: [
      {
        title: 'Overview',
        order: 1,
        body: '**{{service.name}}** ({{service.type}}) guards the lab perimeter at `{{service.url}}`.',
      },
      {
        title: 'Interfaces',
        order: 2,
        body: '{{iface.table}}',
      },
      {
        title: 'Firewall rules',
        order: 3,
        body: '{{fw.count}} rules are currently active.\n\n{{fw.table}}',
      },
    ],
  },
  {
    id: 'tpl-container-host',
    name: 'Container Host',
    description:
      'Container platform documentation: deployed stacks and the running container inventory.',
    appliesTo: { category: 'containers_paas', type: 'portainer' },
    sections: [
      {
        title: 'Overview',
        order: 1,
        body: '**{{service.name}}** runs {{service.type}} at `{{service.url}}` and orchestrates the lab workloads.',
      },
      {
        title: 'Stacks',
        order: 2,
        body: '{{stack.count}} stacks are deployed.\n\n{{stack.table}}',
      },
    ],
  },
];

/* ── Mock synced snapshots, keyed by connector type ────────────────────── */

type Snapshot = Record<string, string>;

/** Sample placeholder data per connector type. Deterministic, hand-authored. */
const SNAPSHOTS: Record<string, Snapshot> = {
  proxmox: {
    'node.version': '8.2.4',
    'node.cpu': 'AMD Ryzen 9 5950X (16c / 32t)',
    'node.memory': '128 GB ECC',
    'node.storage': 'local-zfs (4 TB NVMe mirror)',
    'vm.count': '3',
    'vm.table':
      '| VMID | Name | Cores | Memory | State |\n| --- | --- | --- | --- | --- |\n| 101 | k8s-control-01 | 4 | 8 GB | running |\n| 102 | k8s-worker-01 | 4 | 8 GB | running |\n| 103 | k8s-worker-02 | 4 | 8 GB | running |',
  },
  opnsense: {
    'iface.table':
      '| Interface | Role | Address |\n| --- | --- | --- |\n| igc0 | WAN | dhcp |\n| igc1 | LAN | 10.0.0.1/24 |\n| igc2 | DMZ | 10.0.10.1/24 |',
    'fw.count': '14',
    'fw.table':
      '| # | Action | Source | Destination | Port |\n| --- | --- | --- | --- | --- |\n| 1 | pass | LAN net | any | any |\n| 2 | block | WAN net | LAN net | any |\n| 3 | pass | DMZ net | WAN net | 443 |',
  },
  portainer: {
    'stack.count': '4',
    'stack.table':
      '| Stack | Services | Status |\n| --- | --- | --- |\n| monitoring | 3 | running |\n| media | 5 | running |\n| reverse-proxy | 1 | running |\n| backups | 2 | running |',
  },
};

/** Lightweight connector shape used by the preview generator. */
export interface PreviewConnector {
  id: string;
  name: string;
  type: string;
  url: string;
}

/** Deterministic stand-in connectors so preview works without the connectors API. */
export const previewConnectors: PreviewConnector[] = [
  { id: 'svc-pve1', name: 'pve1', type: 'proxmox', url: 'https://10.0.0.11:8006' },
  { id: 'svc-opnsense', name: 'opnsense', type: 'opnsense', url: 'https://10.0.0.1' },
  { id: 'svc-portainer', name: 'portainer', type: 'portainer', url: 'https://10.0.0.20:9443' },
];

/* ── Preview generator ─────────────────────────────────────────────────── */

/** Resolve a single `{{token}}` against the active snapshot + connector facts. */
function resolveToken(token: string, snap: Snapshot, conn: PreviewConnector | null): string {
  switch (token) {
    case 'service.name':
      return conn?.name ?? '(sample-service)';
    case 'service.type':
      return conn?.type ?? '(type)';
    case 'service.url':
      return conn?.url ?? '(url)';
    case 'service.lastSync':
      return '4 minutes ago';
    default:
      return snap[token] ?? `_(no data for ${token})_`;
  }
}

/** Fill every `{{token}}` in a body string. */
function fillBody(body: string, snap: Snapshot, conn: PreviewConnector | null): string {
  return body.replace(/\{\{\s*([\w.]+)\s*}}/g, (_m, token: string) =>
    resolveToken(token, snap, conn)
  );
}

/**
 * Render a template's sections into a single Markdown document. Used both for
 * the resolved preview (`fill = true`) and the raw template source shown on the
 * left side of the preview diff (`fill = false`).
 */
export function renderTemplate(
  template: Template,
  conn: PreviewConnector | null,
  fill: boolean
): string {
  const snap = conn ? (SNAPSHOTS[conn.type] ?? {}) : {};
  const heading = conn ? conn.name : template.name;
  const ordered = [...template.sections].sort((a, b) => a.order - b.order);
  const parts = [`# ${heading}`];
  for (const s of ordered) {
    parts.push(`## ${s.title}`);
    parts.push(fill ? fillBody(s.body, snap, conn) : s.body);
  }
  return parts.join('\n\n') + '\n';
}

/** Build the DocVersion the preview endpoint returns for a template + connector. */
export function generatePreview(template: Template, connectorId?: string): DocVersion {
  const conn = connectorId ? (previewConnectors.find((c) => c.id === connectorId) ?? null) : null;
  return {
    rev: 0,
    createdAt: new Date().toISOString(),
    author: null,
    trigger: 'template',
    content: renderTemplate(template, conn, true),
  };
}
