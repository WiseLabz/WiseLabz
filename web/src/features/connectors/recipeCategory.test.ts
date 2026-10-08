import { describe, expect, it } from 'vitest';
import { readRecipeCategory } from './recipeCategory';

describe('readRecipeCategory', () => {
  it.each([
    ['block', 'version: 1\ncategory: media\n', 'media'],
    ['quoted with a comment', '# note\n"category": "dns" # shown\n', 'dns'],
    ['flow root', '{version: 1, category: storage}\n', 'storage'],
    ['BOM and CRLF', '﻿version: 1\r\ncategory: networking\r\n', 'networking'],
    ['nested key only', 'auth:\n  category: media\n', undefined],
    ['unknown category', 'category: spaceships\n', undefined],
    ['syntax error', 'category: [\n', undefined],
    ['not a mapping', '- category\n', undefined],
    ['empty', '', undefined],
  ])('reads %s text', (_name, recipe, expected) => {
    expect(readRecipeCategory(recipe)).toBe(expected);
  });
});
