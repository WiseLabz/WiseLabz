import { describe, expect, it } from 'vitest';
import { ConnectorCategory } from '../api/model';
import { categoryIcon, categoryIconFor } from './categoryIcon';

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

  it('returns each category\'s own icon from categoryIconFor', () => {
    for (const cat of Object.values(ConnectorCategory)) {
      expect(categoryIconFor(cat)).toBe(categoryIcon[cat]);
    }
  });

  it('falls back to the other icon for an unknown category', () => {
    expect(categoryIconFor('gaming')).toBe(categoryIcon.other);
    expect(categoryIconFor('toString')).toBe(categoryIcon.other);
  });
});
