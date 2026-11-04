import { SETTINGS_CHILDREN, SYSTEM_CHILDREN, type MediaNavType, type NavView } from './nav';

const MEDIA_TYPES: MediaNavType[] = ['series', 'movies', 'music', 'books'];
const MEDIA_SUBS = ['library', 'import', 'statistics', 'blocklist'] as const;
const LEAF_VIEWS = new Set<NavView>(['dashboard', 'calendar', 'indexers', 'queue', 'plugins', 'users']);

const KNOWN_VIEWS: Set<NavView> = (() => {
  const views = new Set<NavView>(LEAF_VIEWS);
  for (const media of MEDIA_TYPES) {
    for (const sub of MEDIA_SUBS) {
      views.add(`${media}.${sub}`);
    }
  }
  for (const child of SETTINGS_CHILDREN) views.add(child.id);
  for (const child of SYSTEM_CHILDREN) views.add(child.id);
  return views;
})();

export function isKnownView(view: string): view is NavView {
  return KNOWN_VIEWS.has(view);
}

/** Map a nav view to a pathname (SPA deep link). */
export function viewToPath(view: NavView): string {
  if (view === 'dashboard') return '/';
  return `/${view.split('.').join('/')}`;
}

/** Parse a pathname into a nav view, or null if unrecognized. */
export function pathToView(pathname: string): NavView | null {
  const raw = pathname.replace(/\/+$/, '') || '/';
  if (raw === '/' || raw === '') return 'dashboard';

  const parts = raw.replace(/^\//, '').split('/').filter(Boolean);
  if (parts.length === 0) return 'dashboard';

  if (parts.length === 1) {
    const id = parts[0];
    if (id === 'dashboard') return 'dashboard';
    if (LEAF_VIEWS.has(id)) return id;
    if ((MEDIA_TYPES as string[]).includes(id)) return `${id}.library`;
    if (id === 'settings') return 'settings.media';
    if (id === 'system') return 'system.status';
    return null;
  }

  const view = parts.join('.');
  return isKnownView(view) ? view : null;
}

export function readViewFromLocation(loc: Pick<Location, 'pathname'> = window.location): NavView {
  return pathToView(loc.pathname) ?? 'dashboard';
}

export function syncViewToHistory(view: NavView, mode: 'push' | 'replace'): void {
  const path = viewToPath(view);
  const state = { view };
  if (mode === 'replace') {
    window.history.replaceState(state, '', path);
    return;
  }
  const current = window.history.state as { view?: string } | null;
  if (current?.view === view && window.location.pathname === path) return;
  window.history.pushState(state, '', path);
}
