import React, { useCallback, useEffect, useState } from 'react';
import {
  DownloadCloud,
  Server,
  Plus,
  Trash2,
  FolderPlus,
  HardDrive,
  CheckCircle2,
  CircleOff,
  Sparkles,
  RefreshCw,
  Radar,
} from 'lucide-react';
import type {
  AppConfig,
  DiscoveredDownloadClient,
  DownloadClient,
  DownloadClientOptions,
  QualityProfile,
  QueueItem,
  RemotePathMapping,
  RootFolder,
} from '../types';
import { api, formatBytes } from '../api';
import {
  blankDownloadClientForm,
  discoveredToForm,
  DownloadClientForm,
  DownloadClientModal,
  downloadClientToForm,
} from './modals/DownloadClientModal';
import { useConfirm } from './modals/ConfirmModal';

const QUEUE_POLL_MS = 5000;

function urlPort(baseUrl: string): string {
  try {
    const raw = baseUrl.includes('://') ? baseUrl : `http://${baseUrl}`;
    const u = new URL(raw);
    if (u.port) return u.port;
    return u.protocol === 'https:' ? '443' : '80';
  } catch {
    return '';
  }
}

function normalizeClientUrl(baseUrl: string): string {
  return baseUrl.trim().toLowerCase().replace(/\/+$/, '');
}

/** True when a discovery hit matches an already-configured client. */
function isAlreadyAdded(hit: DiscoveredDownloadClient, configured: DownloadClient[]): boolean {
  const hitUrl = normalizeClientUrl(hit.baseUrl);
  const hitPort = urlPort(hit.baseUrl);
  return configured.some((dc) => {
    if (normalizeClientUrl(dc.baseUrl) === hitUrl) return true;
    return (
      dc.implementation === hit.implementation &&
      hitPort !== '' &&
      urlPort(dc.baseUrl) === hitPort
    );
  });
}

