import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  LogOut,
  RefreshCw,
  Shield,
  CheckCircle2,
  AlertCircle,
  Menu,
  X,
} from 'lucide-react';
import type {
  APIKey,
  Artist,
  Author,
  Book,
  CardigannDefinition,
  DiscoveredPlugin,
  DownloadClient,
  Edition,
  Indexer,
  Movie,
  OverviewResponse,
  QualityProfile,
  QueueItem,
  ReleaseSearchRequest,
  Role,
  RootFolder,
  Series,
  User,
  AppConfig,
} from './types';
import { api, getStoredToken, setStoredToken } from './api';
import { applyUiPreferences } from './uiPrefs';
import { BROWSER_PLUGIN_ID, subscribeBrowserNotificationStream } from './notifications';
import {
  buildNavGroups,
  canAccessView,
  firstAccessibleView,
  flattenVisibleNavItems,
  groupForView,
  mediaTypeFromView,
  viewLabel,
} from './nav';
import type { NavView } from './nav';
import { isKnownView, readViewFromLocation, syncViewToHistory } from './viewHistory';
import { SideNav } from './components/nav/SideNav';
import { LoginView } from './components/LoginView';
import { DashboardTab } from './components/DashboardTab';
import { CalendarTab } from './components/CalendarTab';
import { SeriesTab } from './components/SeriesTab';
import { MoviesTab } from './components/MoviesTab';
import { MusicTab } from './components/MusicTab';
import { BooksTab } from './components/BooksTab';
import { LibraryImportPage } from './components/media/LibraryImportPage';
import { StatisticsPage } from './components/media/StatisticsPage';
import { BlocklistPage } from './components/media/BlocklistPage';
import { IndexersSearchTab } from './components/IndexersSearchTab';
import { InteractiveSearchModal } from './components/InteractiveSearchModal';
import { SetupWizard } from './components/SetupWizard';
import { QueueClientsTab } from './components/QueueClientsTab';
import { PluginsTab } from './components/PluginsTab';
import { UsersRbacTab } from './components/UsersRbacTab';
import { SettingsTab, type SettingsPageId } from './components/settings/SettingsTab';
import { SystemStatusPage } from './components/system/SystemStatusPage';
import { SystemTasksPage } from './components/system/SystemTasksPage';
import { SystemBackupPage } from './components/system/SystemBackupPage';
import { SystemUpdatesPage } from './components/system/SystemUpdatesPage';
import { SystemEventsPage } from './components/system/SystemEventsPage';
import { SystemLogsPage } from './components/system/SystemLogsPage';
import { SystemChangelogPage } from './components/system/SystemChangelogPage';
import { useGamepad } from './htpc/useGamepad';
import { useRovingFocus } from './htpc/useRovingFocus';
import { useConfirm } from './components/modals/ConfirmModal';

function settingsPageFromView(view: NavView): SettingsPageId | null {
  if (!view.startsWith('settings.')) return null;
  const id = view.slice('settings.'.length) as SettingsPageId;
  return id;
}

