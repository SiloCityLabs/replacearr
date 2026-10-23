import React, { useEffect, useState } from 'react';
import {
  Radio,
  Search,
  Plus,
  Trash2,
  Download,
  FileCode2,
  Globe,
  Lock,
  Sparkles,
  X,
  ArrowUpDown,
  Puzzle,
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
  ReleaseResult,
} from '../types';
import { api, formatBytes } from '../api';

interface IndexersSearchTabProps {
  indexers: Indexer[];
  definitions: CardigannDefinition[];
  plugins: DiscoveredPlugin[];
  initialSearchQuery?: string;
  initialSearchType?: string;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const IndexersSearchTab: React.FC<IndexersSearchTabProps> = ({
  indexers,
  definitions,
  plugins,
  initialSearchQuery = '',
  initialSearchType = 'search',
  onRefresh,
  onNotify,
}) => {
  const [subView, setSubView] = useState<'search' | 'indexers' | 'cardigann' | 'flaresolverr'>(
    'search'
  );

  // Unified Search State
  const [query, setQuery] = useState(initialSearchQuery || 'Severance');
  const [searchType, setSearchType] = useState(initialSearchType || 'search');
  const [results, setResults] = useState<ReleaseResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [grabbingGuid, setGrabbingGuid] = useState<string | null>(null);

  // Cardigann Definitions Filter
  const [defFilter, setDefFilter] = useState('');

  // FlareSolverr & Indexer Proxies State
  const [proxies, setProxies] = useState<IndexerProxy[]>([]);
  const [clearances, setClearances] = useState<DomainClearance[]>([]);
  const [testingHost, setTestingHost] = useState<string | null>(null);
  const [healthByHost, setHealthByHost] = useState<Record<string, FlareSolverrHealth>>({});
  const [showAddProxyModal, setShowAddProxyModal] = useState(false);
  const [proxyName, setProxyName] = useState('FlareSolverr');
  const [proxyImpl, setProxyImpl] = useState<'FlareSolverr' | 'Http' | 'Socks5'>('FlareSolverr');
  const [proxyHost, setProxyHost] = useState('http://localhost:8191/');
  const [proxyTimeout, setProxyTimeout] = useState(60);
  const [proxySessionTtl, setProxySessionTtl] = useState(15);
  const [proxyDisableMedia, setProxyDisableMedia] = useState(false);

  // Interactive Solve Test State
  const [solveUrl, setSolveUrl] = useState('https://1337x.to/');
  const [solving, setSolving] = useState(false);
  const [solveResult, setSolveResult] = useState<Record<string, unknown> | null>(null);

  // Add Indexer Modal State
  const [showAddModal, setShowAddModal] = useState(false);
  const [idxName, setIdxName] = useState('');
  const [idxImplementation, setIdxImplementation] = useState('Torznab');
  const [idxProtocol, setIdxProtocol] = useState<'torrent' | 'usenet'>('torrent');
  const [idxPrivacy, setIdxPrivacy] = useState<'public' | 'private' | 'semiPrivate'>('public');
  const [idxBaseUrl, setIdxBaseUrl] = useState('https://1337x.to/');
  const [idxApiKey, setIdxApiKey] = useState('');
  const [idxDefFile, setIdxDefFile] = useState('');
  const [idxPluginId, setIdxPluginId] = useState('');
  const [idxPriority, setIdxPriority] = useState(25);

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

  const runSearch = async (q: string, t: string) => {
    setSearching(true);
    try {
      const res = await api.searchReleases(q, t);
      setResults(res);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Release search failed', true);
    } finally {
      setSearching(false);
    }
  };

  useEffect(() => {
    void loadProxiesAndClearances();
    if (initialSearchQuery) {
      setQuery(initialSearchQuery);
      setSearchType(initialSearchType || 'search');
      setSubView('search');
      void runSearch(initialSearchQuery, initialSearchType || 'search');
    } else if (results.length === 0) {
      void runSearch(query, searchType);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialSearchQuery, initialSearchType]);

  const handleGrab = async (rel: ReleaseResult) => {
    setGrabbingGuid(rel.guid || rel.title);
    try {
      await api.grabRelease({
        title: rel.title,
        downloadUrl: rel.downloadUrl,
        protocol: rel.protocol || 'torrent',
        indexer: rel.indexer || 'Replacearr',
        size: rel.size || 1500000000,
        mediaType: searchType === 'movie' ? 'movie' : 'tv',
      });
      onNotify(`Grabbed "${rel.title}" and sent to Download Queue!`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to grab release', true);
    } finally {
      setGrabbingGuid(null);
    }
  };

  const handleCreateIndexer = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const created = await api.addIndexer({
        name: idxName,
        implementation: idxImplementation,
        implementationName: idxImplementation,
        configContract: `${idxImplementation}Settings`,
        protocol: idxProtocol,
        privacy: idxPrivacy,
        enable: true,
        enableRss: true,
        enableAutomaticSearch: true,
        enableInteractiveSearch: true,
        priority: idxPriority,
        baseUrl: idxBaseUrl,
        apiKey: idxApiKey,
        definitionFile: idxDefFile,
        pluginId: idxPluginId,
        categories: [2000, 5000],
      });
      onNotify(`Added indexer "${created.name}" (${created.implementation})`);
      setShowAddModal(false);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to create indexer', true);
    }
  };

  const handleDeleteIndexer = async (idx: Indexer) => {
    try {
      await api.deleteIndexer(idx.id);
      onNotify(`Removed indexer "${idx.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete indexer', true);
    }
  };

  const handleCreateProxy = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const created = await api.addIndexerProxy({
        name: proxyName,
        implementation: proxyImpl,
        implementationName: proxyImpl,
        configContract: `${proxyImpl}Settings`,
        host: proxyHost,
        requestTimeout: proxyTimeout,
        sessionTtlMinutes: proxySessionTtl,
        disableMedia: proxyDisableMedia,
        enable: true,
      });
      onNotify(`Added indexer proxy "${created.name}" (${created.host})`);
      setShowAddProxyModal(false);
      await loadProxiesAndClearances();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add indexer proxy', true);
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
    setIdxName(def.name);
    setIdxImplementation('Cardigann');
    setIdxProtocol('torrent');
    setIdxPrivacy(
      def.type === 'private' ? 'private' : def.type === 'semi-private' ? 'semiPrivate' : 'public'
    );
    setIdxBaseUrl(def.links?.[0] || 'https://');
    setIdxDefFile(def.id);
    setIdxPluginId('');
    setShowAddModal(true);
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
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-bold text-white">Indexers & Unified Search</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30 font-mono">
                Prowlarr v1 + FlareSolverr
              </span>
            </div>
            <p className="text-xs text-slate-400">
              {indexers.length} active indexers • {proxies.length} FlareSolverr/Indexer proxies •{' '}
              {definitions.length} Cardigann YAML definitions
            </p>
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex flex-wrap rounded-xl bg-slate-950 p-1 border border-slate-800">
            <button
              onClick={() => setSubView('search')}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium transition ${
                subView === 'search'
                  ? 'bg-amber-500 text-slate-950 font-semibold'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              Unified Search ({results.length})
            </button>
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
              Cardigann YAML ({definitions.length})
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
            onClick={() => {
              setIdxName('');
              setIdxImplementation('Torznab');
              setIdxBaseUrl('https://indexer.example.com/api');
              setIdxDefFile('');
              setShowAddModal(true);
            }}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-amber-500 hover:bg-amber-400 text-slate-950 text-xs font-bold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Add Indexer</span>
          </button>
        </div>
      </div>

      {/* SUBVIEW 1: Interactive Unified Release Search */}
      {subView === 'search' && (
        <div className="space-y-4">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void runSearch(query, searchType);
            }}
            className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 flex flex-col sm:flex-row gap-3"
          >
            <div className="relative flex-1">
              <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Query across Torznab, Newznab, Cardigann YAML & Go Plugins..."
                className="w-full pl-10 pr-4 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white placeholder-slate-500 focus:outline-none focus:border-amber-500"
              />
            </div>

            <select
              value={searchType}
              onChange={(e) => setSearchType(e.target.value)}
              className="px-3.5 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-200 focus:outline-none focus:border-amber-500"
            >
              <option value="search">All Categories (search)</option>
              <option value="tvsearch">TV Episodes / Packs (tvsearch)</option>
              <option value="movie">Movies (movie)</option>
            </select>

            <button
              type="submit"
              disabled={searching}
              className="px-5 py-2.5 bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-slate-950 font-bold text-xs rounded-lg transition shrink-0"
            >
              {searching ? 'Querying Indexers...' : 'Search Indexers'}
            </button>
          </form>

          {/* Results Table */}
          <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
            <div className="px-4 py-3 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs text-slate-400 min-w-0">
              <span className="inline-flex items-center gap-1.5 shrink-0">
                <ArrowUpDown className="w-3.5 h-3.5 text-amber-400" />
                Showing {results.length} federated release results
              </span>
              <span className="font-mono text-[11px] truncate min-w-0">
                GET /api/v1/search?q={encodeURIComponent(query)}&type={searchType}
              </span>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="border-b border-slate-800 text-[11px] font-semibold uppercase tracking-wider text-slate-400 bg-slate-950/60">
                    <th className="py-3 px-4">Protocol</th>
                    <th className="py-3 px-4">Release Title</th>
                    <th className="py-3 px-4">Indexer</th>
                    <th className="py-3 px-4">Quality</th>
                    <th className="py-3 px-4">Size</th>
                    <th className="py-3 px-4">Peers</th>
                    <th className="py-3 px-4 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/70 text-xs">
                  {results.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="py-10 text-center text-slate-400">
                        No releases found. Try searching for any TV show or movie title.
                      </td>
                    </tr>
                  ) : (
                    results.map((rel, i) => (
                      <tr
                        key={`${rel.guid || i}-${rel.title}`}
                        className="hover:bg-slate-800/40 transition"
                      >
                        <td className="py-3 px-4 whitespace-nowrap">
                          <span
                            className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded font-semibold ${
                              rel.protocol === 'usenet'
                                ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                                : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                            }`}
                          >
                            {rel.protocol || 'torrent'}
                          </span>
                        </td>
                        <td className="py-3 px-4 font-medium text-slate-100 max-w-md truncate">
                          {rel.title}
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap">
                          <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-300 font-mono text-[11px]">
                            {rel.indexer}
                          </span>
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap">
                          <span className="px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30 font-mono text-[11px]">
                            {rel.quality || 'WEBDL-1080p'}
                          </span>
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap font-mono text-slate-300">
                          {formatBytes(rel.size)}
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap font-mono">
                          {rel.protocol === 'usenet' ? (
                            <span className="text-amber-400">NZB</span>
                          ) : (
                            <span className="text-emerald-400">
                              {rel.seeders ?? 0} S / {rel.peers ?? 0} P
                            </span>
                          )}
                        </td>
                        <td className="py-3 px-4 whitespace-nowrap text-right">
                          <button
                            onClick={() => handleGrab(rel)}
                            disabled={grabbingGuid === (rel.guid || rel.title)}
                            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-semibold transition"
                          >
                            <Download className="w-3.5 h-3.5" />
                            <span>
                              {grabbingGuid === (rel.guid || rel.title)
                                ? 'Grabbing...'
                                : 'Grab to Queue'}
                            </span>
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* SUBVIEW 2: Configured Indexers */}
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
                <button
                  onClick={() => handleDeleteIndexer(idx)}
                  className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                  title="Delete Indexer"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
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
                  Prowlarr Cardigann YAML Definitions (.tmp/ProwlarrIndexers)
                </h3>
                <p className="text-xs text-slate-400">
                  Click any YAML definition to add and configure it as an active Replacearr indexer
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
              onClick={() => {
                setProxyName('FlareSolverr');
                setProxyImpl('FlareSolverr');
                setProxyHost('http://localhost:8191/');
                setProxyTimeout(60);
                setProxySessionTtl(15);
                setProxyDisableMedia(false);
                setShowAddProxyModal(true);
              }}
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

                      <button
                        onClick={() => handleDeleteProxy(proxy)}
                        className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                        title="Delete Indexer Proxy"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
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

      {/* Add Indexer Proxy (FlareSolverr) Modal */}
      {showAddProxyModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <ShieldCheck className="w-5 h-5 text-orange-400" />
                <h3 className="text-base font-bold text-white">
                  Add FlareSolverr / Indexer Proxy
                </h3>
              </div>
              <button
                onClick={() => setShowAddProxyModal(false)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleCreateProxy} className="p-5 space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Proxy Name
                  </label>
                  <input
                    type="text"
                    required
                    value={proxyName}
                    onChange={(e) => setProxyName(e.target.value)}
                    placeholder="FlareSolverr"
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Implementation
                  </label>
                  <select
                    value={proxyImpl}
                    onChange={(e) =>
                      setProxyImpl(e.target.value as 'FlareSolverr' | 'Http' | 'Socks5')
                    }
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="FlareSolverr">FlareSolverr (Cloudflare / Turnstile)</option>
                    <option value="Http">HTTP Proxy</option>
                    <option value="Socks5">SOCKS5 Proxy</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Host URL
                </label>
                <input
                  type="text"
                  required
                  value={proxyHost}
                  onChange={(e) => setProxyHost(e.target.value)}
                  placeholder="http://localhost:8191/ or http://flaresolverr:8191/"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Request Timeout (seconds, 1-180)
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={180}
                    value={proxyTimeout}
                    onChange={(e) => setProxyTimeout(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Session TTL (minutes)
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={1440}
                    value={proxySessionTtl}
                    onChange={(e) => setProxySessionTtl(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>
              </div>

              <label className="flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
                <input
                  type="checkbox"
                  checked={proxyDisableMedia}
                  onChange={(e) => setProxyDisableMedia(e.target.checked)}
                  className="rounded border-slate-700 bg-slate-950 text-orange-500"
                />
                <span>
                  Disable Media (<code className="text-amber-300">disableMedia: true</code>) — skip
                  images, CSS, and fonts in headless Chrome
                </span>
              </label>

              <div className="pt-3 border-t border-slate-800 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowAddProxyModal(false)}
                  className="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-orange-500 hover:bg-orange-400 text-slate-950 text-xs font-bold transition"
                >
                  Save Proxy
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Add Indexer Modal */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Sparkles className="w-5 h-5 text-amber-400" />
                <h3 className="text-base font-bold text-white">Add Indexer</h3>
              </div>
              <button
                onClick={() => setShowAddModal(false)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleCreateIndexer} className="p-5 space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Indexer Name
                  </label>
                  <input
                    type="text"
                    required
                    value={idxName}
                    onChange={(e) => setIdxName(e.target.value)}
                    placeholder="e.g. 1337x, NZBGeek, Custom Torznab"
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Implementation
                  </label>
                  <select
                    value={idxImplementation}
                    onChange={(e) => {
                      const val = e.target.value;
                      setIdxImplementation(val);
                      if (val === 'Newznab') setIdxProtocol('usenet');
                      else setIdxProtocol('torrent');
                    }}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="Torznab">Torznab (Torrent Feed)</option>
                    <option value="Newznab">Newznab (Usenet Feed)</option>
                    <option value="Cardigann">Cardigann YAML Definition</option>
                    <option value="Plugin">Go Subprocess Plugin</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Protocol
                  </label>
                  <select
                    value={idxProtocol}
                    onChange={(e) => setIdxProtocol(e.target.value as 'torrent' | 'usenet')}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="torrent">torrent</option>
                    <option value="usenet">usenet</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Privacy
                  </label>
                  <select
                    value={idxPrivacy}
                    onChange={(e) =>
                      setIdxPrivacy(e.target.value as 'public' | 'private' | 'semiPrivate')
                    }
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="public">Public</option>
                    <option value="semiPrivate">Semi-Private</option>
                    <option value="private">Private</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Priority (1-50)
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={50}
                    value={idxPriority}
                    onChange={(e) => setIdxPriority(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  Base URL
                </label>
                <input
                  type="text"
                  value={idxBaseUrl}
                  onChange={(e) => setIdxBaseUrl(e.target.value)}
                  placeholder="https://indexer.example.com"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                />
              </div>

              {idxImplementation === 'Cardigann' && (
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Cardigann Definition ID
                  </label>
                  <select
                    value={idxDefFile}
                    onChange={(e) => setIdxDefFile(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                  >
                    <option value="">-- Select YAML Definition --</option>
                    {definitions.map((d) => (
                      <option key={d.id} value={d.id}>
                        {d.name} ({d.id}.yml)
                      </option>
                    ))}
                  </select>
                </div>
              )}

              {idxImplementation === 'Plugin' && (
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Go Plugin ID
                  </label>
                  <div className="flex items-center gap-2">
                    <Puzzle className="w-4 h-4 text-purple-400" />
                    <select
                      value={idxPluginId}
                      onChange={(e) => setIdxPluginId(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                    >
                      <option value="">-- Select Discovered Go Plugin --</option>
                      {plugins.map((p) => (
                        <option key={p.manifest.id} value={p.manifest.id}>
                          {p.manifest.name} ({p.manifest.id})
                        </option>
                      ))}
                    </select>
                  </div>
                </div>
              )}

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">
                  API Key (optional for public indexers)
                </label>
                <input
                  type="password"
                  value={idxApiKey}
                  onChange={(e) => setIdxApiKey(e.target.value)}
                  placeholder="Optional Torznab/Newznab API Key"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                />
              </div>

              <div className="pt-3 border-t border-slate-800 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowAddModal(false)}
                  className="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-amber-500 hover:bg-amber-400 text-slate-950 text-xs font-bold transition"
                >
                  Save Indexer
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
