import type { LucideIcon } from 'lucide-react';
import {
  LayoutDashboard,
  CalendarDays,
  Tv,
  Film,
  Music,
  BookOpen,
  Radio,
  DownloadCloud,
  Puzzle,
  Users,
  Settings,
  Server,
  FolderCog,
  Trash2,
  SlidersHorizontal,
  Gauge,
  ListPlus,
  Clapperboard,
  Captions,
  Tags,
  Palette,
  MonitorSmartphone,
  Activity,
  ListTodo,
  Archive,
  RefreshCw,
  History,
  ScrollText,
  FileText,
  Library,
  Upload,
  BarChart3,
  Ban,
} from 'lucide-react';
import { hasPermission } from './permissions';

/** Dot-separated view id, e.g. series.library, settings.media, system.logs */
export type NavView = string;

export type MediaNavType = 'series' | 'movies' | 'music' | 'books';

export interface NavChild {
  id: NavView;
  label: string;
  icon?: LucideIcon;
}

export interface NavGroup {
  id: string;
  label: string;
  icon: LucideIcon;
  accent: string;
  badge?: number | string;
  /** Leaf item with no children */
  view?: NavView;
  children?: NavChild[];
  defaultChild?: NavView;
}

const MEDIA_READ_PERM: Record<MediaNavType, string> = {
  series: 'tv:read',
  movies: 'movies:read',
  music: 'music:read',
  books: 'books:read',
};

const MEDIA_CHILDREN = (prefix: MediaNavType): NavChild[] => [
  { id: `${prefix}.library`, label: 'Library', icon: Library },
  { id: `${prefix}.import`, label: 'Library Import', icon: Upload },
  { id: `${prefix}.blocklist`, label: 'Blocklist', icon: Ban },
  { id: `${prefix}.statistics`, label: 'Statistics', icon: BarChart3 },
];

export const SETTINGS_CHILDREN: NavChild[] = [
  { id: 'settings.general', label: 'General', icon: Settings },
  { id: 'settings.ui', label: 'UI', icon: Palette },
  { id: 'settings.media', label: 'Media Management', icon: FolderCog },
  { id: 'settings.profiles', label: 'Profiles', icon: SlidersHorizontal },
  { id: 'settings.quality', label: 'Quality', icon: Gauge },
  { id: 'settings.importLists', label: 'Import Lists', icon: ListPlus },
  { id: 'settings.metadata', label: 'Metadata', icon: Clapperboard },
  { id: 'settings.subtitles', label: 'Subtitles', icon: Captions },
  { id: 'settings.tags', label: 'Tags', icon: Tags },
  { id: 'settings.sessions', label: 'Sessions', icon: MonitorSmartphone },
  { id: 'settings.recycleBin', label: 'Recycle Bin', icon: Trash2 },
];

export const SYSTEM_CHILDREN: NavChild[] = [
  { id: 'system.status', label: 'Status', icon: Activity },
  { id: 'system.tasks', label: 'Tasks', icon: ListTodo },
  { id: 'system.backup', label: 'Backup', icon: Archive },
  { id: 'system.updates', label: 'Updates', icon: RefreshCw },
  { id: 'system.events', label: 'Events', icon: History },
  { id: 'system.logs', label: 'Logs', icon: ScrollText },
  { id: 'system.changelog', label: 'Changelog', icon: FileText },
];

/**
 * Permissions required to see a nav view (all must match).
 * Empty list = any authenticated user. Aligns with native API RequirePermission.
 */
export function permissionsForView(view: NavView): string[] {
  if (view === 'dashboard' || view === 'settings.sessions') return [];
  if (view === 'calendar') return ['system:read'];
  if (view === 'indexers') return ['indexers:read'];
  if (view === 'queue') return ['downloads:read'];
  if (view === 'plugins') return ['plugins:read'];
  if (view === 'users') return ['users:read'];

  const [prefix, sub] = view.split('.', 2);
  if (prefix === 'series' || prefix === 'movies' || prefix === 'music' || prefix === 'books') {
    const mediaRead = MEDIA_READ_PERM[prefix];
    if (sub === 'import') return [mediaRead, 'system:manage'];
    if (sub === 'statistics' || sub === 'blocklist') return [mediaRead, 'system:read'];
    return [mediaRead];
  }
  if (prefix === 'settings') {
    if (sub === 'profiles' || sub === 'quality') return ['profiles:read'];
    return ['system:read'];
  }
  if (prefix === 'system') return ['system:read'];
  return ['system:read'];
}

export function canAccessView(view: NavView, permissions: string[] | null | undefined): boolean {
  return permissionsForView(view).every((perm) => hasPermission(permissions, perm));
}

function filterMediaChildren(prefix: MediaNavType, permissions: string[] | null | undefined): NavChild[] {
  if (!hasPermission(permissions, MEDIA_READ_PERM[prefix])) return [];
  return MEDIA_CHILDREN(prefix).filter((child) => canAccessView(child.id, permissions));
}

function withFilteredChildren(
  group: NavGroup,
  children: NavChild[],
  preferredDefault?: NavView
): NavGroup | null {
  if (!children.length) return null;
  const defaultChild =
    preferredDefault && children.some((c) => c.id === preferredDefault)
      ? preferredDefault
      : children[0].id;
  return { ...group, children, defaultChild };
}