export const App: React.FC = () => {
  const { confirm, dialog: confirmDialog } = useConfirm();
  const [user, setUser] = useState<User | null>(null);
  const [masterApiKey, setMasterApiKey] = useState<string>('');
  const [authChecking, setAuthChecking] = useState<boolean>(true);
  const [activeView, setActiveView] = useState<NavView>(() => readViewFromLocation());
  const [expandedNav, setExpandedNav] = useState<Record<string, boolean>>(() => ({
    [groupForView(readViewFromLocation())]: true,
  }));
  const [settingsDirty, setSettingsDirty] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const settingsDirtyRef = useRef(false);
  const activeViewRef = useRef<NavView>(activeView);
  const userPermissionsRef = useRef<string[]>([]);

  const [overview, setOverview] = useState<OverviewResponse | null>(null);
  const [series, setSeries] = useState<Series[]>([]);
  const [movies, setMovies] = useState<Movie[]>([]);
  const [artists, setArtists] = useState<Artist[]>([]);
  const [authors, setAuthors] = useState<Author[]>([]);
  const [books, setBooks] = useState<Book[]>([]);
  const [editions, setEditions] = useState<Edition[]>([]);
  const [indexers, setIndexers] = useState<Indexer[]>([]);
  const [definitions, setDefinitions] = useState<CardigannDefinition[]>([]);
  const [queue, setQueue] = useState<QueueItem[]>([]);
  const [clients, setClients] = useState<DownloadClient[]>([]);
  const [profiles, setProfiles] = useState<QualityProfile[]>([]);
  const [rootFolders, setRootFolders] = useState<RootFolder[]>([]);
  const [plugins, setPlugins] = useState<DiscoveredPlugin[]>([]);
  const [users, setUsers] = useState<User[]>([]);
  const [roles, setRoles] = useState<Role[]>([]);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [permissions, setPermissions] = useState<string[]>([]);
  const [appConfig, setAppConfig] = useState<AppConfig | null>(null);

  const [interactiveSearch, setInteractiveSearch] = useState<ReleaseSearchRequest | null>(null);
  const [pendingOpenAdd, setPendingOpenAdd] = useState(false);
  const [addPanelNonce, setAddPanelNonce] = useState(0);

  const navRef = useRef<HTMLElement | null>(null);
  const [toast, setToast] = useState<{ message: string; isError?: boolean } | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const notify = useCallback((message: string, isError = false) => {
    setToast({ message, isError });
  }, []);

  useEffect(() => {
    if (!toast) return;
    // Errors often need copy/paste time; success can clear quickly.
    const ms = toast.isError ? 20000 : 4000;
    const timer = setTimeout(() => setToast(null), ms);
    return () => clearTimeout(timer);
  }, [toast]);

  useEffect(() => {
    if (!mobileNavOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setMobileNavOpen(false);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [mobileNavOpen]);

  const loadAllData = useCallback(async () => {
    setRefreshing(true);
    try {
      const [
        ov,
        sr,
        mv,
        ar,
        au,
        bk,
        ed,
        idx,
        defs,
        q,
        dc,
        qp,
        rf,
        pl,
        us,
        rl,
        ak,
        perms,
        cfg,
      ] = await Promise.all([
        api.getOverview().catch(() => null),
        api.getSeries().catch(() => []),
        api.getMovies().catch(() => []),
        api.getArtists().catch(() => []),
        api.getAuthors().catch(() => []),
        api.getBooks().catch(() => []),
        api.getEditions().catch(() => []),
        api.getIndexers().catch(() => []),
        api.getCardigannDefinitions().catch(() => []),
        api.getQueue().catch(() => []),
        api.getDownloadClients().catch(() => []),
        api.getProfiles().catch(() => []),
        api.getRootFolders().catch(() => []),
        api.getPlugins().catch(() => []),
        api.getUsers().catch(() => []),
        api.getRoles().catch(() => []),
        api.getApiKeys().catch(() => []),
        api.getPermissions().catch(() => []),
        api.getConfig().catch(() => null),
      ]);

      if (ov) setOverview(ov);
      setSeries(sr);
      setMovies(mv);
      setArtists(ar);
      setAuthors(au);
      setBooks(bk);
      setEditions(ed);
      setIndexers(idx);
      setDefinitions(defs);
      setQueue(q);
      setClients(dc);
      setProfiles(qp);
      setRootFolders(rf);
      setPlugins(pl);
      setUsers(us);
      setRoles(rl);
      setApiKeys(ak);
      setPermissions(perms);
      if (cfg) {
        const normalized: AppConfig = {
          ...cfg,
          setupComplete: Boolean(cfg.setupComplete),
          mediaManagement: {
            ...cfg.mediaManagement,
            unwantedFileExtensions: cfg.mediaManagement?.unwantedFileExtensions ?? '.exe',
            tvUnwantedFileExtensions: cfg.mediaManagement?.tvUnwantedFileExtensions ?? '',
            movieUnwantedFileExtensions: cfg.mediaManagement?.movieUnwantedFileExtensions ?? '',
            musicUnwantedFileExtensions: cfg.mediaManagement?.musicUnwantedFileExtensions ?? '',
            bookUnwantedFileExtensions: cfg.mediaManagement?.bookUnwantedFileExtensions ?? '',
            deleteUnwantedFiles: cfg.mediaManagement?.deleteUnwantedFiles ?? true,
          },
          subtitles: cfg.subtitles ?? {
            enabled: true,
            autoDownload: true,
            languages: ['en'],
            preferHearingImpaired: false,
            includeForced: true,
            openSubtitles: { apiKey: '', username: '', password: '' },
          },
          downloadClientOptions: {
            ...cfg.downloadClientOptions,
            discoverSubnets: cfg.downloadClientOptions?.discoverSubnets ?? '',
            hostProcNet: cfg.downloadClientOptions?.hostProcNet ?? '/host/proc/net',
          },
          plugins: {
            catalogUrl:
              cfg.plugins?.catalogUrl ??
              'https://raw.githubusercontent.com/replacearr/replacearr-plugins/main/registry.json',
          },
        };
        setAppConfig(normalized);
        applyUiPreferences(normalized.ui);
      }
    } finally {
      setRefreshing(false);
    }
  }, []);

  const refreshQueue = useCallback(async () => {
    try {
      const q = await api.getQueue();
      setQueue(q);
    } catch {
      /* keep last known queue on transient poll failures */
    }
  }, []);

  useEffect(() => {
    const checkSession = async () => {
      const token = getStoredToken();
      if (!token) {
        setAuthChecking(false);
        return;
      }
      try {
        const res = await api.me();
        setUser(res.user);
        setMasterApiKey(res.masterApiKey || '');
        await loadAllData();
      } catch {
        setStoredToken(null);
        setUser(null);
      } finally {
        setAuthChecking(false);
      }
    };
    void checkSession();
  }, [loadAllData]);

  const browserPluginEnabled = plugins.some(
    (p) => p.manifest.id === BROWSER_PLUGIN_ID && p.enabled
  );

  useEffect(() => {
    if (!user || !browserPluginEnabled) {
      return;
    }
    return subscribeBrowserNotificationStream(true);
  }, [user, browserPluginEnabled]);

  const handleLogin = async (username: string, password: string, rememberMe: boolean) => {
    const res = await api.login(username, password, rememberMe);
    setUser(res.user);
    setMasterApiKey(res.masterApiKey || '');
    await loadAllData();
    notify(`Signed in as ${res.user.username} (${res.user.roleName})`);
  };

  const handleLogout = async () => {
    await api.logout();
    setUser(null);
    setMasterApiKey('');
  };

  const handleInteractiveSearch = (req: ReleaseSearchRequest) => {
    if (!(req.seriesId || req.movieId || req.artistId || req.albumId || req.authorId || req.bookId)) {
      notify('Interactive Search requires a library item', true);
      return;
    }
    setInteractiveSearch(req);
  };

  useEffect(() => {
    settingsDirtyRef.current = settingsDirty;
  }, [settingsDirty]);

  useEffect(() => {
    activeViewRef.current = activeView;
  }, [activeView]);

  useEffect(() => {
    userPermissionsRef.current = user?.permissions ?? [];
  }, [user?.permissions]);

  // Normalize the landing URL and attach history state so the first in-app
  // navigation creates a real Back entry instead of leaving the tab.
  useEffect(() => {
    syncViewToHistory(activeViewRef.current, 'replace');
  }, []);

  const applyView = useCallback((view: NavView) => {
    if (!canAccessView(view, userPermissionsRef.current)) {
      return;
    }
    const group = groupForView(view);
    // Accordion: only the destination section stays open.
    setExpandedNav({ [group]: true });
    setActiveView(view);
    setMobileNavOpen(false);
  }, []);

  const navigateTo = useCallback(
    (view: NavView) => {
      if (!canAccessView(view, userPermissionsRef.current)) {
        return;
      }
      if (view === activeViewRef.current) {
        setMobileNavOpen(false);
        return;
      }
      void (async () => {
        if (
          settingsDirtyRef.current &&
          activeViewRef.current.startsWith('settings.') &&
          !view.startsWith('settings.')
        ) {
          const ok = await confirm({
            title: 'Unsaved changes',
            message: 'Discard unsaved settings changes?',
            confirmLabel: 'Discard',
          });
          if (!ok) return;
          setSettingsDirty(false);
        }
        applyView(view);
        syncViewToHistory(view, 'push');
      })();
    },
    [applyView, confirm]
  );

  useEffect(() => {
    const onPopState = (event: PopStateEvent) => {
      const stateView = (event.state as { view?: string } | null)?.view;
      const view =
        (typeof stateView === 'string' && isKnownView(stateView) && stateView) ||
        readViewFromLocation();
      void (async () => {
        if (!canAccessView(view, userPermissionsRef.current)) {
          syncViewToHistory(activeViewRef.current, 'replace');
          return;
        }
        if (
          settingsDirtyRef.current &&
          activeViewRef.current.startsWith('settings.') &&
          !view.startsWith('settings.')
        ) {
          const ok = await confirm({
            title: 'Unsaved changes',
            message: 'Discard unsaved settings changes?',
            confirmLabel: 'Discard',
          });
          if (!ok) {
            // Stay on the settings page; put the current view back on the stack.
            syncViewToHistory(activeViewRef.current, 'push');
            return;
          }
          setSettingsDirty(false);
        }
        applyView(view);
      })();
    };
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, [applyView, confirm]);

  const handleOpenAddLibrary = (tab: 'series' | 'movies' | 'music' | 'books') => {
    setPendingOpenAdd(true);
    setAddPanelNonce((n) => n + 1);
    navigateTo(`${tab}.library`);
  };

  const saveAppConfig = useCallback(async (next: AppConfig): Promise<AppConfig> => {
    const saved = await api.saveConfig(next);
    setAppConfig(saved);
    applyUiPreferences(saved.ui);
    return saved;
  }, []);

  const userPermissions = user?.permissions;

  const navGroups = useMemo(
    () =>
      buildNavGroups(
        {
          series: series.length,
          movies: movies.length,
          artists: artists.length,
          books: books.length,
          indexers: indexers.length,
          queue: queue.length,
          plugins: plugins.length,
          users: users.length,
        },
        userPermissions
      ),
    [
      series.length,
      movies.length,
      artists.length,
      books.length,
      indexers.length,
      queue.length,
      plugins.length,
      users.length,
      userPermissions,
    ]
  );

  // Deep links / stale history: bounce to the first allowed view.
  useEffect(() => {
    if (!user) return;
    if (canAccessView(activeView, user.permissions)) return;
    const fallback = firstAccessibleView(navGroups);
    applyView(fallback);
    syncViewToHistory(fallback, 'replace');
  }, [user, activeView, navGroups, applyView]);

  const visibleNav = useMemo(
    () => flattenVisibleNavItems(navGroups, expandedNav),
    [navGroups, expandedNav]
  );

  const navRoving = useRovingFocus({
    itemCount: visibleNav.length,
    orientation: 'vertical',
    onActivate: (index) => {
      const item = visibleNav[index];
      if (item) navigateTo(item.view);
    },
  });

  useGamepad((action) => {
    const focusNavItem = (delta: number) => {
      const next = navRoving.move(delta);
      const items = navRef.current?.querySelectorAll<HTMLElement>('[data-roving-item="true"]');
      items?.[next]?.focus();
    };

    switch (action) {
      case 'up':
        focusNavItem(-1);
        break;
      case 'down':
        focusNavItem(1);
        break;
      case 'confirm': {
        const el = document.activeElement;
        if (el instanceof HTMLElement) {
          if (el.tagName === 'BUTTON' || el.getAttribute('role') === 'button') {
            el.click();
          } else {
            el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
          }
        }
        break;
      }
      case 'back':
        if (activeView !== 'dashboard') {
          navigateTo('dashboard');
        }
        break;
      default:
        break;
    }
  }, Boolean(user));

  if (authChecking) {
    return (
      <div className="min-h-screen bg-slate-950 text-slate-200 flex items-center justify-center">
        <div className="flex items-center gap-3 text-sm text-slate-400">
          <RefreshCw className="w-5 h-5 text-indigo-400 animate-spin" />
          <span>Loading Replacearr control plane...</span>
        </div>
      </div>
    );
  }

  if (!user) {
    return <LoginView onLogin={handleLogin} />;
  }

  if (appConfig && !appConfig.setupComplete) {
    return (
      <SetupWizard
        config={appConfig}
        onComplete={(saved) => {
          setAppConfig(saved);
          applyUiPreferences(saved.ui);
          void loadAllData();
        }}
        onLogout={() => void handleLogout()}
        onNotify={notify}
      />
    );
  }

  const activeNavLabel = viewLabel(activeView, navGroups);
  const media = mediaTypeFromView(activeView);
  const mediaSub = media ? activeView.split('.')[1] : null;
  const settingsPage = settingsPageFromView(activeView);

  const renderMediaLibrary = () => {
    if (!media || mediaSub !== 'library') return null;
    if (media === 'series') {
      return (
        <SeriesTab
          series={series}
          profiles={profiles}
          rootFolders={rootFolders}
          onRefresh={loadAllData}
          onSearchReleases={handleInteractiveSearch}
          onNotify={notify}
          appUi={appConfig?.ui}
          openAddPanel={pendingOpenAdd}
          openAddNonce={addPanelNonce}
          onAddPanelOpened={() => setPendingOpenAdd(false)}
        />
      );
    }
    if (media === 'movies') {
      return (
        <MoviesTab
          movies={movies}
          profiles={profiles}
          rootFolders={rootFolders}
          onRefresh={loadAllData}
          onSearchReleases={handleInteractiveSearch}
          onNotify={notify}
          openAddPanel={pendingOpenAdd}
          openAddNonce={addPanelNonce}
          onAddPanelOpened={() => setPendingOpenAdd(false)}
        />
      );
    }
    if (media === 'music') {
      return (
        <MusicTab
          artists={artists}
          profiles={profiles}
          rootFolders={rootFolders}
          onRefresh={loadAllData}
          onSearchReleases={handleInteractiveSearch}
          onNotify={notify}
          openAddPanel={pendingOpenAdd}
          openAddNonce={addPanelNonce}
          onAddPanelOpened={() => setPendingOpenAdd(false)}
        />
      );
    }
    return (
      <BooksTab
        authors={authors}
        books={books}
        editions={editions}
        profiles={profiles}
        rootFolders={rootFolders}
        onRefresh={loadAllData}
        onSearchReleases={handleInteractiveSearch}
        onNotify={notify}
        openAddPanel={pendingOpenAdd}
        openAddNonce={addPanelNonce}
        onAddPanelOpened={() => setPendingOpenAdd(false)}
      />
    );
  };

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col lg:flex-row">
      <aside className="w-full lg:w-64 bg-slate-900/95 border-b lg:border-b-0 lg:border-r border-slate-800 shrink-0 flex flex-col justify-between relative z-40 sticky top-0 lg:static">
        <div>
          <div className="h-14 px-4 lg:h-auto lg:p-5 border-b border-slate-800 flex items-center gap-3">
            <div className="flex items-center gap-3 min-w-0">
              <img src="/icon-192.png" alt="Replacearr" className="w-8 h-8 lg:w-9 lg:h-9 rounded-xl shrink-0" />
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="lg:hidden font-semibold text-sm text-white truncate">{activeNavLabel}</span>
                  <span className="hidden lg:inline font-bold text-base text-white tracking-tight truncate">
                    Replacearr
                  </span>
                  <span className="hidden lg:inline text-[10px] font-mono px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300 shrink-0">
                    Go
                  </span>
                </div>
              </div>
            </div>

            <button
              type="button"
              className="lg:hidden ml-auto inline-flex items-center justify-center min-h-10 min-w-10 rounded-lg bg-slate-950/60 border border-slate-800 text-slate-200 hover:bg-slate-800 transition"
              aria-label={mobileNavOpen ? 'Close navigation' : 'Open navigation'}
              aria-expanded={mobileNavOpen}
              aria-controls="app-primary-nav"
              onClick={() => setMobileNavOpen((open) => !open)}
            >
              {mobileNavOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
            </button>
          </div>

          <nav
            id="app-primary-nav"
            ref={navRef}
            className={`${
              mobileNavOpen ? 'flex' : 'hidden'
            } lg:flex absolute lg:static inset-x-0 top-14 lg:top-auto p-3 flex-col gap-1 bg-slate-900 border-b border-slate-800 lg:border-b-0 shadow-xl lg:shadow-none max-h-[calc(100dvh-3.5rem)] overflow-y-auto`}
          >
            <SideNav
              groups={navGroups}
              activeView={activeView}
              expanded={expandedNav}
              onToggleGroup={(id) => setExpandedNav((prev) => ({ ...prev, [id]: !prev[id] }))}
              onSelectView={navigateTo}
              itemProps={(index) => navRoving.itemProps(index)}
              footer={
                <div className="lg:hidden mt-2 pt-3 border-t border-slate-800 space-y-2 px-1">
                  <button
                    type="button"
                    onClick={() => {
                      void loadAllData();
                      setMobileNavOpen(false);
                    }}
                    disabled={refreshing}
                    className="w-full inline-flex items-center justify-between gap-2.5 px-3 py-2.5 rounded-xl text-xs font-medium text-slate-300 bg-slate-950 hover:bg-slate-800 border border-slate-800 transition disabled:opacity-60"
                  >
                    <span className="inline-flex items-center gap-2.5">
                      <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin text-indigo-400' : ''}`} />
                      Sync
                    </span>
                  </button>
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="text-xs font-bold text-white truncate">{user.username}</span>
                        <span className="inline-flex items-center gap-0.5 text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                          <Shield className="w-2.5 h-2.5" />
                          {user.roleName}
                        </span>
                      </div>
                    </div>
                    <button
                      type="button"
                      onClick={handleLogout}
                      title="Sign Out"
                      className="p-2 rounded-lg bg-slate-950 hover:bg-rose-500/15 text-slate-400 hover:text-rose-300 border border-slate-800 transition"
                    >
                      <LogOut className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              }
            />
          </nav>
        </div>

        <div className="hidden lg:block p-4 border-t border-slate-800 bg-slate-950/40">
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-bold text-white truncate">{user.username}</span>
                <span className="inline-flex items-center gap-0.5 text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                  <Shield className="w-2.5 h-2.5" />
                  {user.roleName}
                </span>
              </div>
            </div>
            <button
              onClick={handleLogout}
              title="Sign Out"
              className="p-2 rounded-lg bg-slate-900 hover:bg-rose-500/15 text-slate-400 hover:text-rose-300 border border-slate-800 transition"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {mobileNavOpen && (
        <button
          type="button"
          aria-label="Dismiss navigation"
          className="lg:hidden fixed inset-0 z-30 bg-slate-950/50"
          onClick={() => setMobileNavOpen(false)}
        />
      )}

      <div className="flex-1 flex flex-col min-w-0">
        <header className="hidden lg:flex h-14 border-b border-slate-800 bg-slate-900/60 backdrop-blur px-3 sm:px-6 items-center justify-between gap-2 sm:gap-4 sticky top-0 z-30">
          <div className="flex items-center gap-2 sm:gap-3 min-w-0">
            <span className="text-sm font-semibold text-white truncate">{activeNavLabel}</span>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={() => void loadAllData()}
              disabled={refreshing}
              title="Reload library, queue, and settings from the server"
              aria-label="Reload library, queue, and settings from the server"
              className="inline-flex items-center justify-center gap-1.5 min-h-8 min-w-8 px-2.5 sm:px-3 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-xs text-slate-300 transition"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-indigo-400' : ''}`} />
              <span className="hidden sm:inline">Sync</span>
            </button>
          </div>
        </header>

        {toast && (
          <div
            className={`fixed top-[4.5rem] left-1/2 -translate-x-1/2 z-[60] max-w-[calc(100vw-1.5rem)] inline-flex items-start gap-2 px-3 py-2 rounded-lg text-xs font-medium border shadow-lg ${
              toast.isError
                ? 'bg-rose-950 border-rose-500/40 text-rose-200'
                : 'bg-emerald-950 border-emerald-500/40 text-emerald-200'
            }`}
          >
            {toast.isError ? (
              <AlertCircle className="w-3.5 h-3.5 shrink-0 mt-0.5" />
            ) : (
              <CheckCircle2 className="w-3.5 h-3.5 shrink-0 mt-0.5" />
            )}
            <span className="break-words select-text">{toast.message}</span>
          </div>
        )}

        {interactiveSearch && (
          <InteractiveSearchModal
            request={interactiveSearch}
            onClose={() => setInteractiveSearch(null)}
            onRefresh={loadAllData}
            onNotify={notify}
          />
        )}

        {confirmDialog}

        <main className="flex-1 p-4 sm:p-6 max-w-7xl w-full mx-auto">
          {activeView === 'dashboard' && (
            <DashboardTab
              overview={overview}
              profiles={profiles}
              rootFolders={rootFolders}
              onNavigate={(tab) => {
                if (tab === 'series' || tab === 'movies' || tab === 'music' || tab === 'books') {
                  navigateTo(`${tab}.library`);
                } else if (tab === 'settings') {
                  navigateTo('settings.media');
                } else if (tab === 'system') {
                  navigateTo('system.status');
                } else {
                  navigateTo(tab);
                }
              }}
              onAddSeries={() => handleOpenAddLibrary('series')}
              onAddMovie={() => handleOpenAddLibrary('movies')}
              onAddArtist={() => handleOpenAddLibrary('music')}
              onAddBook={() => handleOpenAddLibrary('books')}
              appUi={appConfig?.ui}
            />
          )}

          {activeView === 'calendar' && (
            <CalendarTab appUi={appConfig?.ui} onNavigate={navigateTo} onNotify={notify} />
          )}

          {renderMediaLibrary()}

          {media && mediaSub === 'import' && (
            <LibraryImportPage
              media={media}
              rootFolders={rootFolders}
              profiles={profiles}
              series={series}
              movies={movies}
              artists={artists}
              authors={authors}
              books={books}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {media && mediaSub === 'statistics' && (
            <StatisticsPage media={media} onNotify={notify} />
          )}

          {media && mediaSub === 'blocklist' && (
            <BlocklistPage media={media} appUi={appConfig?.ui} onNotify={notify} />
          )}

          {activeView === 'indexers' && (
            <IndexersSearchTab
              indexers={indexers}
              definitions={definitions}
              plugins={plugins}
              appConfig={appConfig}
              onSaveConfig={saveAppConfig}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {activeView === 'queue' && (
            <QueueClientsTab
              queue={queue}
              clients={clients}
              profiles={profiles}
              rootFolders={rootFolders}
              appConfig={appConfig}
              onSaveConfig={saveAppConfig}
              onRefresh={loadAllData}
              onRefreshQueue={refreshQueue}
              onNotify={notify}
            />
          )}

          {activeView === 'plugins' && (
            <PluginsTab
              plugins={plugins}
              masterApiKey={masterApiKey}
              appConfig={appConfig}
              onSaveConfig={saveAppConfig}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {activeView === 'users' && (
            <UsersRbacTab
              users={users}
              roles={roles}
              apiKeys={apiKeys}
              permissions={permissions}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {settingsPage && (
            <SettingsTab
              page={settingsPage}
              config={appConfig}
              overview={overview}
              indexers={indexers}
              profiles={profiles}
              rootFolders={rootFolders}
              onRefresh={loadAllData}
              onNotify={notify}
              onDirtyChange={setSettingsDirty}
              onNavigate={navigateTo}
              onSignedOut={() => {
                setUser(null);
                setMasterApiKey('');
              }}
              onConfigChange={(next) => {
                setAppConfig(next);
                applyUiPreferences(next.ui);
              }}
            />
          )}

          {activeView === 'system.status' && (
            <SystemStatusPage overview={overview} onNotify={notify} />
          )}
          {activeView === 'system.tasks' && (
            <SystemTasksPage appUi={appConfig?.ui} onNotify={notify} />
          )}
          {activeView === 'system.backup' && (
            <SystemBackupPage
              config={appConfig}
              onSave={saveAppConfig}
              onNotify={notify}
              appUi={appConfig?.ui}
            />
          )}
          {activeView === 'system.updates' && <SystemUpdatesPage onNotify={notify} />}
          {activeView === 'system.events' && (
            <SystemEventsPage appUi={appConfig?.ui} onNotify={notify} />
          )}
          {activeView === 'system.logs' && (
            <SystemLogsPage appUi={appConfig?.ui} onNotify={notify} />
          )}
          {activeView === 'system.changelog' && <SystemChangelogPage onNotify={notify} />}
        </main>
      </div>
    </div>
  );
};

export default App;
