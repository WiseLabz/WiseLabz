import type { DiscoveryCandidate } from '../../api/model';

/** Stable identity of a candidate within a scan: one product on one address and port. */
export const candidateKey = (c: DiscoveryCandidate) => `${c.type}|${c.address}|${c.port}`;

/** Orders candidates by IPv4 address, then port, then type. */
export const byAddress = (a: DiscoveryCandidate, b: DiscoveryCandidate) => {
  const octets = (ip: string) => ip.split('.').map(Number);
  const [x, y] = [octets(a.address), octets(b.address)];
  for (let i = 0; i < 4; i++) if (x[i] !== y[i]) return x[i] - y[i];
  return a.port - b.port || a.type.localeCompare(b.type);
};