interface QueueClientsTabProps {
  queue: QueueItem[];
  clients: DownloadClient[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  appConfig?: AppConfig | null;
  onSaveConfig?: (next: AppConfig) => Promise<AppConfig>;
  onRefresh: () => Promise<void>;
  onRefreshQueue?: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const QueueClientsTab: React.FC<QueueClientsTabProps> = ({
  queue,
  clients,
  profiles,
  rootFolders,
  appConfig,
  onSaveConfig,
  onRefresh,
  onRefreshQueue,
  onNotify,
}) => {
  const { confirm, dialog } = useConfirm();
  const [clientForm, setClientForm] = useState<DownloadClientForm | null>(null);
  const [opts, setOpts] = useState<DownloadClientOptions | null>(appConfig?.downloadClientOptions ?? null);
  const [mappings, setMappings] = useState<RemotePathMapping[]>(appConfig?.remotePathMappings ?? []);
  const [savingOpts, setSavingOpts] = useState(false);
  const [queueRefreshing, setQueueRefreshing] = useState(false);
  const [discovering, setDiscovering] = useState(false);
  const [discovered, setDiscovered] = useState<DiscoveredDownloadClient[] | null>(null);

  const handleFindNearby = async () => {
    setDiscovering(true);
    try {
      const hits = await api.discoverDownloadClients();
      // Backend already filters configured clients; also drop any that match local state.
      setDiscovered(hits.filter((h) => !isAlreadyAdded(h, clients)));
      if (hits.length === 0) {
        onNotify('No clients found on local or SSDP hosts');
      }
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Client discovery failed', true);
    } finally {
      setDiscovering(false);
    }
  };

  // Checklist UX: as clients are added, peel them off the nearby list without re-scanning.
  useEffect(() => {
    setDiscovered((prev) => {
      if (!prev || prev.length === 0) return prev;
      const next = prev.filter((h) => !isAlreadyAdded(h, clients));
      return next.length === prev.length ? prev : next;
    });
  }, [clients]);

  const pendingDiscoveries = (discovered ?? []).filter((h) => !isAlreadyAdded(h, clients));

  const refreshQueue = useCallback(async () => {
    if (onRefreshQueue) {
      await onRefreshQueue();
      return;
    }
    await onRefresh();
  }, [onRefresh, onRefreshQueue]);

  const handleManualQueueRefresh = async () => {
    setQueueRefreshing(true);
    try {
      await refreshQueue();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to refresh queue', true);
    } finally {
      setQueueRefreshing(false);
    }
  };

  useEffect(() => {
    if (appConfig) {
      setOpts(appConfig.downloadClientOptions);
      setMappings(appConfig.remotePathMappings ?? []);
    }
  }, [appConfig]);

  useEffect(() => {
    const id = window.setInterval(() => {
      void refreshQueue();
    }, QUEUE_POLL_MS);
    return () => window.clearInterval(id);
  }, [refreshQueue]);

  // Root folder form
  const [newFolderPath, setNewFolderPath] = useState('');
  const [newFolderType, setNewFolderType] = useState('all');

  const handleRemoveQueueItem = async (item: QueueItem) => {
    const ok = await confirm({
      title: 'Remove from queue',
      message: `Remove "${item.title}" from the download queue?`,
      confirmLabel: 'Remove',
    });
    if (!ok) return;
    try {
      await api.removeQueueItem(item.id);
      onNotify(`Removed "${item.title}" from download queue`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to remove queue item', true);
    }
  };

  const handleDeleteClient = async (dc: DownloadClient) => {
    const ok = await confirm({
      title: 'Delete download client',
      message: `Delete download client "${dc.name}"?`,
      confirmLabel: 'Delete',
    });
    if (!ok) return;
    try {
      await api.deleteDownloadClient(dc.id);
      onNotify(`Removed download client "${dc.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete client', true);
    }
  };

  const handleAddRootFolder = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFolderPath.trim()) return;
    try {
      const created = await api.addRootFolder({
        path: newFolderPath.trim(),
        mediaType: newFolderType,
      });
      onNotify(`Added root folder "${created.path}"`);
      setNewFolderPath('');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add root folder', true);
    }
  };

  const handleDeleteRootFolder = async (folder: RootFolder) => {
    const ok = await confirm({
      title: 'Remove root folder',
      message: `Remove root folder "${folder.path}"?`,
      confirmLabel: 'Remove',
    });
    if (!ok) return;
    try {
      await api.deleteRootFolder(folder.id);
      onNotify(`Removed root folder "${folder.path}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to remove root folder', true);
    }
  };

  return (
    <div className="space-y-6">
      {/* Live Download Queue Section */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-sky-500/15 text-sky-400 border border-sky-500/30">
              <DownloadCloud className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-white">Live Download Queue</h2>
              <p className="text-xs text-slate-400">
                {queue.length} active transfers across Torrent & Usenet download clients · auto-refresh 5s
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => void handleManualQueueRefresh()}
            disabled={queueRefreshing}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-300 hover:text-white hover:bg-slate-800 transition disabled:opacity-60"
            title="Refresh download progress from clients"
            aria-label="Refresh download queue"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${queueRefreshing ? 'animate-spin text-sky-400' : ''}`} />
            <span>Refresh</span>
          </button>
        </div>

        {queue.length === 0 ? (
          <div className="py-10 text-center border border-dashed border-slate-800 rounded-xl">
            <DownloadCloud className="w-9 h-9 text-slate-600 mx-auto mb-2" />
            <p className="text-sm font-medium text-slate-300">No Active Downloads</p>
            <p className="text-xs text-slate-500 mt-1">
              Use Indexers & Search to grab a release and watch progress here.
            </p>
          </div>
        ) : (
          <div className="space-y-3">
            {queue.map((item) => {
              const pct =
                item.size > 0
                  ? Math.min(
                      100,
                      Math.max(0, Math.round(((item.size - item.sizeleft) / item.size) * 100))
                    )
                  : 0;
              return (
                <div
                  key={item.id}
                  className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4"
                >
                  <div className="flex-1 min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded font-semibold ${
                          item.protocol === 'usenet'
                            ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                            : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                        }`}
                      >
                        {item.protocol || 'torrent'}
                      </span>
                      <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                        {item.mediaType || 'tv'}
                      </span>
                      <span className="text-sm font-bold text-white break-all min-w-0">{item.title}</span>
                    </div>

                    <div className="w-full h-2 bg-slate-800 rounded-full overflow-hidden">
                      <div
                        className="h-full bg-gradient-to-r from-sky-500 via-indigo-500 to-emerald-400 rounded-full transition-all"
                        style={{ width: `${pct}%` }}
                      />
                    </div>

                    <div className="flex flex-wrap items-center gap-4 text-xs text-slate-400">
                      <span>
                        Client: <strong className="text-slate-200">{item.downloadClient || '—'}</strong>
                      </span>
                      <span>
                        Indexer: <strong className="text-slate-200">{item.indexer || '—'}</strong>
                      </span>
                      <span>
                        Status: <strong className="text-emerald-400">{item.status}</strong>
                      </span>
                      <span className="font-mono">
                        {item.size > 0
                          ? `${formatBytes(item.size - item.sizeleft)} / ${formatBytes(item.size)} (${pct}%)`
                          : 'size unknown'}
                      </span>
                      <span className="font-mono text-sky-300">
                        {item.timeleft ? `ETA: ${item.timeleft}` : 'ETA: not tracked'}
                      </span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      onClick={() => handleRemoveQueueItem(item)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/30 text-xs font-medium transition"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                      <span>Remove</span>
                    </button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Download Clients Management */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3 min-w-0">
            <div className="p-2.5 rounded-xl bg-indigo-500/15 text-indigo-400 border border-indigo-500/30">
              <Server className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-white">Download Clients</h3>
              <p className="text-xs text-slate-400">
                qBittorrent, Transmission, Deluge, SABnzbd, NZBGet & Go Plugin clients
              </p>
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 shrink-0">
            <button
              type="button"
              disabled={discovering}
              onClick={() => void handleFindNearby()}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-slate-800 hover:bg-slate-700 disabled:opacity-50 text-slate-200 text-xs font-semibold rounded-lg border border-slate-700 transition"
            >
              <Radar className={`w-4 h-4 ${discovering ? 'animate-pulse' : ''}`} />
              <span>{discovering ? 'Scanning…' : 'Find nearby'}</span>
            </button>
            <button
              type="button"
              onClick={() => setClientForm(blankDownloadClientForm())}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
            >
              <Plus className="w-4 h-4" />
              <span>Add Download Client</span>
            </button>
          </div>
        </div>

        {pendingDiscoveries.length > 0 && (
          <div className="rounded-xl border border-sky-500/25 bg-sky-500/5 p-3 space-y-2">
            <p className="text-xs text-sky-200/90 font-medium">
              Nearby clients — click to add ({pendingDiscoveries.length} remaining)
            </p>
            <ul className="space-y-1.5">
              {pendingDiscoveries.map((hit) => (
                <li key={`${hit.implementation}:${hit.baseUrl}`}>
                  <button
                    type="button"
                    onClick={() => setClientForm(discoveredToForm(hit))}
                    className="w-full text-left px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 hover:border-sky-500/40 transition"
                  >
                    <div className="text-sm font-semibold text-white">
                      {hit.nameHint || hit.implementation}
                      <span className="ml-2 text-[10px] font-mono uppercase text-slate-500">
                        {hit.protocol}
                      </span>
                    </div>
                    <div className="text-xs font-mono text-slate-400 break-all">{hit.baseUrl}</div>
                    {hit.detail ? (
                      <div className="text-[11px] text-slate-500 mt-0.5">{hit.detail}</div>
                    ) : null}
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {clients.map((dc) => (
            <div
              key={dc.id}
              className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex flex-col justify-between space-y-3"
            >
              <div>
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <h4 className="text-sm font-bold text-white break-words">{dc.name}</h4>
                    <p className="text-xs font-mono text-slate-400 mt-0.5 break-all">
                      {dc.baseUrl}
                    </p>
                  </div>
                  <span
                    className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded font-semibold ${
                      dc.protocol === 'usenet'
                        ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                        : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                    }`}
                  >
                    {dc.protocol}
                  </span>
                </div>

                <div className="mt-3 flex flex-wrap gap-2 text-[11px] text-slate-400">
                  <span className="px-2 py-0.5 rounded bg-slate-900 border border-slate-800">
                    Impl: <strong className="text-slate-200">{dc.implementation}</strong>
                  </span>
                  <span className="px-2 py-0.5 rounded bg-slate-900 border border-slate-800">
                    Category: <strong className="text-slate-200">{dc.category || 'replacearr'}</strong>
                  </span>
                </div>
              </div>

              <div className="pt-3 border-t border-slate-800/80 flex items-center justify-between">
                <span
                  className={`inline-flex items-center gap-1 text-xs ${
                    dc.enable ? 'text-emerald-400' : 'text-amber-400'
                  }`}
                >
                  {dc.enable ? (
                    <CheckCircle2 className="w-3.5 h-3.5" />
                  ) : (
                    <CircleOff className="w-3.5 h-3.5" />
                  )}
                  {dc.enable ? 'Enabled' : 'Disabled'}
                </span>
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => setClientForm(downloadClientToForm(dc))}
                    className="px-2 py-1 rounded-lg text-xs text-slate-300 hover:text-white hover:bg-slate-800 transition"
                  >
                    Edit
                  </button>
                  <button
                    type="button"
                    onClick={() => void handleDeleteClient(dc)}
                    className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Root Folders & Quality Profiles */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center gap-2.5">
            <HardDrive className="w-4 h-4 text-emerald-400" />
            <h3 className="text-sm font-bold text-white">Root Media Folders</h3>
          </div>

          <div className="space-y-2">
            {rootFolders.length === 0 ? (
              <p className="text-xs text-slate-500">No root folders yet. Add one below or use the setup wizard.</p>
            ) : (
              rootFolders.map((rf) => (
                <div
                  key={rf.id}
                  className="p-3 rounded-lg bg-slate-950 border border-slate-800 flex items-center justify-between gap-2 text-xs min-w-0"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <code className="font-mono text-slate-200 truncate">{rf.path}</code>
                    <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 shrink-0">
                      {rf.mediaType}
                    </span>
                  </div>
                  <div className="flex items-center gap-2 shrink-0">
                    <span className="font-mono text-emerald-400 whitespace-nowrap">
                      {formatBytes(rf.freeSpace)} free
                    </span>
                    <button
                      type="button"
                      onClick={() => void handleDeleteRootFolder(rf)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-rose-300 hover:bg-rose-500/10 transition"
                      title={`Remove ${rf.path}`}
                      aria-label={`Remove root folder ${rf.path}`}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>

          <form onSubmit={handleAddRootFolder} className="flex flex-wrap gap-2 pt-2">
            <input
              type="text"
              value={newFolderPath}
              onChange={(e) => setNewFolderPath(e.target.value)}
              placeholder="/mnt/nas/media"
              className="flex-1 min-w-[10rem] px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
            />
            <select
              value={newFolderType}
              onChange={(e) => setNewFolderType(e.target.value)}
              className="px-2.5 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300"
            >
              {/* <option value="all">all</option> */}
              <option value="tv">tv</option>
              <option value="movie">movie</option>
              <option value="music">music</option>
              <option value="ebook">ebook</option>
              <option value="audiobook">audiobook</option>
            </select>
            <button
              type="submit"
              className="inline-flex items-center gap-1 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-white text-xs font-medium rounded-lg transition"
            >
              <FolderPlus className="w-3.5 h-3.5" />
              <span>Add</span>
            </button>
          </form>
        </div>

        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center gap-2.5">
            <Sparkles className="w-4 h-4 text-indigo-400" />
            <h3 className="text-sm font-bold text-white">Quality Profiles (/api/v1/profiles)</h3>
          </div>
          <div className="space-y-2.5">
            {profiles.map((qp) => (
              <div
                key={qp.id}
                className="p-3.5 rounded-lg bg-slate-950 border border-slate-800 space-y-2"
              >
                <div className="flex items-center justify-between">
                  <span className="text-xs font-bold text-white">{qp.name}</span>
                  <span className="text-[11px] text-emerald-400 font-mono">
                    {qp.upgradeAllowed ? 'Upgrades Allowed' : 'Static'}
                  </span>
                </div>
                <div className="flex flex-wrap gap-1.5">
                  {qp.items?.map((item) => (
                    <span
                      key={item.id}
                      className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300"
                    >
                      {item.name}
                    </span>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {opts && appConfig && onSaveConfig ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
          <h3 className="text-sm font-bold text-white">Completed Download Handling</h3>
          <div className="space-y-2 text-xs text-slate-300">
            {(
              [
                ['enableCompletedDownloadHandling', 'Enable Completed Download Handling'],
                ['autoRedownloadFailed', 'Auto Redownload Failed'],
                ['autoRedownloadFailedFromInteractiveSearch', 'Auto Redownload Failed from Interactive Search'],
              ] as const
            ).map(([key, label]) => (
              <label key={key} className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={Boolean(opts[key])}
                  onChange={(e) => setOpts({ ...opts, [key]: e.target.checked })}
                />
                {label}
              </label>
            ))}
            <label className="block space-y-1">
              <span className="text-slate-400">Working folders</span>
              <input
                value={opts.downloadClientWorkingFolders}
                onChange={(e) => setOpts({ ...opts, downloadClientWorkingFolders: e.target.value })}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
              />
            </label>
          </div>

          <h3 className="text-sm font-bold text-white pt-2">Remote Path Mappings</h3>
          <div className="space-y-2">
            {mappings.map((m, index) => (
              <div key={`${m.id}-${index}`} className="grid grid-cols-1 md:grid-cols-[1fr_1fr_1fr_auto] gap-2">
                <input
                  value={m.host}
                  placeholder="Host"
                  onChange={(e) =>
                    setMappings((prev) =>
                      prev.map((row, i) => (i === index ? { ...row, host: e.target.value } : row))
                    )
                  }
                  className="bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
                <input
                  value={m.remotePath}
                  placeholder="Remote path"
                  onChange={(e) =>
                    setMappings((prev) =>
                      prev.map((row, i) => (i === index ? { ...row, remotePath: e.target.value } : row))
                    )
                  }
                  className="bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
                <input
                  value={m.localPath}
                  placeholder="Local path"
                  onChange={(e) =>
                    setMappings((prev) =>
                      prev.map((row, i) => (i === index ? { ...row, localPath: e.target.value } : row))
                    )
                  }
                  className="bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
                <button
                  type="button"
                  onClick={() => setMappings((prev) => prev.filter((_, i) => i !== index))}
                  className="px-2 py-1 rounded-lg text-xs text-rose-300 border border-rose-500/30"
                >
                  Remove
                </button>
              </div>
            ))}
            <button
              type="button"
              onClick={() =>
                setMappings((prev) => [...prev, { id: 0, host: '', remotePath: '', localPath: '' }])
              }
              className="px-3 py-1.5 rounded-lg text-xs border border-slate-700 text-slate-300"
            >
              Add mapping
            </button>
          </div>

          <button
            type="button"
            disabled={savingOpts}
            onClick={() => {
              void (async () => {
                setSavingOpts(true);
                try {
                  await onSaveConfig({
                    ...appConfig,
                    downloadClientOptions: opts,
                    remotePathMappings: mappings,
                  });
                  onNotify('Download client options saved');
                } catch (err) {
                  onNotify(err instanceof Error ? err.message : 'Save failed', true);
                } finally {
                  setSavingOpts(false);
                }
              })();
            }}
            className="px-3 py-2 rounded-lg text-xs bg-indigo-600 text-white disabled:opacity-60"
          >
            Save options
          </button>
        </div>
      ) : null}

      {clientForm ? (
        <DownloadClientModal
          initial={clientForm}
          onClose={() => setClientForm(null)}
          onSaved={onRefresh}
          onNotify={onNotify}
        />
      ) : null}
      {dialog}
    </div>
  );
};
