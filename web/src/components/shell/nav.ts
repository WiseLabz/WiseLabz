/** Single source of truth for primary navigation, shared by every shell variant. */
import {
  GaugeIcon,
  LayersIcon,
  FileTextIcon,
  DiffIcon,
  BellIcon,
  AlertTriangleIcon,
} from '../icons';

export interface NavItem {
  to: string;
  label: string;
  Icon: React.ComponentType<{ size?: number; className?: string }>;
  /** show the live pending-alerts badge on this item */
  badge?: boolean;
  badgeSource?: 'alerts' | 'findings';
}

export const NAV: NavItem[] = [
  { to: '/dashboard', label: 'Dashboard', Icon: GaugeIcon },
  { to: '/services', label: 'Services', Icon: LayersIcon },
  { to: '/docs', label: 'Docs', Icon: FileTextIcon },
  { to: '/changes', label: 'Changes', Icon: DiffIcon },
  { to: '/alerts', label: 'Alerts', Icon: BellIcon, badge: true },
  { to: '/findings', label: 'Findings', Icon: AlertTriangleIcon, badge: true, badgeSource: 'findings' },
];
