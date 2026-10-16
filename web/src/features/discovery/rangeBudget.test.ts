import { describe, expect, it } from 'vitest';
import { calculateRangeBudget, countRangeHosts } from './rangeBudget';

describe('range budget', () => {
  it('counts usable hosts once across overlapping ranges', () => {
    expect(countRangeHosts(['192.168.1.0/24', '192.168.1.128/25'])).toBe(254);
    expect(countRangeHosts(['10.0.0.0/22'])).toBe(1022);
    expect(countRangeHosts(['10.0.0.0/31', '10.0.0.1/32'])).toBe(2);
  });

  it('fills the remaining budget from known hosts', () => {
    expect(calculateRangeBudget(['192.168.1.0/24', '192.168.2.0/24'], 900, true)).toEqual({
      rangeHosts: 508,
      inventoryHosts: 516,
      used: 1024,
      skippedInventory: 384,
    });
  });

  it('keeps ranges over budget visible without making the inventory count negative', () => {
    const ranges = ['192.168.1.0/24', '192.168.2.0/24', '192.168.3.0/24', '192.168.4.0/24', '192.168.5.0/24'];
    expect(calculateRangeBudget(ranges, 900, true)).toEqual({
      rangeHosts: 1270,
      inventoryHosts: 0,
      used: 1270,
      skippedInventory: 900,
    });
    expect(calculateRangeBudget(ranges, 900, false)).toEqual({
      rangeHosts: 1270,
      inventoryHosts: 0,
      used: 1270,
      skippedInventory: 0,
    });
  });

  it('does not count unsupported or invalid ranges', () => {
    expect(countRangeHosts(['8.8.8.0/24', 'fd00::/64', '10.0.0.0/21', 'bad'])).toBe(0);
  });
});
