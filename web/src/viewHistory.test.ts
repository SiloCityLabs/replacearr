import { describe, expect, it } from 'vitest';
import { isKnownView, pathToView, viewToPath } from './viewHistory';

describe('viewHistory', () => {
  it('maps views to paths', () => {
    expect(viewToPath('dashboard')).toBe('/');
    expect(viewToPath('calendar')).toBe('/calendar');
    expect(viewToPath('series.library')).toBe('/series/library');
    expect(viewToPath('settings.media')).toBe('/settings/media');
    expect(viewToPath('system.logs')).toBe('/system/logs');
  });

  it('parses paths back to views', () => {
    expect(pathToView('/')).toBe('dashboard');
    expect(pathToView('/calendar')).toBe('calendar');
    expect(pathToView('/series')).toBe('series.library');
    expect(pathToView('/series/library')).toBe('series.library');
    expect(pathToView('/movies/blocklist')).toBe('movies.blocklist');
    expect(pathToView('/settings')).toBe('settings.media');
    expect(pathToView('/settings/ui')).toBe('settings.ui');
    expect(pathToView('/settings/recycleBin')).toBe('settings.recycleBin');
    expect(pathToView('/settings/sessions')).toBe('settings.sessions');
    expect(pathToView('/system')).toBe('system.status');
    expect(pathToView('/nope')).toBeNull();
    expect(pathToView('/settings/nope')).toBeNull();
  });

  it('validates known views', () => {
    expect(isKnownView('queue')).toBe(true);
    expect(isKnownView('books.import')).toBe(true);
    expect(isKnownView('settings.recycleBin')).toBe(true);
    expect(isKnownView('settings.sessions')).toBe(true);
    expect(isKnownView('not.a.view')).toBe(false);
  });
});
