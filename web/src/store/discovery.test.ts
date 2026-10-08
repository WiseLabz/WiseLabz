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
