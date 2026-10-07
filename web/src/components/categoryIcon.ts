/** Connector category → icon. Kept out of icons.tsx so that file stays
 *  component-only (react-refresh / fast-refresh friendliness). */
import {
  ServerIcon,
  BoxIcon,
  NetworkIcon,
  GlobeIcon,
  DatabaseIcon,
  ActivityIcon,
  FilmStripIcon,
  DotsThreeIcon,
} from './icons';
import type { ConnectorCategory } from '../api/model';

export const categoryIcon = {
  virtualization: ServerIcon,
  containers_paas: BoxIcon,
  networking: NetworkIcon,
  dns: GlobeIcon,
  storage: DatabaseIcon,
  monitoring: ActivityIcon,
  media: FilmStripIcon,
  other: DotsThreeIcon,
} as const satisfies Record<ConnectorCategory, unknown>;

/** Icon for a category coming from the API. A category this bundle does not
 *  know (stale cached bundle, newer server) falls back to the `other` icon
 *  instead of rendering an undefined component. */
export function categoryIconFor(category: string) {
  return Object.prototype.hasOwnProperty.call(categoryIcon, category)
    ? categoryIcon[category as ConnectorCategory]
    : categoryIcon.other;
}
