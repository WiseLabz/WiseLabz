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

export const categoryIcon = {
  virtualization: ServerIcon,
  containers_paas: BoxIcon,
  networking: NetworkIcon,
  dns: GlobeIcon,
  storage: DatabaseIcon,
  monitoring: ActivityIcon,
  media: FilmStripIcon,
  other: DotsThreeIcon,
} as const;
