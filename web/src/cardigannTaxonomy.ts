import type { CardigannDefinition } from './types';

/** Newznab parent categories that show up as useful browse filters. */
export const CONTENT_TYPE_OPTIONS = [
  'Movies',
  'TV',
  'Audio',
  'Books',
  'XXX',
  'PC',
  'Console',
  'Other',
] as const;

export type ContentType = (typeof CONTENT_TYPE_OPTIONS)[number];

export const PRIVACY_TYPE_OPTIONS = [
  { value: 'public', label: 'Public' },
  { value: 'semi-private', label: 'Semi-private' },
  { value: 'private', label: 'Private' },
] as const;

export const SEARCH_MODE_OPTIONS = [
  { value: 'tv-search', label: 'TV search' },
  { value: 'movie-search', label: 'Movie search' },
  { value: 'music-search', label: 'Music search' },
  { value: 'book-search', label: 'Book search' },
] as const;

export interface DefinitionTaxonomyFilters {
  query: string;
  type: string;
  language: string;
  contentType: string;
  searchMode: string;
}

export const emptyDefinitionFilters = (): DefinitionTaxonomyFilters => ({
  query: '',
  type: '',
  language: '',
  contentType: '',
  searchMode: '',
});

function asRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null;
}

/** Parent Newznab categories derived from caps.categorymappings[].cat */
export function definitionContentTypes(def: CardigannDefinition): string[] {
  const mappings = def.caps?.categorymappings;
  if (!Array.isArray(mappings)) return [];

  const parents = new Set<string>();
  for (const mapping of mappings) {
    const row = asRecord(mapping);
    const cat = typeof row?.cat === 'string' ? row.cat.trim() : '';
    if (!cat) continue;
    const parent = cat.split('/')[0]?.trim() ?? '';
    if (!parent || /^\d+$/.test(parent)) continue;
    parents.add(parent);
  }
  return [...parents].sort((a, b) => a.localeCompare(b));
}

/** Search capability keys from caps.modes */
export function definitionSearchModes(def: CardigannDefinition): string[] {
  const modes = asRecord(def.caps?.modes);
  if (!modes) return [];
  return Object.keys(modes)
    .filter((key) => Array.isArray(modes[key]))
    .sort((a, b) => a.localeCompare(b));
}

export function languageLabel(code: string): string {
  const normalized = code.trim() || 'en-US';
  try {
    const display = new Intl.DisplayNames(['en'], { type: 'language' });
    const name = display.of(normalized) ?? display.of(normalized.split('-')[0] ?? normalized);
    return name ? `${name} (${normalized})` : normalized;
  } catch {
    return normalized;
  }
}

export function uniqueDefinitionLanguages(definitions: CardigannDefinition[]): string[] {
  const langs = new Set<string>();
  for (const def of definitions) {
    langs.add((def.language || 'en-US').trim() || 'en-US');
  }
  return [...langs].sort((a, b) => a.localeCompare(b));
}

export function matchesDefinitionFilters(
  def: CardigannDefinition,
  filters: DefinitionTaxonomyFilters
): boolean {
  const q = filters.query.trim().toLowerCase();
  if (q) {
    const haystack = [
      def.name,
      def.id,
      def.description || '',
      def.fileName || '',
      ...(def.links ?? []),
    ]
      .join(' ')
      .toLowerCase();
    if (!haystack.includes(q)) return false;
  }

  if (filters.type) {
    const type = (def.type || 'public').toLowerCase();
    if (type !== filters.type.toLowerCase()) return false;
  }

  if (filters.language) {
    const language = (def.language || 'en-US').trim();
    if (language !== filters.language) return false;
  }

  if (filters.contentType) {
    if (!definitionContentTypes(def).includes(filters.contentType)) return false;
  }

  if (filters.searchMode) {
    if (!definitionSearchModes(def).includes(filters.searchMode)) return false;
  }

  return true;
}

export function filterDefinitions(
  definitions: CardigannDefinition[],
  filters: DefinitionTaxonomyFilters
): CardigannDefinition[] {
  return definitions.filter((def) => matchesDefinitionFilters(def, filters));
}
