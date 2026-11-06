import { describe, expect, it } from 'vitest';
import {
  buildNavGroups,
  canAccessView,
  firstAccessibleView,
  permissionsForView,
} from './nav';

const emptyBadges = {
  series: 0,
  movies: 0,
  artists: 0,
  books: 0,
  indexers: 0,
  queue: 0,
  plugins: 0,
  users: 0,
};

const viewerPerms = [
  'tv:read',
  'movies:read',
  'music:read',
  'books:read',
  'indexers:read',
  'downloads:read',
  'profiles:read',
  'system:read',
];

describe('permissionsForView', () => {
  it('maps media and admin surfaces to API permissions', () => {
    expect(permissionsForView('dashboard')).toEqual([]);
    expect(permissionsForView('series.library')).toEqual(['tv:read']);
    expect(permissionsForView('series.import')).toEqual(['tv:read', 'system:manage']);
    expect(permissionsForView('movies.statistics')).toEqual(['movies:read', 'system:read']);
    expect(permissionsForView('users')).toEqual(['users:read']);
    expect(permissionsForView('settings.sessions')).toEqual([]);
    expect(permissionsForView('settings.profiles')).toEqual(['profiles:read']);
    expect(permissionsForView('system.logs')).toEqual(['system:read']);
  });
});

describe('buildNavGroups RBAC filtering', () => {
  it('hides admin-only sections for viewers', () => {
    const groups = buildNavGroups(emptyBadges, viewerPerms);
    const ids = groups.map((g) => g.id);
    expect(ids).toContain('dashboard');
    expect(ids).toContain('series');
    expect(ids).toContain('indexers');
    expect(ids).toContain('queue');
    expect(ids).toContain('settings');
    expect(ids).toContain('system');
    expect(ids).not.toContain('plugins');
    expect(ids).not.toContain('users');

    const series = groups.find((g) => g.id === 'series')!;
    expect(series.children?.map((c) => c.id)).toEqual([
      'series.library',
      'series.blocklist',
      'series.statistics',
    ]);
  });

  it('shows users and plugins when granted', () => {
    const groups = buildNavGroups(emptyBadges, [...viewerPerms, 'plugins:read', 'users:read']);
    expect(groups.map((g) => g.id)).toEqual(
      expect.arrayContaining(['plugins', 'users'])
    );
  });

  it('keeps sessions when system:read is missing', () => {
    const groups = buildNavGroups(emptyBadges, ['tv:read']);
    const settings = groups.find((g) => g.id === 'settings');
    expect(settings?.children?.map((c) => c.id)).toEqual(['settings.sessions']);
    expect(groups.map((g) => g.id)).not.toContain('system');
    expect(groups.map((g) => g.id)).not.toContain('calendar');
  });

  it('admin star unlocks every group', () => {
    const groups = buildNavGroups(emptyBadges, ['*']);
    expect(groups.map((g) => g.id)).toEqual([
      'dashboard',
      'calendar',
      'movies',
      'series',
      'music',
      'books',
      'indexers',
      'queue',
      'plugins',
      'users',
      'settings',
      'system',
    ]);
  });
});

describe('canAccessView / firstAccessibleView', () => {
  it('denies media import without system:manage', () => {
    expect(canAccessView('series.import', viewerPerms)).toBe(false);
    expect(canAccessView('series.library', viewerPerms)).toBe(true);
  });

  it('falls back to dashboard when nothing else is allowed', () => {
    const groups = buildNavGroups(emptyBadges, []);
    expect(firstAccessibleView(groups)).toBe('dashboard');
  });
});
