import React, { useCallback, useEffect, useState } from 'react';
import {
  LayoutDashboard,
  Tv,
  Film,
  Radio,
  DownloadCloud,
  Puzzle,
  Users,
  Link2,
  Settings,
  Layers,
  LogOut,
  RefreshCw,
  Shield,
  Database,
  CheckCircle2,
  AlertCircle,
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
  Role,
  RootFolder,
  Series,
  User,
  AppConfig,
} from './types';
import { api, getStoredToken, setStoredToken } from './api';
import { applyUiPreferences } from './uiPrefs';
import { LoginView } from './components/LoginView';
import { DashboardTab } from './components/DashboardTab';
import { SeriesTab } from './components/SeriesTab';
import { MoviesTab } from './components/MoviesTab';
import { IndexersSearchTab } from './components/IndexersSearchTab';
import { QueueClientsTab } from './components/QueueClientsTab';
import { PluginsTab } from './components/PluginsTab';
import { UsersRbacTab } from './components/UsersRbacTab';
import { CompatHubTab } from './components/CompatHubTab';
import { SettingsTab } from './components/settings/SettingsTab';

export const App: React.FC = () => {
  const [user, setUser] = useState<User | null>(null);
  const [masterApiKey, setMasterApiKey] = useState<string>('');
  const [authChecking, setAuthChecking] = useState<boolean>(true);
  const [activeTab, setActiveTab] = useState<NavTab>('dashboard');

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

  // Cross-tab search state
  const [pendingSearchQuery, setPendingSearchQuery] = useState<string>('');
  const [pendingSearchType, setPendingSearchType] = useState<string>('search');

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
        setAppConfig(cfg);
        applyUiPreferences(cfg.ui);
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

  const handleTriggerSearch = (query: string, type = 'search') => {
    setPendingSearchQuery(query);
    setPendingSearchType(type);
    setActiveTab('indexers');
  };

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
      label: 'TV Series (Sonarr)',
      badge: series.length,
      accent: 'text-indigo-400',
      icon: Tv,
    },
    {
      id: 'movies',
      label: 'Movies (Radarr)',
      badge: movies.length,
      accent: 'text-emerald-400',
      icon: Film,
    },
    {
      id: 'indexers',
      label: 'Indexers & Search',
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
    {
      id: 'compatibility',
      label: 'Compatibility Hub',
      badge: 'v3/v1',
      accent: 'text-amber-400',
      icon: Link2,
    },
  ];

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col lg:flex-row">
      {/* Sidebar Navigation */}
      <aside className="w-full lg:w-64 bg-slate-900/95 border-b lg:border-b-0 lg:border-r border-slate-800 shrink-0 flex flex-col justify-between">
        <div>
          {/* Brand Header */}
          <div className="p-5 border-b border-slate-800 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-indigo-500 via-emerald-500 to-amber-500 p-[1px]">
                <div className="w-full h-full bg-slate-950 rounded-[11px] flex items-center justify-center">
                  <Layers className="w-5 h-5 text-indigo-400" />
                </div>
              </div>
              <div>
                <div className="flex items-center gap-1.5">
                  <span className="font-bold text-base text-white tracking-tight">
                    Replacearr
                  </span>
                  <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-indigo-500/20 text-indigo-300">
                    Go
                  </span>
                </div>
                <p className="text-[11px] text-slate-400">
                  Sonarr • Radarr • Prowlarr
                </p>
              </div>
            </div>
          </div>

          {/* Navigation Links */}
          <nav className="p-3 flex lg:flex-col gap-1 overflow-x-auto">
            {navItems.map((item) => {
              const Icon = item.icon;
              const isActive = activeTab === item.id;
              return (
                <button
                  key={item.id}
                  onClick={() => setActiveTab(item.id)}
                  className={`flex items-center justify-between gap-2.5 px-3 py-2.5 rounded-xl text-xs font-medium whitespace-nowrap transition w-auto shrink-0 lg:w-full lg:shrink ${
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
          </nav>
        </div>

        {/* Sidebar Footer: System & User Info */}
        <div className="hidden lg:block p-4 border-t border-slate-800 space-y-3 bg-slate-950/40">
          <div className="p-3 rounded-xl bg-slate-900 border border-slate-800 space-y-1.5 text-xs">
            <div className="flex items-center justify-between text-slate-400">
              <span className="inline-flex items-center gap-1.5">
                <Database className="w-3.5 h-3.5 text-emerald-400" />
                SQLite WAL
              </span>
              <span className="text-[11px] font-mono text-emerald-400">ONLINE</span>
            </div>
            <div className="text-[11px] font-mono text-slate-500 truncate">
              v{overview?.system?.version || '1.0.0'} • Zero-CGO
            </div>
          </div>

          <div className="flex items-center justify-between gap-2 pt-1">
            <div className="min-w-0">
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-bold text-white truncate">{user.username}</span>
                <span className="inline-flex items-center gap-0.5 text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                  <Shield className="w-2.5 h-2.5" />
                  {user.roleName}
                </span>
              </div>
              <p className="text-[11px] text-slate-500 truncate">
                {user.email || `${user.username}@replacearr.local`}
              </p>
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

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0">
        {/* Top Bar */}
        <header className="h-14 border-b border-slate-800 bg-slate-900/60 backdrop-blur px-3 sm:px-6 flex items-center justify-between gap-2 sm:gap-4 sticky top-0 z-30">
          <div className="flex items-center gap-2 sm:gap-3 min-w-0">
            <span className="text-sm font-semibold text-white capitalize truncate">
              {navItems.find((n) => n.id === activeTab)?.label}
            </span>
            <span className="hidden lg:inline-flex shrink-0 items-center gap-1.5 text-[11px] font-mono px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-300 border border-emerald-500/20">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
              Sonarr / Radarr / Prowlarr Shims Active
            </span>
          </div>

          <div className="flex items-center gap-2 shrink-0">
            <button
              onClick={() => void loadAllData()}
              disabled={refreshing}
              className="inline-flex items-center justify-center gap-1.5 min-h-8 min-w-8 px-2.5 sm:px-3 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-xs text-slate-300 transition"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-indigo-400' : ''}`} />
              <span className="hidden sm:inline">Sync</span>
            </button>

            <button
              onClick={handleLogout}
              className="lg:hidden inline-flex items-center justify-center min-h-8 min-w-8 px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-300"
              title="Sign Out"
            >
              <LogOut className="w-3.5 h-3.5" />
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

        {/* Active View Container */}
        <main className="flex-1 p-4 sm:p-6 max-w-7xl w-full mx-auto">
          {activeTab === 'dashboard' && (
            <DashboardTab
              overview={overview}
              profiles={profiles}
              rootFolders={rootFolders}
              onNavigate={setActiveTab}
              onQuickSearch={handleTriggerSearch}
              appUi={appConfig?.ui}
            />
          )}

          {activeTab === 'series' && (
            <SeriesTab
              series={series}
              profiles={profiles}
              rootFolders={rootFolders}
              onRefresh={loadAllData}
              onSearchReleases={handleTriggerSearch}
              onNotify={notify}
              appUi={appConfig?.ui}
            />
          )}

          {activeTab === 'movies' && (
            <MoviesTab
              movies={movies}
              profiles={profiles}
              rootFolders={rootFolders}
              onRefresh={loadAllData}
              onSearchReleases={handleTriggerSearch}
              onNotify={notify}
            />
          )}

          {activeTab === 'indexers' && (
            <IndexersSearchTab
              indexers={indexers}
              definitions={definitions}
              plugins={plugins}
              initialSearchQuery={pendingSearchQuery}
              initialSearchType={pendingSearchType}
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

          {activeTab === 'compatibility' && (
            <CompatHubTab masterApiKey={masterApiKey} />
          )}
        </main>
      </div>
    </div>
  );
};

export default App;
