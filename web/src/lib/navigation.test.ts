import { afterEach, describe, expect, it, vi } from 'vitest';
import { setAppNavigator, navigateTo } from './navigation';

describe('navigation', () => {
  afterEach(() => {
    setAppNavigator(null);
  });

  describe('setAppNavigator', () => {
    it('sets the navigator function', () => {
      const mock = vi.fn();
      setAppNavigator(mock);
      navigateTo('/test');
      expect(mock).toHaveBeenCalledWith('/test');
    });

    it('replaces previous navigator', () => {
      const first = vi.fn();
      const second = vi.fn();
      setAppNavigator(first);
      setAppNavigator(second);
      navigateTo('/path');
      expect(first).not.toHaveBeenCalled();
      expect(second).toHaveBeenCalledWith('/path');
    });

    it('clears navigator when set to null', () => {
      const mock = vi.fn();
      setAppNavigator(mock);
      setAppNavigator(null);
      navigateTo('/path');
      expect(mock).not.toHaveBeenCalled();
    });
  });

  describe('navigateTo', () => {
    it('calls the registered navigator with destination', () => {
      const mock = vi.fn();
      setAppNavigator(mock);
      navigateTo('/documents/123');
      expect(mock).toHaveBeenCalledWith('/documents/123');
    });

    it('is a no-op before navigator is registered', () => {
      expect(() => navigateTo('/path')).not.toThrow();
    });

    it('navigates to different paths', () => {
      const mock = vi.fn();
      setAppNavigator(mock);
      navigateTo('/path1');
      navigateTo('/path2');
      expect(mock).toHaveBeenCalledTimes(2);
      expect(mock).toHaveBeenNthCalledWith(1, '/path1');
      expect(mock).toHaveBeenNthCalledWith(2, '/path2');
    });
  });
});
