import { describe, expect, it } from 'vitest';
import { ConnectorCategory } from '../api/model';
import { categoryIcon } from './categoryIcon';

describe('categoryIcon (#513)', () => {
  it('defines an icon component for every connector category', () => {
    const categories = Object.values(ConnectorCategory);
    expect(categories).toHaveLength(8);

    for (const cat of categories) {
      const Icon = categoryIcon[cat];
      expect(Icon, `missing icon for category ${cat}`).toBeDefined();
      expect(typeof Icon).toBe('function');
    }
  });
});
