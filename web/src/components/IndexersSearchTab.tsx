import React, { useEffect, useState } from 'react';
import {
  Radio,
  Search,
  Plus,
  Trash2,
  FileCode2,
  Globe,
  Lock,
  ShieldCheck,
  Activity,
  RefreshCw,
} from 'lucide-react';
import type {
  CardigannDefinition,
  DiscoveredPlugin,
  DomainClearance,
  FlareSolverrHealth,
  Indexer,
  IndexerProxy,
} from '../types';
import { api } from '../api';
import {
  blankIndexerForm,
  IndexerForm,
  IndexerModal,
  indexerToForm,
} from './modals/IndexerModal';
import {
  blankIndexerProxyForm,
  IndexerProxyForm,
  IndexerProxyModal,
  indexerProxyToForm,
} from './modals/IndexerProxyModal';

interface IndexersSearchTabProps {
  indexers: Indexer[];
  definitions: CardigannDefinition[];
  plugins: DiscoveredPlugin[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const IndexersSearchTab: React.FC<IndexersSearchTabProps> = ({
  indexers,
  definitions,
  plugins,
  onRefresh,
  onNotify,
}) => {
  const [subView, setSubView] = useState<'indexers' | 'cardigann' | 'flaresolverr'>('indexers');

  // Cardigann Definitions Filter
  const [defFilter, setDefFilter] = useState('');

  // FlareSolverr & Indexer Proxies State
  const [proxies, setProxies] = useState<IndexerProxy[]>([]);
  const [clearances, setClearances] = useState<DomainClearance[]>([]);
  const [testingHost, setTestingHost] = useState<string | null>(null);
  const [healthByHost, setHealthByHost] = useState<Record<string, FlareSolverrHealth>>({});
  const [indexerForm, setIndexerForm] = useState<IndexerForm | null>(null);
  const [proxyForm, setProxyForm] = useState<IndexerProxyForm | null>(null);

  // Interactive Solve Test State
  const [solveUrl, setSolveUrl] = useState('https://1337x.to/');
  const [solving, setSolving] = useState(false);
  const [solveResult, setSolveResult] = useState<Record<string, unknown> | null>(null);

  const loadProxiesAndClearances = async () => {
    try {
      const [pList, cList] = await Promise.all([
        api.getIndexerProxies(),
        api.getClearances(),
      ]);
      setProxies(pList);
      setClearances(cList);
    } catch {
      // Ignore initial load error
    }
  };

  useEffect(() => {
    void loadProxiesAndClearances();
  }, []);

  const handleDeleteIndexer = async (idx: Indexer) => {
    try {
      await api.deleteIndexer(idx.id);
      onNotify(`Removed indexer "${idx.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete indexer', true);
    }
  };

  const handleDeleteProxy = async (proxy: IndexerProxy) => {
    try {
      await api.deleteIndexerProxy(proxy.id);
      onNotify(`Removed indexer proxy "${proxy.name}"`);
      await loadProxiesAndClearances();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete indexer proxy', true);
    }
  };

  const handleTestProxy = async (host: string) => {
    setTestingHost(host);
    try {
      const health = await api.testIndexerProxy(host);
      setHealthByHost((prev) => ({ ...prev, [host]: health }));
      if (health.healthy) {
        onNotify(
          `FlareSolverr online (${health.version || 'v1'}) — ${health.latencyMs}ms`
        );
      } else {
        onNotify(health.message || `FlareSolverr unreachable at ${host}`, true);
      }
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'FlareSolverr test failed', true);
    } finally {
      setTestingHost(null);
    }
  };

  const handleSolveUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    setSolving(true);
    setSolveResult(null);
    try {
      const primaryHost =
        proxies.find((p) => p.enable && p.implementation === 'FlareSolverr')?.host ||
        'http://localhost:8191/';
      const res = await api.solveWithFlareSolverr({
        host: primaryHost,
        url: solveUrl,
        requestTimeout: 60,
        sessionTtlMinutes: 15,
      });
      setSolveResult(res);
      onNotify(`Solved Cloudflare challenge for ${solveUrl}`);
      await loadProxiesAndClearances();
    } catch (err) {
      onNotify(
        err instanceof Error ? err.message : 'FlareSolverr solve request failed',
        true
      );
    } finally {
      setSolving(false);
    }
  };

  const handleClearClearances = async () => {
    try {
      await api.clearClearances();
      setClearances([]);
      onNotify('Cleared cached Cloudflare clearance cookies & User-Agents');
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to clear cache', true);
    }
  };

  const handleSelectCardigannDef = (def: CardigannDefinition) => {
    setIndexerForm({
      ...blankIndexerForm(),
      name: def.name,
      implementation: 'Cardigann',
      protocol: 'torrent',
      privacy:
        def.type === 'private' ? 'private' : def.type === 'semi-private' ? 'semiPrivate' : 'public',
      baseUrl: def.links?.[0] || 'https://',
      definitionFile: def.fileName || def.id,
    });
  };

  const filteredDefs = definitions.filter(
    (d) =>
      d.name.toLowerCase().includes(defFilter.toLowerCase()) ||
      d.id.toLowerCase().includes(defFilter.toLowerCase()) ||
      (d.description || '').toLowerCase().includes(defFilter.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Top Header & Sub-Navigation */}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-amber-500/15 text-amber-400 border border-amber-500/30">
            <Radio className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-white">Indexers</h2>
            <p className="text-xs text-slate-400">
              {indexers.length} active indexers • {proxies.length} FlareSolverr/Indexer proxies •{' '}
              {definitions.length} YAML definitions
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex flex-wrap rounded-xl bg-slate-950 p-1 border border-slate-800">
            <button
              onClick={() => setSubView('indexers')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                subView === 'indexers'
                  ? 'bg-amber-500 text-slate-950 font-semibold'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              Indexers ({indexers.length})
            </button>
            <button
              onClick={() => setSubView('cardigann')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                subView === 'cardigann'
                  ? 'bg-amber-500 text-slate-950 font-semibold'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              YAML Definitions ({definitions.length})
            </button>
            <button
              onClick={() => {
                setSubView('flaresolverr');
                void loadProxiesAndClearances();
              }}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                subView === 'flaresolverr'
                  ? 'bg-amber-500 text-slate-950 font-semibold'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              <ShieldCheck className="w-3.5 h-3.5" />
              <span>FlareSolverr ({proxies.length})</span>
            </button>
          </div>

          <button
            type="button"
            onClick={() => setIndexerForm(blankIndexerForm())}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-amber-500 hover:bg-amber-400 text-slate-950 text-xs font-bold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Add Indexer</span>
          </button>
        </div>
      </div>

      {/* Configured Indexers */}
      {subView === 'indexers' && (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {indexers.map((idx) => (
            <div
              key={idx.id}
              className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col justify-between space-y-4"
            >
              <div>
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-bold text-white break-words">{idx.name}</h3>
                      <span
                        className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded ${
                          idx.protocol === 'usenet'
                            ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                            : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                        }`}
                      >
                        {idx.protocol}
                      </span>
                    </div>
                    <p className="text-xs text-slate-400 mt-1 font-mono truncate">
                      {idx.baseUrl || idx.definitionFile || idx.pluginId || 'Built-in'}
                    </p>
                  </div>

                  <span className="text-[11px] font-mono px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-300 shrink-0">
                    {idx.implementation}
                  </span>
                </div>

                <div className="mt-4 flex flex-wrap gap-2 text-[11px]">
                  <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-400">
                    Priority: <strong className="text-slate-200">{idx.priority}</strong>
                  </span>
                  <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-400">
                    Privacy: <strong className="text-slate-200">{idx.privacy}</strong>
                  </span>
                  {idx.definitionFile && (
                    <span className="px-2 py-0.5 rounded bg-amber-500/10 border border-amber-500/30 text-amber-300 font-mono">
                      YAML: {idx.definitionFile}
                    </span>
                  )}
                </div>
              </div>

              <div className="pt-3 border-t border-slate-800/80 flex items-center justify-between">
                <div className="flex items-center gap-2 text-xs text-emerald-400">
                  <span className="w-2 h-2 rounded-full bg-emerald-400" />
                  <span>RSS & Interactive Search Enabled</span>
                </div>
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setIndexerForm(indexerToForm(idx))}
                    className="px-2 py-1 rounded-lg text-xs text-slate-300 hover:text-white hover:bg-slate-800 transition"
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleDeleteIndexer(idx)}
                    className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                    title="Delete Indexer"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* SUBVIEW 3: Cardigann YAML Definitions Browser */}
      {subView === 'cardigann' && (
        <div className="space-y-4">
          <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div className="flex items-center gap-2.5">
              <FileCode2 className="w-5 h-5 text-amber-400" />
              <div>
                <h3 className="text-sm font-bold text-white">
                  Indexer YAML Definitions
                </h3>
                <p className="text-xs text-slate-400">
                  Click any definition to add and configure it as an active indexer
                </p>
              </div>
            </div>

            <div className="relative w-full sm:w-72">
              <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={defFilter}
                onChange={(e) => setDefFilter(e.target.value)}
                placeholder="Filter YAML definitions..."
                className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
              />
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            {filteredDefs.map((def) => (
              <div
                key={def.id}
                className="bg-slate-900/90 border border-slate-800 hover:border-amber-500/40 rounded-xl p-4 flex flex-col justify-between transition"
              >
                <div>
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0 flex flex-wrap items-center gap-2">
                      <h4 className="text-sm font-bold text-white break-words">{def.name}</h4>
                      <code className="text-[11px] font-mono text-amber-300 bg-amber-500/10 px-1.5 py-0.5 rounded break-all">
                        {def.id}.yml
                      </code>
                    </div>
                    <span
                      className={`inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded shrink-0 ${
                        def.type === 'private'
                          ? 'bg-rose-500/15 text-rose-300 border border-rose-500/30'
                          : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                      }`}
                    >
                      {def.type === 'private' ? (
                        <Lock className="w-2.5 h-2.5" />
                      ) : (
                        <Globe className="w-2.5 h-2.5" />
                      )}
                      {def.type || 'public'}
                    </span>
                  </div>

                  <p className="text-xs text-slate-400 mt-2 line-clamp-2">
                    {def.description || `Cardigann YAML definition for ${def.name}`}
                  </p>

                  {def.links && def.links.length > 0 && (
                    <div className="mt-2.5 text-[11px] font-mono text-slate-500 truncate">
                      {def.links[0]}
                    </div>
                  )}
                </div>

                <div className="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between">
                  <span className="text-[11px] text-slate-500 font-mono">
                    {def.language || 'en-US'}
                  </span>
                  <button
                    onClick={() => handleSelectCardigannDef(def)}
                    className="px-3 py-1.5 rounded-lg bg-amber-500/15 hover:bg-amber-500 text-amber-300 hover:text-slate-950 border border-amber-500/30 text-xs font-semibold transition"
                  >
                    + Use Definition
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* SUBVIEW 4: FlareSolverr & Indexer Proxies */}
      {subView === 'flaresolverr' && (
        <div className="space-y-6">
          <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-start gap-3">
              <div className="p-2.5 rounded-xl bg-orange-500/15 text-orange-400 border border-orange-500/30">
                <ShieldCheck className="w-5 h-5" />
              </div>
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-bold text-white">
                    FlareSolverr Cloudflare & DDoS-Guard Bypass
                  </h3>
                  <span className="text-[11px] font-mono px-2 py-0.5 rounded bg-orange-500/15 text-orange-300 border border-orange-500/30">
                    POST /v1 (request.get / sessions.*)
                  </span>
                </div>
                <p className="text-xs text-slate-400 mt-1">
                  Automatically detects Cloudflare IUAM, Turnstile, and DDoS-Guard 403/503 challenges,
                  solves them via headless Chromium, and caches <code className="text-amber-300">cf_clearance</code> cookies + User-Agent per tracker domain.
                </p>
              </div>
            </div>

            <button
              type="button"
              onClick={() => setProxyForm(blankIndexerProxyForm())}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-orange-500 hover:bg-orange-400 text-slate-950 text-xs font-bold rounded-lg transition shrink-0"
            >
              <Plus className="w-4 h-4" />
              <span>Add Indexer Proxy</span>
            </button>
          </div>

          {/* Configured Proxies Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {proxies.map((proxy) => {
              const health = healthByHost[proxy.host];
              return (
                <div
                  key={proxy.id}
                  className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col justify-between space-y-4"
                >
                  <div>
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <h4 className="text-base font-bold text-white break-words">{proxy.name}</h4>
                          <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-orange-500/15 text-orange-300 border border-orange-500/30">
                            {proxy.implementation}
                          </span>
                        </div>
                        <p className="text-xs text-slate-400 font-mono mt-1 break-all">{proxy.host}</p>
                      </div>

                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          type="button"
                          onClick={() => setProxyForm(indexerProxyToForm(proxy))}
                          className="px-2 py-1 rounded-lg text-xs text-slate-300 hover:text-white hover:bg-slate-800 transition"
                        >
                          Edit
                        </button>
                        <button
                          type="button"
                          onClick={() => void handleDeleteProxy(proxy)}
                          className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                          title="Delete Indexer Proxy"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    </div>

                    <div className="mt-4 flex flex-wrap gap-2 text-[11px]">
                      <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-400">
                        Max Timeout:{' '}
                        <strong className="text-slate-200">{proxy.requestTimeout}s</strong>
                      </span>
                      <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-400">
                        Session TTL:{' '}
                        <strong className="text-slate-200">{proxy.sessionTtlMinutes || 15}m</strong>
                      </span>
                      {proxy.disableMedia && (
                        <span className="px-2 py-0.5 rounded bg-indigo-500/15 border border-indigo-500/30 text-indigo-300">
                          Media Blocked (Fast Mode)
                        </span>
                      )}
                    </div>

                    {health && (
                      <div
                        className={`mt-3 p-3 rounded-lg border text-xs ${
                          health.healthy
                            ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                            : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                        }`}
                      >
                        <div className="font-semibold flex items-center justify-between">
                          <span>
                            {health.healthy ? 'Connected to FlareSolverr' : 'Connection Failed'}
                          </span>
                          <span className="font-mono">{health.latencyMs} ms</span>
                        </div>
                        <p className="text-[11px] opacity-90 mt-1 font-mono break-all">
                          {health.message}
                          {health.version ? ` • v${health.version}` : ''}
                        </p>
                      </div>
                    )}
                  </div>

                  <div className="pt-3 border-t border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                    <span className="text-[11px] text-slate-400 font-mono break-all min-w-0">
                      Prowlarr Compat: /prowlarr/api/v1/indexerproxy/{proxy.id}
                    </span>
                    <button
                      onClick={() => handleTestProxy(proxy.host)}
                      disabled={testingHost === proxy.host}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold transition"
                    >
                      <Activity className="w-3.5 h-3.5 text-orange-400" />
                      <span>
                        {testingHost === proxy.host ? 'Testing...' : 'Test FlareSolverr'}
                      </span>
                    </button>
                  </div>
                </div>
              );
            })}
          </div>

          {/* Interactive Cloudflare Solver & Clearance Cache */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
              <div>
                <h4 className="text-sm font-bold text-white">
                  Interactive FlareSolverr Challenge Solver
                </h4>
                <p className="text-xs text-slate-400 mt-0.5">
                  Send a test <code className="text-amber-300">request.get</code> command through
                  FlareSolverr and cache the resulting clearance cookies
                </p>
              </div>

              <form onSubmit={handleSolveUrl} className="flex flex-col min-[420px]:flex-row gap-2">
                <input
                  type="url"
                  required
                  value={solveUrl}
                  onChange={(e) => setSolveUrl(e.target.value)}
                  placeholder="https://1337x.to/"
                  className="flex-1 min-w-0 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                />
                <button
                  type="submit"
                  disabled={solving}
                  className="px-4 py-2 bg-orange-500 hover:bg-orange-400 disabled:opacity-50 text-slate-950 text-xs font-bold rounded-lg transition shrink-0"
                >
                  {solving ? 'Solving...' : 'Solve URL'}
                </button>
              </form>

              {solveResult && (
                <pre className="p-3 rounded-lg bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 max-h-48 overflow-auto">
                  {JSON.stringify(solveResult, null, 2)}
                </pre>
              )}
            </div>

            <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                <div className="min-w-0">
                  <h4 className="text-sm font-bold text-white">
                    Cached Cloudflare Clearances ({clearances.length})
                  </h4>
                  <p className="text-xs text-slate-400 mt-0.5">
                    Per-domain <code className="text-amber-300">cf_clearance</code> cookies and
                    matching browser User-Agents
                  </p>
                </div>
                {clearances.length > 0 && (
                  <button
                    onClick={handleClearClearances}
                    className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-300 hover:text-rose-300 text-xs transition"
                  >
                    <RefreshCw className="w-3 h-3" />
                    <span>Flush Cache</span>
                  </button>
                )}
              </div>

              {clearances.length === 0 ? (
                <div className="py-6 text-center text-xs text-slate-500 bg-slate-950/50 rounded-lg border border-slate-800/60">
                  No domain clearances cached yet. Clearances are populated automatically whenever an
                  indexer encounters a Cloudflare or DDoS-Guard challenge.
                </div>
              ) : (
                <div className="space-y-2">
                  {clearances.map((cl) => (
                    <div
                      key={cl.domain}
                      className="p-3 rounded-lg bg-slate-950 border border-slate-800 text-xs space-y-1"
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-mono font-bold text-emerald-400">{cl.domain}</span>
                        <span className="text-[11px] text-slate-400 font-mono">
                          {cl.cookies?.length || 0} cookies
                        </span>
                      </div>
                      <p className="text-[11px] text-slate-400 font-mono truncate">
                        UA: {cl.userAgent}
                      </p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {indexerForm ? (
        <IndexerModal
          initial={indexerForm}
          definitions={definitions}
          plugins={plugins}
          onClose={() => setIndexerForm(null)}
          onSaved={onRefresh}
          onNotify={onNotify}
        />
      ) : null}

      {proxyForm ? (
        <IndexerProxyModal
          initial={proxyForm}
          onClose={() => setProxyForm(null)}
          onSaved={loadProxiesAndClearances}
          onNotify={onNotify}
        />
      ) : null}
    </div>
  );
};
