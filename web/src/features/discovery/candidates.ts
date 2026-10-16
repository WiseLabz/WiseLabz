import { DiscoverySource, type DiscoveryCandidate, type DiscoverySource as DiscoverySourceType } from '../../api/model';

/** Stable identity of a candidate within a scan: one product on one address and port. */
export const candidateKey = (c: DiscoveryCandidate) => `${c.type}|${c.address}|${c.port}`;

export const addOnKey = (c: DiscoveryCandidate, addOn: { type: string; url: string }) =>
  `${candidateKey(c)}|${addOn.type}|${addOn.url}`;

/** Formats an IP and port for display or a TLS Probe target. IPv6 literals need brackets. */
export const endpointLabel = (address: string, port: number) =>
  `${address.includes(':') ? `[${address}]` : address}:${port}`;

export const targetKey = (address: string, port: number) => `${address}|${port}`;

/** Orders candidates by numeric IP address (IPv4 first), then port, then type. */
export const byAddress = (a: DiscoveryCandidate, b: DiscoveryCandidate) => {
  return compareAddresses(a.address, b.address) || a.port - b.port || a.type.localeCompare(b.type);
};

export const candidateSource = (candidate: DiscoveryCandidate): DiscoverySourceType =>
  candidate.source ?? DiscoverySource.range;

export function compareAddresses(a: string, b: string): number {
  const left4 = parseIPv4(a);
  const right4 = parseIPv4(b);
  if (left4 !== undefined || right4 !== undefined) {
    if (left4 === undefined) return 1;
    if (right4 === undefined) return -1;
    return left4 - right4;
  }

  const left6 = parseIPv6(a);
  const right6 = parseIPv6(b);
  if (left6 !== undefined && right6 !== undefined) return left6 < right6 ? -1 : left6 > right6 ? 1 : 0;
  return a.localeCompare(b);
}

function parseIPv4(address: string): number | undefined {
  const octets = address.split('.');
  if (octets.length !== 4 || octets.some((part) => !/^\d{1,3}$/.test(part))) return undefined;
  const values = octets.map(Number);
  if (values.some((value) => value > 255)) return undefined;
  return values.reduce((result, value) => result * 256 + value, 0);
}

function parseIPv6(address: string): bigint | undefined {
  let value = address.split('%', 1)[0];
  if (!value.includes(':')) return undefined;
  if (value.includes('.')) {
    const separator = value.lastIndexOf(':');
    const ipv4 = parseIPv4(value.slice(separator + 1));
    if (ipv4 === undefined) return undefined;
    value = `${value.slice(0, separator)}:${(ipv4 >>> 16).toString(16)}:${(ipv4 & 0xffff).toString(16)}`;
  }

  const halves = value.split('::');
  if (halves.length > 2) return undefined;
  const left = halves[0] ? halves[0].split(':') : [];
  const right = halves.length === 2 && halves[1] ? halves[1].split(':') : [];
  const zeros = 8 - left.length - right.length;
  if ((halves.length === 1 && zeros !== 0) || (halves.length === 2 && zeros < 1)) return undefined;
  const words = [...left, ...Array(zeros).fill('0'), ...right];
  if (words.some((word) => !/^[\da-f]{1,4}$/i.test(word))) return undefined;
  return words.reduce((result, word) => (result << 16n) | BigInt(`0x${word}`), 0n);
}
