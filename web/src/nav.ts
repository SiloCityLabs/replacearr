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

export function buildNavGroups(badges: {
  series: number;
  movies: number;
  artists: number;
  books: number;
  indexers: number;
  queue: number;
  plugins: number;
  users: number;
}): NavGroup[] {
  return [
    {
      id: 'dashboard',
      label: 'Dashboard',
      icon: LayoutDashboard,
      accent: 'text-indigo-400',
      view: 'dashboard',
    },
    {
      id: 'calendar',
      label: 'Calendar',
      icon: CalendarDays,
      accent: 'text-cyan-400',
      view: 'calendar',
    },
    {
      id: 'movies',
      label: 'Movies',
      icon: Film,
      accent: 'text-emerald-400',
      badge: badges.movies,
      children: MEDIA_CHILDREN('movies'),
      defaultChild: 'movies.library',
    },
    {
      id: 'series',
      label: 'TV Series',
      icon: Tv,
      accent: 'text-indigo-400',
      badge: badges.series,
      children: MEDIA_CHILDREN('series'),
      defaultChild: 'series.library',
    },
    {
      id: 'music',
      label: 'Music',
      icon: Music,
      accent: 'text-fuchsia-400',
      badge: badges.artists,
      children: MEDIA_CHILDREN('music'),
      defaultChild: 'music.library',
    },
    {
      id: 'books',
      label: 'Books',
      icon: BookOpen,
      accent: 'text-amber-300',
      badge: badges.books,
      children: MEDIA_CHILDREN('books'),
      defaultChild: 'books.library',
    },
    {
      id: 'indexers',
      label: 'Indexers',
      icon: Radio,
      accent: 'text-amber-400',
      badge: badges.indexers,
      view: 'indexers',
    },
    {
      id: 'queue',
      label: 'Queue & Clients',
      icon: DownloadCloud,
      accent: 'text-sky-400',
      badge: badges.queue,
      view: 'queue',
    },
    {
      id: 'plugins',
      label: 'Plugins',
      icon: Puzzle,
      accent: 'text-purple-400',
      badge: badges.plugins,
      view: 'plugins',
    },
    {
      id: 'users',
      label: 'Users & Roles',
      icon: Users,
      accent: 'text-emerald-400',
      badge: badges.users,
      view: 'users',
    },
    {
      id: 'settings',
      label: 'Settings',
      icon: Settings,
      accent: 'text-indigo-300',
      children: SETTINGS_CHILDREN,
      defaultChild: 'settings.media',
    },
    {
      id: 'system',
      label: 'System',
      icon: Server,
      accent: 'text-slate-300',
      children: SYSTEM_CHILDREN,
      defaultChild: 'system.status',
    },
  ];
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
