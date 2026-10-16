import { beforeEach, describe, expect, it } from 'vitest';
import type { DiscoveryCandidate, DiscoveryScan } from '../api/model';
import { useDiscovery } from './discovery';

const pve: DiscoveryCandidate = {
  type: 'proxmox',
  name: 'Proxmox VE',
  address: '10.0.0.5',
  port: 8006,
  url: 'https://10.0.0.5:8006/api2/json',
  urlField: 'url',
};
const ha: DiscoveryCandidate = {
  type: 'home_assistant',
  name: 'Home Assistant',
  address: '10.0.0.7',
  port: 8123,
  url: 'http://10.0.0.7:8123',
  urlField: 'url',
};

const running = (over: Partial<DiscoveryScan> = {}): DiscoveryScan => ({
  id: 'scan-1',
  cidr: '10.0.0.0/24',
  state: 'running',
  startedAt: '2026-10-07T12:00:00Z',
  done: 0,
  total: 254,
  answered: 0,
  partial: false,
  candidates: [],
  ...over,
});

const scan = () => useDiscovery.getState().scan;

beforeEach(() => useDiscovery.setState({ scan: null }));

describe('discovery store', () => {
  it('preserves source counts, skipped hosts, query answers and TLS targets on hydration and events', () => {
    const s = useDiscovery.getState();
    const additions = {
      ranges: ['10.0.0.0/24', '100.100.0.0/24'],
      includeInventory: true,
      askNetwork: true,
      sources: { range: 508, inventory: 200, passive: 2 },
      skipped: { inventory: 4, passive: 3 },
      passiveAnswered: 5,
      tlsTargets: [{ address: 'fd00::5', port: 443, source: 'inventory' as const }],
    };
    s.setScan(running({ ...additions, total: 708 }));
    s.applyProgress({ scanId: 'scan-1', done: 100, total: 710, answered: 2 });
    const candidate = { ...pve, source: 'passive' as const, addOns: [{ type: 'dnsresolver', url: 'https://10.0.0.5:8006' }] };
    s.applyCandidate({ scanId: 'scan-1', candidate });
    s.applyComplete({ scanId: 'scan-1', state: 'completed', partial: false });
    expect(scan()).toMatchObject({ ...additions, total: 710, candidates: [candidate], state: 'completed' });
  });

  it('marks saved add-ons and only the certificate targets actually added', () => {
    const s = useDiscovery.getState();
    const addOn = { type: 'dnsresolver', url: 'https://10.0.0.1:443' };
    const targets = [
      { address: '10.0.0.1', port: 443, source: 'range' as const },
      { address: '10.0.0.1', port: 8006, source: 'range' as const },
      { address: 'fd00::5', port: 443, source: 'inventory' as const },
    ];
    const appliance = { ...pve, addOns: [addOn, { ...addOn, url: 'https://10.0.0.2:443' }] };
    s.setScan(running({ candidates: [appliance, ha], tlsTargets: targets }));
    s.markConnected([{ addOn, connectorId: 'dns-1' }, { tlsTargets: [targets[0], targets[2]], connectorId: 'probe-1' }]);
    expect(scan()?.candidates[0].addOns).toEqual([{ ...addOn, connectorId: 'dns-1' }, appliance.addOns[1]]);
    expect(scan()?.candidates[0].connectorId).toBeUndefined();
    expect(scan()?.candidates[1]).toEqual(ha);
    expect(scan()?.tlsTargets).toEqual([
      { ...targets[0], monitoredBy: 'probe-1' }, targets[1], { ...targets[2], monitoredBy: 'probe-1' },
    ]);
  });

  it('hydrates from the read endpoint, including a null for "no scan"', () => {
    useDiscovery.getState().setScan(running({ done: 40, candidates: [pve] }));
    expect(scan()).toMatchObject({ id: 'scan-1', done: 40, candidates: [pve] });
    useDiscovery.getState().setScan(null);
    expect(scan()).toBeNull();
  });

  it('applies progress, candidate and complete events of the current scan', () => {
    const s = useDiscovery.getState();
    s.setScan(running());
    s.applyProgress({ scanId: 'scan-1', done: 100, total: 254, answered: 2 });
    s.applyCandidate({ scanId: 'scan-1', candidate: pve });
    s.applyCandidate({ scanId: 'scan-1', candidate: ha });
    expect(scan()).toMatchObject({ done: 100, total: 254, answered: 2, state: 'running' });
    expect(scan()?.candidates).toEqual([pve, ha]);

    s.applyComplete({ scanId: 'scan-1', state: 'completed', partial: true });
    expect(scan()).toMatchObject({ state: 'completed', partial: true });
    expect(scan()?.endedAt).toBeTruthy();
    expect(scan()?.candidates).toEqual([pve, ha]);
  });

  it('ignores events from another scan id', () => {
    const s = useDiscovery.getState();
    s.setScan(running({ id: 'scan-2', candidates: [pve] }));
    s.applyProgress({ scanId: 'scan-1', done: 200, total: 254, answered: 9 });
    s.applyCandidate({ scanId: 'scan-1', candidate: ha });
    s.applyComplete({ scanId: 'scan-1', state: 'cancelled', partial: false });
    expect(scan()).toMatchObject({ id: 'scan-2', state: 'running', done: 0, answered: 0 });
    expect(scan()?.candidates).toEqual([pve]);
  });

  it('ignores events while no scan is held', () => {
    const s = useDiscovery.getState();
    s.applyProgress({ scanId: 'scan-1', done: 1, total: 254, answered: 0 });
    s.applyCandidate({ scanId: 'scan-1', candidate: pve });
    s.applyComplete({ scanId: 'scan-1', state: 'completed', partial: false });
    expect(scan()).toBeNull();
  });

  it('does not add a candidate twice, nor move progress backwards, nor add after the scan ended', () => {
    const s = useDiscovery.getState();
    s.setScan(running({ done: 120, answered: 3 }));
    s.applyCandidate({ scanId: 'scan-1', candidate: pve });
    s.applyCandidate({ scanId: 'scan-1', candidate: pve });
    s.applyProgress({ scanId: 'scan-1', done: 90, total: 254, answered: 1 });
    expect(scan()?.candidates).toEqual([pve]);
    expect(scan()).toMatchObject({ done: 120, answered: 3 });

    s.applyComplete({ scanId: 'scan-1', state: 'cancelled', partial: false });
    s.applyCandidate({ scanId: 'scan-1', candidate: ha });
    s.applyProgress({ scanId: 'scan-1', done: 200, total: 254, answered: 5 });
    expect(scan()?.candidates).toEqual([pve]);
    expect(scan()).toMatchObject({ state: 'cancelled', done: 120 });
  });

  it('ignores a late or duplicate complete once the scan is no longer running', () => {
    const s = useDiscovery.getState();
    s.setScan(running({ candidates: [pve] }));
    s.applyComplete({ scanId: 'scan-1', state: 'cancelled', partial: false });
    const before = scan();
    s.applyComplete({ scanId: 'scan-1', state: 'completed', partial: true });
    expect(scan()).toBe(before);
    expect(scan()).toMatchObject({ state: 'cancelled', partial: false });
  });

  it('marks candidates connected, matching type, address and port', () => {
    const s = useDiscovery.getState();
    s.setScan(running({ state: 'completed', candidates: [pve, ha] }));
    s.markConnected([{ candidate: pve, connectorId: 'c-1' }]);
    expect(scan()?.candidates).toEqual([{ ...pve, connectorId: 'c-1' }, ha]);
    // A later read replaces the scan wholesale, as the server's answer wins.
    s.setScan(running({ state: 'completed', candidates: [pve, ha] }));
    expect(scan()?.candidates[0].connectorId).toBeUndefined();
  });
});
