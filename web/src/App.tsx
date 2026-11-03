import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  LayoutDashboard,
  Tv,
  Film,
  Radio,
  DownloadCloud,
  Puzzle,
  Users,
  Settings,
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
  CardigannDefinition,
  DiscoveredPlugin,
  DownloadClient,
  Indexer,
  Movie,
  NavTab,
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
import { LoginView } from './components/LoginView';
import { DashboardTab } from './components/DashboardTab';
import { SeriesTab } from './components/SeriesTab';
import { MoviesTab } from './components/MoviesTab';
import { IndexersSearchTab } from './components/IndexersSearchTab';
import { InteractiveSearchModal } from './components/InteractiveSearchModal';
import { SetupWizard } from './components/SetupWizard';
import { QueueClientsTab } from './components/QueueClientsTab';
import { PluginsTab } from './components/PluginsTab';
import { UsersRbacTab } from './components/UsersRbacTab';
import { SettingsTab } from './components/settings/SettingsTab';
import { useGamepad } from './htpc/useGamepad';
import { useRovingFocus } from './htpc/useRovingFocus';

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>(null);
  const [masterApiKey, setMasterApiKey] = useState<string>('');
  const [authChecking, setAuthChecking] = useState<boolean>(true);
  const [activeTab, setActiveTab] = useState<NavTab>('dashboard');
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  // Unified Data State
  const [overview, setOverview] = useState<OverviewResponse | null>(null);
  const [series, setSeries] = useState<Series[]>([]);
  const [movies, setMovies] = useState<Movie[]>([]);
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

  // Toast Notification
  const [toast, setToast] = useState<{ message: string; isError?: boolean } | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const notify = useCallback((message: string, isError = false) => {
    setToast({ message, isError });
  }, []);

  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(null), 4000);
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
        };
        setAppConfig(normalized);
        applyUiPreferences(normalized.ui);
      }
    } finally {
      setRefreshing(false);
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

  const handleLogin = async (username: string, password: string) => {
    const res = await api.login(username, password);
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
    if (!(req.seriesId || req.movieId)) {
      notify('Interactive Search requires a library series or movie', true);
      return;
    }
    setInteractiveSearch(req);
  };

  const handleOpenAddLibrary = (tab: 'series' | 'movies') => {
    setPendingOpenAdd(true);
    setAddPanelNonce((n) => n + 1);
    setActiveTab(tab);
  };

  const saveAppConfig = useCallback(
    async (next: AppConfig): Promise<AppConfig> => {
      const saved = await api.saveConfig(next);
      setAppConfig(saved);
      applyUiPreferences(saved.ui);
      return saved;
    },
    []
  );

  const navItems: Array<{
    id: NavTab;
    label: string;
    badge?: number | string;
    accent: string;
    icon: React.ComponentType<{ className?: string }>;
  }> = [
    {
      id: 'dashboard',
      label: 'Dashboard',
      accent: 'text-indigo-400',
      icon: LayoutDashboard,
    },
    {
      id: 'series',
      label: 'TV Series',
      badge: series.length,
      accent: 'text-indigo-400',
      icon: Tv,
    },
    {
      id: 'movies',
      label: 'Movies',
      badge: movies.length,
      accent: 'text-emerald-400',
      icon: Film,
    },
    {
      id: 'indexers',
      label: 'Indexers',
      badge: indexers.length,
      accent: 'text-amber-400',
      icon: Radio,
    },
    {
      id: 'queue',
      label: 'Queue & Clients',
      badge: queue.length,
      accent: 'text-sky-400',
      icon: DownloadCloud,
    },
    {
      id: 'plugins',
      label: 'Go Plugins',
      badge: plugins.length,
      accent: 'text-purple-400',
      icon: Puzzle,
    },
    {
      id: 'users',
      label: 'Users & RBAC',
      badge: users.length,
      accent: 'text-emerald-400',
      icon: Users,
    },
    {
      id: 'settings',
      label: 'Settings',
      accent: 'text-indigo-300',
      icon: Settings,
    },
  ];

  const navRoving = useRovingFocus({
    itemCount: navItems.length,
    orientation: 'vertical',
    onActivate: (index) => setActiveTab(navItems[index]?.id ?? 'dashboard'),
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
        if (activeTab !== 'dashboard') {
          setActiveTab('dashboard');
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

  const activeNavLabel = navItems.find((n) => n.id === activeTab)?.label ?? 'Dashboard';

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col lg:flex-row">
      {/* Sidebar Navigation */}
      <aside className="w-full lg:w-64 bg-slate-900/95 border-b lg:border-b-0 lg:border-r border-slate-800 shrink-0 flex flex-col justify-between relative z-40 sticky top-0 lg:static">
        <div>
          {/* Brand Header */}
          <div className="h-14 px-4 lg:h-auto lg:p-5 border-b border-slate-800 flex items-center gap-3">
            <div className="flex items-center gap-3 min-w-0">
              <img
                src="/icon-192.png"
                alt="Replacearr"
                className="w-8 h-8 lg:w-9 lg:h-9 rounded-xl shrink-0"
              />
              <div className="min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="lg:hidden font-semibold text-sm text-white truncate">
                    {activeNavLabel}
                  </span>
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

          {/* Navigation Links */}
          <nav
            id="app-primary-nav"
            ref={navRef}
            className={`${
              mobileNavOpen ? 'flex' : 'hidden'
            } lg:flex absolute lg:static inset-x-0 top-14 lg:top-auto p-3 flex-col gap-1 bg-slate-900 border-b border-slate-800 lg:border-b-0 shadow-xl lg:shadow-none max-h-[calc(100dvh-3.5rem)] overflow-y-auto`}
          >
            {navItems.map((item, index) => {
              const Icon = item.icon;
              const isActive = activeTab === item.id;
              const roving = navRoving.itemProps(index);
              return (
                <button
                  key={item.id}
                  type="button"
                  {...roving}
                  onClick={() => {
                    setActiveTab(item.id);
                    setMobileNavOpen(false);
                  }}
                  className={`flex items-center justify-between gap-2.5 px-3 py-2.5 rounded-xl text-xs font-medium whitespace-nowrap transition w-full ${
                    isActive
                      ? 'bg-indigo-600/15 text-white border border-indigo-500/30 shadow-sm'
                      : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60 border border-transparent'
                  }`}
                >
                  <div className="flex items-center gap-2.5">
                    <Icon className={`w-4 h-4 ${isActive ? item.accent : 'text-slate-400'}`} />
                    <span>{item.label}</span>
                  </div>
                  {item.badge !== undefined && (
                    <span
                      className={`text-[10px] font-mono px-1.5 py-0.5 rounded-md ${
                        isActive
                          ? 'bg-indigo-500/25 text-indigo-200'
                          : 'bg-slate-950 text-slate-400 border border-slate-800'
                      }`}
                    >
                      {item.badge}
                    </span>
                  )}
                </button>
              );
            })}

            <div className="lg:hidden mt-2 pt-3 border-t border-slate-800 space-y-2 px-1">
              <button
                type="button"
                onClick={() => {
                  void loadAllData();
                  setMobileNavOpen(false);
                }}
                disabled={refreshing}
                title="Reload library, queue, and settings from the server"
                aria-label="Reload library, queue, and settings from the server"
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
          </nav>
        </div>

        {/* Sidebar Footer: User Info */}
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

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Bar (desktop only — mobile uses the compact nav header) */}
        <header className="hidden lg:flex h-14 border-b border-slate-800 bg-slate-900/60 backdrop-blur px-3 sm:px-6 items-center justify-between gap-2 sm:gap-4 sticky top-0 z-30">
          <div className="flex items-center gap-2 sm:gap-3 min-w-0">
            <span className="text-sm font-semibold text-white capitalize truncate">
              {activeNavLabel}
            </span>
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
            className={`fixed top-[4.5rem] left-1/2 -translate-x-1/2 z-40 max-w-[calc(100vw-1.5rem)] inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium border shadow-lg ${
              toast.isError
                ? 'bg-rose-950 border-rose-500/40 text-rose-200'
                : 'bg-emerald-950 border-emerald-500/40 text-emerald-200'
            }`}
          >
            {toast.isError ? (
              <AlertCircle className="w-3.5 h-3.5 shrink-0" />
            ) : (
              <CheckCircle2 className="w-3.5 h-3.5 shrink-0" />
            )}
            <span className="break-words">{toast.message}</span>
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

        {/* Active View Container */}
        <main className="flex-1 p-4 sm:p-6 max-w-7xl w-full mx-auto">
          {activeTab === 'dashboard' && (
            <DashboardTab
              overview={overview}
              profiles={profiles}
              rootFolders={rootFolders}
              onNavigate={setActiveTab}
              onAddSeries={() => handleOpenAddLibrary('series')}
              onAddMovie={() => handleOpenAddLibrary('movies')}
              appUi={appConfig?.ui}
            />
          )}

          {activeTab === 'series' && (
            <SeriesTab
              series={series}
              profiles={profiles}
              rootFolders={rootFolders}
              onRefresh={loadAllData}
              onSearchReleases={handleInteractiveSearch}
              onNotify={notify}
              appUi={appConfig?.ui}
              openAddPanel={pendingOpenAdd && activeTab === 'series'}
              openAddNonce={addPanelNonce}
              onAddPanelOpened={() => setPendingOpenAdd(false)}
            />
          )}

          {activeTab === 'movies' && (
            <MoviesTab
              movies={movies}
              profiles={profiles}
              rootFolders={rootFolders}
              onRefresh={loadAllData}
              onSearchReleases={handleInteractiveSearch}
              onNotify={notify}
              openAddPanel={pendingOpenAdd && activeTab === 'movies'}
              openAddNonce={addPanelNonce}
              onAddPanelOpened={() => setPendingOpenAdd(false)}
            />
          )}

          {activeTab === 'indexers' && (
            <IndexersSearchTab
              indexers={indexers}
              definitions={definitions}
              plugins={plugins}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {activeTab === 'queue' && (
            <QueueClientsTab
              queue={queue}
              clients={clients}
              profiles={profiles}
              rootFolders={rootFolders}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {activeTab === 'plugins' && (
            <PluginsTab
              plugins={plugins}
              masterApiKey={masterApiKey}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {activeTab === 'users' && (
            <UsersRbacTab
              users={users}
              roles={roles}
              apiKeys={apiKeys}
              permissions={permissions}
              onRefresh={loadAllData}
              onNotify={notify}
            />
          )}

          {activeTab === 'settings' && (
            <SettingsTab
              config={appConfig}
              overview={overview}
              indexers={indexers}
              definitions={definitions}
              plugins={plugins}
              clients={clients}
              profiles={profiles}
              rootFolders={rootFolders}
              users={users}
              roles={roles}
              apiKeys={apiKeys}
              permissions={permissions}
              onRefresh={loadAllData}
              onNotify={notify}
              onConfigChange={(next) => {
                setAppConfig(next);
                applyUiPreferences(next.ui);
              }}
            />
          )}
        </main>
      </div>
    </div>
  );
};

export default App;
