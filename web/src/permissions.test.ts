import { describe, expect, it } from 'vitest';
import { hasAnyPermission, hasPermission } from './permissions';

describe('hasPermission', () => {
  it('allows empty required permission for any caller', () => {
    expect(hasPermission(undefined, '')).toBe(true);
    expect(hasPermission([], '')).toBe(true);
  });

  it('matches exact permission and admin star', () => {
    expect(hasPermission(['tv:read'], 'tv:read')).toBe(true);
    expect(hasPermission(['tv:read'], 'tv:write')).toBe(false);
    expect(hasPermission(['*'], 'users:manage')).toBe(true);
  });

  it('rejects missing grants', () => {
    expect(hasPermission(null, 'tv:read')).toBe(false);
    expect(hasPermission([], 'tv:read')).toBe(false);
  });
});

describe('hasAnyPermission', () => {
  it('returns true when any listed permission matches', () => {
    expect(hasAnyPermission(['movies:read'], ['tv:read', 'movies:read'])).toBe(true);
    expect(hasAnyPermission(['downloads:read'], ['tv:read', 'movies:read'])).toBe(false);
  });
});
