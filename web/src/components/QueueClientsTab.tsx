import React, { useState } from 'react';
import {
  DownloadCloud,
  Server,
  Plus,
  Trash2,
  FolderPlus,
  HardDrive,
  X,
  CheckCircle2,
  Sparkles,
} from 'lucide-react';
import type { DownloadClient, QualityProfile, QueueItem, RootFolder } from '../types';
import { api, formatBytes } from '../api';

interface QueueClientsTabProps {
  queue: QueueItem[];
  clients: DownloadClient[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const QueueClientsTab: React.FC<QueueClientsTabProps> = ({
  queue,
  clients,
  profiles,
  rootFolders,
  onRefresh,
  onNotify,
}) => {
  const [showAddClientModal, setShowAddClientModal] = useState(false);
  const [clientName, setClientName] = useState('');
  const [clientImpl, setClientImpl] = useState('qBittorrent');
  const [clientProtocol, setClientProtocol] = useState<'torrent' | 'usenet'>('torrent');
  const [clientHost, setClientHost] = useState('localhost');
  const [clientPort, setClientPort] = useState(8080);
  const [clientUsername, setClientUsername] = useState('admin');
  const [clientCategory, setClientCategory] = useState('replacearr');

  // Root folder form
  const [newFolderPath, setNewFolderPath] = useState('');
  const [newFolderType, setNewFolderType] = useState('all');

  const handleRemoveQueueItem = async (item: QueueItem) => {
    try {
      await api.removeQueueItem(item.id);
      onNotify(`Removed "${item.title}" from download queue`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to remove queue item', true);
    }
  };

  const handleAddClient = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const created = await api.addDownloadClient({
        name: clientName || clientImpl,
        implementation: clientImpl,
        implementationName: clientImpl,
        configContract: `${clientImpl}Settings`,
        protocol: clientProtocol,
        enable: true,
        priority: 1,
        host: clientHost,
        port: clientPort,
        useSsl: false,
        username: clientUsername,
        category: clientCategory,
        removeCompletedDownloads: true,
        removeFailedDownloads: true,
      });
      onNotify(`Added download client "${created.name}"`);
      setShowAddClientModal(false);
      setClientName('');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add download client', true);
    }
  };

  const handleDeleteClient = async (dc: DownloadClient) => {
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
                {queue.length} active transfers across Torrent & Usenet download clients
              </p>
            </div>
          </div>
          <span className="text-xs font-mono text-slate-500">GET /api/v1/queue</span>
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
                      Math.max(5, Math.round(((item.size - item.sizeleft) / item.size) * 100))
                    )
                  : 70;
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
                        Client: <strong className="text-slate-200">{item.downloadClient}</strong>
                      </span>
                      <span>
                        Indexer: <strong className="text-slate-200">{item.indexer || 'Replacearr'}</strong>
                      </span>
                      <span>
                        Status: <strong className="text-emerald-400">{item.status}</strong>
                      </span>
                      <span className="font-mono">
                        {formatBytes(item.size - item.sizeleft)} / {formatBytes(item.size)} ({pct}%)
                      </span>
                      <span className="font-mono text-sky-300">ETA: {item.timeleft || '00:08:30'}</span>
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
                qBittorrent, Transmission, SABnzbd, NZBGet & Go Plugin clients
              </p>
            </div>
          </div>

          <button
            onClick={() => setShowAddClientModal(true)}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Add Download Client</span>
          </button>
        </div>

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
                      {dc.useSsl ? 'https' : 'http'}://{dc.host}:{dc.port}
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
                <span className="inline-flex items-center gap-1 text-xs text-emerald-400">
                  <CheckCircle2 className="w-3.5 h-3.5" /> Enabled
                </span>
                <button
                  onClick={() => handleDeleteClient(dc)}
                  className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
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
            {rootFolders.map((rf) => (
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
                <span className="font-mono text-emerald-400 shrink-0 whitespace-nowrap">
                  {formatBytes(rf.freeSpace)} free
                </span>
              </div>
            ))}
          </div>

          <form onSubmit={handleAddRootFolder} className="flex flex-wrap gap-2 pt-2">
            <input
              type="text"
              value={newFolderPath}
              onChange={(e) => setNewFolderPath(e.target.value)}
              placeholder="/data/anime or /mnt/nas/media"
              className="flex-1 min-w-[10rem] px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
            />
            <select
              value={newFolderType}
              onChange={(e) => setNewFolderType(e.target.value)}
              className="px-2.5 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-300"
            >
              <option value="all">all</option>
              <option value="tv">tv</option>
              <option value="movie">movie</option>
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

      {/* Add Download Client Modal */}
      {showAddClientModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <h3 className="text-base font-bold text-white">Add Download Client</h3>
              <button
                onClick={() => setShowAddClientModal(false)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleAddClient} className="p-5 space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Client Name
                  </label>
                  <input
                    type="text"
                    required
                    value={clientName}
                    onChange={(e) => setClientName(e.target.value)}
                    placeholder="e.g. Primary qBittorrent"
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Implementation
                  </label>
                  <select
                    value={clientImpl}
                    onChange={(e) => {
                      const val = e.target.value;
                      setClientImpl(val);
                      if (val === 'SABnzbd' || val === 'NZBGet') {
                        setClientProtocol('usenet');
                      } else {
                        setClientProtocol('torrent');
                      }
                    }}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="qBittorrent">qBittorrent</option>
                    <option value="Transmission">Transmission</option>
                    <option value="SABnzbd">SABnzbd</option>
                    <option value="NZBGet">NZBGet</option>
                    <option value="Deluge">Deluge</option>
                  </select>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div className="sm:col-span-2">
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Host
                  </label>
                  <input
                    type="text"
                    value={clientHost}
                    onChange={(e) => setClientHost(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Port
                  </label>
                  <input
                    type="number"
                    value={clientPort}
                    onChange={(e) => setClientPort(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                  />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Username
                  </label>
                  <input
                    type="text"
                    value={clientUsername}
                    onChange={(e) => setClientUsername(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Category
                  </label>
                  <input
                    type="text"
                    value={clientCategory}
                    onChange={(e) => setClientCategory(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                  />
                </div>
              </div>

              <div className="pt-3 border-t border-slate-800 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowAddClientModal(false)}
                  className="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold transition"
                >
                  Save Client
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