export function buildNavGroups(
  badges: {
    series: number;
    movies: number;
    artists: number;
    books: number;
    indexers: number;
    queue: number;
    plugins: number;
    users: number;
  },
  permissions: string[] | null | undefined = null
): NavGroup[] {
  const groups: Array<NavGroup | null> = [
    {
      id: 'dashboard',
      label: 'Dashboard',
      icon: LayoutDashboard,
      accent: 'text-indigo-400',
      view: 'dashboard',
    },
    canAccessView('calendar', permissions)
      ? {
          id: 'calendar',
          label: 'Calendar',
          icon: CalendarDays,
          accent: 'text-cyan-400',
          view: 'calendar',
        }
      : null,
    withFilteredChildren(
      {
        id: 'movies',
        label: 'Movies',
        icon: Film,
        accent: 'text-emerald-400',
        badge: badges.movies,
      },
      filterMediaChildren('movies', permissions),
      'movies.library'
    ),
    withFilteredChildren(
      {
        id: 'series',
        label: 'TV Series',
        icon: Tv,
        accent: 'text-indigo-400',
        badge: badges.series,
      },
      filterMediaChildren('series', permissions),
      'series.library'
    ),
    withFilteredChildren(
      {
        id: 'music',
        label: 'Music',
        icon: Music,
        accent: 'text-fuchsia-400',
        badge: badges.artists,
      },
      filterMediaChildren('music', permissions),
      'music.library'
    ),
    withFilteredChildren(
      {
        id: 'books',
        label: 'Books',
        icon: BookOpen,
        accent: 'text-amber-300',
        badge: badges.books,
      },
      filterMediaChildren('books', permissions),
      'books.library'
    ),
    canAccessView('indexers', permissions)
      ? {
          id: 'indexers',
          label: 'Indexers',
          icon: Radio,
          accent: 'text-amber-400',
          badge: badges.indexers,
          view: 'indexers',
        }
      : null,
    canAccessView('queue', permissions)
      ? {
          id: 'queue',
          label: 'Queue & Clients',
          icon: DownloadCloud,
          accent: 'text-sky-400',
          badge: badges.queue,
          view: 'queue',
        }
      : null,
    canAccessView('plugins', permissions)
      ? {
          id: 'plugins',
          label: 'Plugins',
          icon: Puzzle,
          accent: 'text-purple-400',
          badge: badges.plugins,
          view: 'plugins',
        }
      : null,
    canAccessView('users', permissions)
      ? {
          id: 'users',
          label: 'Users & Roles',
          icon: Users,
          accent: 'text-emerald-400',
          badge: badges.users,
          view: 'users',
        }
      : null,
    withFilteredChildren(
      {
        id: 'settings',
        label: 'Settings',
        icon: Settings,
        accent: 'text-indigo-300',
      },
      SETTINGS_CHILDREN.filter((child) => canAccessView(child.id, permissions)),
      'settings.media'
    ),
    withFilteredChildren(
      {
        id: 'system',
        label: 'System',
        icon: Server,
        accent: 'text-slate-300',
      },
      SYSTEM_CHILDREN.filter((child) => canAccessView(child.id, permissions)),
      'system.status'
    ),
  ];

  return groups.filter((g): g is NavGroup => g != null);
}

/** First reachable leaf/default view in the filtered nav, for unauthorized redirects. */
export function firstAccessibleView(groups: NavGroup[]): NavView {
  for (const g of groups) {
    if (g.view) return g.view;
    if (g.children?.length) return g.defaultChild ?? g.children[0].id;
  }
  return 'dashboard';
}

export function viewLabel(view: NavView, groups: NavGroup[]): string {
  for (const g of groups) {
    if (g.view === view) return g.label;
    if (g.children) {
      const child = g.children.find((c) => c.id === view);
      if (child) return `${g.label} · ${child.label}`;
    }
  }
  return 'Dashboard';
}

export function groupForView(view: NavView): string {
  if (view === 'dashboard' || view === 'calendar' || view === 'indexers' || view === 'queue' || view === 'plugins' || view === 'users') {
    return view;
  }
  const prefix = view.split('.')[0] ?? view;
  return prefix;
}

export function mediaTypeFromView(view: NavView): MediaNavType | null {
  const prefix = view.split('.')[0];
  if (prefix === 'series' || prefix === 'movies' || prefix === 'music' || prefix === 'books') {
    return prefix;
  }
  return null;
}

export function mediaTypeApi(media: MediaNavType): 'tv' | 'movies' | 'music' | 'books' {
  if (media === 'series') return 'tv';
  return media;
}

export function flattenVisibleNavItems(
  groups: NavGroup[],
  expanded: Record<string, boolean>
): Array<{ view: NavView; groupId: string; isChild: boolean }> {
  const out: Array<{ view: NavView; groupId: string; isChild: boolean }> = [];
  for (const g of groups) {
    if (g.view) {
      out.push({ view: g.view, groupId: g.id, isChild: false });
      continue;
    }
    if (g.children?.length) {
      out.push({ view: g.defaultChild ?? g.children[0].id, groupId: g.id, isChild: false });
      if (expanded[g.id]) {
        for (const c of g.children) {
          out.push({ view: c.id, groupId: g.id, isChild: true });
        }
      }
    }
  }
  return out;
}
