const MAX_ADDRESSES = 1024;

const PRIVATE_BLOCKS: ReadonlyArray<readonly [number, number]> = [
  [0x0a000000, 8],
  [0xac100000, 12],
  [0xc0a80000, 16],
  [0x64400000, 10],
];

function parseIPv4(value: string): number | undefined {
  const octets = value.split('.');
  if (octets.length !== 4 || octets.some((part) => !/^\d{1,3}$/.test(part))) return undefined;
  const numbers = octets.map(Number);
  if (numbers.some((part) => part > 255)) return undefined;
  return numbers.reduce((result, part) => (result * 256 + part) >>> 0, 0);
}

/** Counts unique usable hosts in valid supported IPv4 ranges. Invalid chips await server validation. */
export function countRangeHosts(ranges: readonly string[]): number {
  const hosts = new Set<number>();
  for (const range of ranges) {
    const [address, prefixText, extra] = range.split('/');
    if (prefixText === undefined || extra !== undefined || !/^\d{1,2}$/.test(prefixText)) continue;
    const prefix = Number(prefixText);
    const ip = parseIPv4(address);
    if (ip === undefined || prefix < 22 || prefix > 32) continue;

    const mask = prefix === 0 ? 0 : (0xffffffff << (32 - prefix)) >>> 0;
    const network = (ip & mask) >>> 0;
    const end = (network | (~mask >>> 0)) >>> 0;
    const contained = PRIVATE_BLOCKS.some(([block, bits]) => {
      const blockMask = (0xffffffff << (32 - bits)) >>> 0;
      return (network & blockMask) >>> 0 === block && (end & blockMask) >>> 0 === block;
    });
    if (!contained) continue;

    const first = prefix <= 30 ? network + 1 : network;
    const last = prefix <= 30 ? end - 1 : end;
    for (let host = first; host <= last; host++) hosts.add(host >>> 0);
  }
  return hosts.size;
}

export function calculateRangeBudget(ranges: readonly string[], knownHosts: number, includeInventory: boolean) {
  const rangeHosts = countRangeHosts(ranges);
  const inventoryHosts = includeInventory
    ? Math.min(Math.max(0, knownHosts), Math.max(0, MAX_ADDRESSES - rangeHosts))
    : 0;
  return {
    rangeHosts,
    inventoryHosts,
    used: rangeHosts + inventoryHosts,
    skippedInventory: includeInventory ? Math.max(0, knownHosts - inventoryHosts) : 0,
  };
}
