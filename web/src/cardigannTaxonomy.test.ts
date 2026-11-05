import { describe, expect, it } from 'vitest';
import type { CardigannDefinition } from './types';
import {
  definitionContentTypes,
  definitionSearchModes,
  filterDefinitions,
  matchesDefinitionFilters,
} from './cardigannTaxonomy';

const sample = (overrides: Partial<CardigannDefinition> = {}): CardigannDefinition => ({
  id: 'sample',
  name: 'Sample Tracker',
  description: 'Private movie tracker',
  language: 'en-US',
  type: 'private',
  links: ['https://example.com'],
  caps: {
    categorymappings: [
      { id: 1, cat: 'Movies/HD', desc: 'HD' },
      { id: 2, cat: 'TV/HD', desc: 'TV' },
      { id: 3, cat: 'XXX', desc: 'Adult' },
    ],
    modes: {
      search: ['q'],
      'movie-search': ['q', 'imdbid'],
      'tv-search': ['q', 'tvdbid'],
    },
  },
  ...overrides,
});

describe('cardigannTaxonomy', () => {
  it('derives parent content types from caps.categorymappings', () => {
    expect(definitionContentTypes(sample())).toEqual(['Movies', 'TV', 'XXX']);
  });

  it('reads search modes from caps.modes', () => {
    expect(definitionSearchModes(sample())).toEqual(['movie-search', 'search', 'tv-search']);
  });

  it('filters by privacy, language, content type, and search mode', () => {
    const defs = [
      sample(),
      sample({
        id: 'public-books',
        name: 'Public Books',
        type: 'public',
        language: 'zh-CN',
        caps: {
          categorymappings: [{ id: 1, cat: 'Books/EBook', desc: 'Ebook' }],
          modes: { search: ['q'], 'book-search': ['q'] },
        },
      }),
    ];

    expect(
      filterDefinitions(defs, {
        query: '',
        type: 'public',
        language: '',
        contentType: '',
        searchMode: '',
      }).map((d) => d.id)
    ).toEqual(['public-books']);

    expect(
      matchesDefinitionFilters(defs[0], {
        query: 'movie',
        type: 'private',
        language: 'en-US',
        contentType: 'Movies',
        searchMode: 'movie-search',
      })
    ).toBe(true);

    expect(
      matchesDefinitionFilters(defs[0], {
        query: '',
        type: '',
        language: '',
        contentType: 'Books',
        searchMode: '',
      })
    ).toBe(false);
  });
});
