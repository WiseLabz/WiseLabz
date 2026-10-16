import { describe, expect, it } from 'vitest';
import { compareAddresses, endpointLabel, targetKey } from './candidates';

describe('candidate address ordering', () => {
  it('sorts IPv4 before IPv6 and compares addresses numerically', () => {
    expect(compareAddresses('10.0.0.2', '10.0.0.10')).toBeLessThan(0);
    expect(compareAddresses('255.255.255.255', '::1')).toBeLessThan(0);
    expect(compareAddresses('fd00::2', 'fd00::10')).toBeLessThan(0);
  });

  it('compares equivalent compressed and expanded IPv6 addresses equally', () => {
    expect(compareAddresses('fd00::1', 'fd00:0:0:0:0:0:0:1')).toBe(0);
  });

  it('brackets IPv6 endpoints while keeping raw address and port keys stable', () => {
    expect(endpointLabel('fd00::1', 443)).toBe('[fd00::1]:443');
    expect(endpointLabel('10.0.0.1', 443)).toBe('10.0.0.1:443');
    expect(targetKey('fd00::1', 443)).toBe('fd00::1|443');
  });
});
