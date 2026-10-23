import React, { useState } from 'react';
import {
  Tv,
  Plus,
  Search,
  Trash2,
  ListVideo,
  CheckCircle2,
  Clock,
  Folder,
  X,
  Radio,
  Sparkles,
  Calendar,
} from 'lucide-react';
import type { AppConfig, Episode, QualityProfile, RootFolder, Series } from '../types';
import { api } from '../api';
import { formatAppDate } from '../uiPrefs';

interface SeriesTabProps {
  series: Series[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onSearchReleases: (query: string, type: string) => void;
  onNotify: (msg: string, isError?: boolean) => void;
  appUi?: AppConfig['ui'] | null;
}

export const SeriesTab: React.FC<SeriesTabProps> = ({
  series,
  profiles,
  rootFolders,
  onRefresh,
  onSearchReleases,
  onNotify,
  appUi,
}) => {
  const [filterText, setFilterText] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);
  const [lookupTerm, setLookupTerm] = useState('');
  const [lookupResults, setLookupResults] = useState<Series[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(profiles[0]?.id || 1);
  const [selectedRootFolder, setSelectedRootFolder] = useState<string>(
    rootFolders.find((r) => r.mediaType === 'tv')?.path || rootFolders[0]?.path || '/data/tv'
  );

  // Episode Drawer State
  const [activeSeries, setActiveSeries] = useState<Series | null>(null);
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [episodesLoading, setEpisodesLoading] = useState(false);

  const filteredSeries = series.filter(
    (s) =>
      s.title.toLowerCase().includes(filterText.toLowerCase()) ||
      (s.network || '').toLowerCase().includes(filterText.toLowerCase())
  );

  const handleLookup = async (e?: React.FormEvent, customTerm?: string) => {
    if (e) e.preventDefault();
    const term = (customTerm ?? lookupTerm).trim();
    if (!term) return;
    setLookupLoading(true);
    try {
      const results = await api.lookupSeries(term);
      setLookupResults(results);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Series lookup failed', true);
    } finally {
      setLookupLoading(false);
    }
  };

  const handleAddSeries = async (candidate: Partial<Series>) => {
    try {
      const created = await api.addSeries({
        ...candidate,
        qualityProfileId: selectedProfileId || 1,
        rootFolderPath: selectedRootFolder || '/data/tv',
        monitored: true,
        seasonFolder: true,
      });
      onNotify(`Added "${created.title}" to TV Series library`);
      setShowAddModal(false);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add series', true);
    }
  };

  const handleDeleteSeries = async (s: Series) => {
    try {
      await api.deleteSeries(s.id);
      onNotify(`Removed "${s.title}" from library`);
      if (activeSeries?.id === s.id) {
        setActiveSeries(null);
      }
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete series', true);
    }
  };

  const openEpisodesDrawer = async (s: Series) => {
    setActiveSeries(s);
    setEpisodesLoading(true);
    try {
      const eps = await api.getEpisodes(s.id);
      setEpisodes(eps);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load episodes', true);
    } finally {
      setEpisodesLoading(false);
    }
  };

  const getPosterUrl = (s: Series): string | null => {
    const poster = s.images?.find((img) => img.coverType === 'poster') || s.images?.[0];
    return poster?.remoteUrl || poster?.url || null;
  };

  return (
    <div className="space-y-6">
      {/* Header & Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-indigo-500/15 text-indigo-400 border border-indigo-500/30">
            <Tv className="w-5 h-5" />
          </div>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-bold text-white">TV Series Library</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-indigo-500/15 text-indigo-300 border border-indigo-500/30 font-mono">
                Sonarr v3 Compatible
              </span>
            </div>
            <p className="text-xs text-slate-400">
              {series.length} series monitored • Automatic season & episode tracking
            </p>
          </div>
        </div>

        <div className="flex flex-col min-[480px]:flex-row min-[480px]:items-center gap-2">
          <div className="relative flex-1 min-w-0">
            <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={filterText}
              onChange={(e) => setFilterText(e.target.value)}
              placeholder="Filter library..."
              className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500"
            />
          </div>
          <button
            onClick={() => {
              setShowAddModal(true);
              if (lookupResults.length === 0) {
                setLookupTerm('Severance');
                void handleLookup(undefined, 'Severance');
              }
            }}
            className="inline-flex items-center justify-center gap-2 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg shadow-lg shadow-indigo-600/20 transition shrink-0"
          >
            <Plus className="w-4 h-4" />
            <span>Add Series</span>
          </button>
        </div>
      </div>

      {/* Series Poster Grid */}
      {filteredSeries.length === 0 ? (
        <div className="bg-slate-900/60 border border-dashed border-slate-800 rounded-2xl p-12 text-center">
          <Tv className="w-10 h-10 text-slate-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-slate-200">No TV Series Found</h3>
          <p className="text-xs text-slate-400 mt-1 mb-5">
            Add your first series using the TVDB/Sonarr lookup modal to begin monitoring episodes.
          </p>
          <button
            onClick={() => {
              setShowAddModal(true);
              setLookupTerm('Severance');
              void handleLookup(undefined, 'Severance');
            }}
            className="inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Lookup & Add Series</span>
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
          {filteredSeries.map((s) => {
            const posterUrl = getPosterUrl(s);
            return (
              <div
                key={s.id}
                className="group bg-slate-900/90 border border-slate-800 hover:border-indigo-500/40 rounded-xl overflow-hidden flex flex-col justify-between transition shadow-lg"
              >
                <div className="p-4 flex gap-4">
                  {/* Poster Artwork */}
                  <div className="w-24 h-36 rounded-lg bg-gradient-to-br from-indigo-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center relative">
                    {posterUrl ? (
                      <img
                        src={posterUrl}
                        alt={s.title}
                        className="w-full h-full object-cover"
                        onError={(e) => {
                          (e.currentTarget as HTMLImageElement).style.display = 'none';
                        }}
                      />
                    ) : (
                      <Tv className="w-8 h-8 text-indigo-400/60" />
                    )}
                    <span className="absolute bottom-1.5 right-1.5 text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-950/90 text-indigo-300 border border-indigo-500/30">
                      {s.year}
                    </span>
                  </div>

                  {/* Metadata */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between gap-2">
                      <h3 className="text-base font-bold text-white truncate">{s.title}</h3>
                      <span
                        className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded-full shrink-0 ${
                          s.status === 'continuing'
                            ? 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                            : 'bg-slate-800 text-slate-300'
                        }`}
                      >
                        {s.status || 'continuing'}
                      </span>
                    </div>

                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                      {s.network && (
                        <span className="text-indigo-300 font-medium">{s.network}</span>
                      )}
                      <span>•</span>
                      <span>{s.seasons?.length || 1} Seasons</span>
                      <span>•</span>
                      <span>{s.runtime || 45}m</span>
                    </div>

                    <p className="mt-2 text-xs text-slate-400 line-clamp-3 leading-relaxed">
                      {s.overview || 'Monitored TV series in Replacearr unified database.'}
                    </p>

                    <div className="mt-3 flex items-center gap-1.5 text-[11px] font-mono text-slate-500 truncate">
                      <Folder className="w-3.5 h-3.5 text-slate-500 shrink-0" />
                      <span className="truncate">{s.path || `${s.rootFolderPath}/${s.title}`}</span>
                    </div>
                  </div>
                </div>

                {/* Action Footer */}
                <div className="px-4 py-3 bg-slate-950/70 border-t border-slate-800/80 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => openEpisodesDrawer(s)}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-800 text-xs font-medium transition"
                    >
                      <ListVideo className="w-3.5 h-3.5 text-indigo-400" />
                      <span>Episodes</span>
                    </button>
                    <button
                      onClick={() => onSearchReleases(s.title, 'tvsearch')}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-indigo-500/15 hover:bg-indigo-500/25 text-indigo-300 border border-indigo-500/30 text-xs font-medium transition"
                    >
                      <Radio className="w-3.5 h-3.5" />
                      <span>Search Releases</span>
                    </button>
                  </div>

                  <button
                    onClick={() => handleDeleteSeries(s)}
                    title="Delete series"
                    className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Episode List Drawer / Modal */}
      {activeSeries && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex justify-end">
          <div className="w-full max-w-2xl bg-slate-900 border-l border-slate-800 h-full flex flex-col justify-between shadow-2xl">
            <div className="p-4 sm:p-5 border-b border-slate-800 flex items-start justify-between gap-3 shrink-0">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30 shrink-0">
                    TVDB #{activeSeries.tvdbId || activeSeries.id}
                  </span>
                  <h3 className="text-lg font-bold text-white break-words">{activeSeries.title}</h3>
                </div>
                <p className="text-xs text-slate-400 mt-0.5 break-all">
                  Path: <code className="font-mono text-slate-300">{activeSeries.path}</code>
                </p>
              </div>
              <button
                onClick={() => setActiveSeries(null)}
                className="p-2 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-5 space-y-3">
              {episodesLoading ? (
                <div className="py-12 text-center text-sm text-slate-400">
                  Loading episode guide...
                </div>
              ) : episodes.length === 0 ? (
                <div className="py-12 text-center text-sm text-slate-400">
                  No episodes recorded for this series yet.
                </div>
              ) : (
                episodes.map((ep) => (
                  <div
                    key={ep.id}
                    className="p-3.5 rounded-xl bg-slate-950/90 border border-slate-800 flex items-center justify-between gap-4"
                  >
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="text-xs font-mono font-semibold px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                          S{String(ep.seasonNumber).padStart(2, '0')}E
                          {String(ep.episodeNumber).padStart(2, '0')}
                        </span>
                        <span className="text-sm font-medium text-white truncate">
                          {ep.title}
                        </span>
                        {ep.hasFile ? (
                          <span className="inline-flex items-center gap-1 text-[11px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded">
                            <CheckCircle2 className="w-3 h-3" /> Downloaded
                          </span>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-[11px] text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded">
                            <Clock className="w-3 h-3" /> Missing / Monitored
                          </span>
                        )}
                      </div>
                      <p className="text-xs text-slate-400 mt-1 truncate">
                        {ep.overview || `Air Date: ${ep.airDate ? formatAppDate(ep.airDate, appUi) : 'TBA'}`}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      <span className="text-xs font-mono text-slate-500 hidden sm:inline-flex items-center gap-1">
                        <Calendar className="w-3 h-3" />
                        {ep.airDate ? formatAppDate(ep.airDate, appUi) : 'TBA'}
                      </span>
                      <button
                        onClick={() => {
                          const code = `S${String(ep.seasonNumber).padStart(2, '0')}E${String(
                            ep.episodeNumber
                          ).padStart(2, '0')}`;
                          setActiveSeries(null);
                          onSearchReleases(`${activeSeries.title} ${code}`, 'tvsearch');
                        }}
                        className="px-2.5 py-1.5 rounded-lg bg-indigo-600/20 hover:bg-indigo-600 text-indigo-300 hover:text-white border border-indigo-500/30 text-xs font-medium transition"
                      >
                        Search
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>

            <div className="p-4 border-t border-slate-800 bg-slate-950 flex items-center justify-between">
              <span className="text-xs text-slate-400">
                {episodes.length} episodes tracked in SQLite
              </span>
              <button
                onClick={() => {
                  const title = activeSeries.title;
                  setActiveSeries(null);
                  onSearchReleases(title, 'tvsearch');
                }}
                className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold transition"
              >
                Search Full Series Pack
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Add Series Lookup Modal */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-3xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[calc(100dvh-2rem)] my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <Sparkles className="w-5 h-5 text-indigo-400" />
                <div>
                  <h3 className="text-base font-bold text-white">
                    Add TV Series (/api/v1/series/lookup)
                  </h3>
                  <p className="text-xs text-slate-400">
                    Search TV metadata and add directly to Replacearr
                  </p>
                </div>
              </div>
              <button
                onClick={() => setShowAddModal(false)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="p-5 border-b border-slate-800 space-y-4 bg-slate-950/50">
              <form onSubmit={(e) => handleLookup(e)} className="flex gap-2">
                <input
                  type="text"
                  value={lookupTerm}
                  onChange={(e) => setLookupTerm(e.target.value)}
                  placeholder="Enter series title or TVDB ID (e.g. Severance, Silo, The Bear, Fallout)..."
                  className="flex-1 px-3.5 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500"
                />
                <button
                  type="submit"
                  disabled={lookupLoading}
                  className="px-4 py-2.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl transition"
                >
                  {lookupLoading ? 'Searching...' : 'Lookup'}
                </button>
              </form>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div>
                  <label className="block text-[11px] font-medium text-slate-400 mb-1">
                    Root Folder
                  </label>
                  <select
                    value={selectedRootFolder}
                    onChange={(e) => setSelectedRootFolder(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    {rootFolders.map((rf) => (
                      <option key={rf.id} value={rf.path}>
                        {rf.path} ({rf.mediaType})
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-[11px] font-medium text-slate-400 mb-1">
                    Quality Profile
                  </label>
                  <select
                    value={selectedProfileId}
                    onChange={(e) => setSelectedProfileId(Number(e.target.value))}
                    className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    {profiles.map((p) => (
                      <option key={p.id} value={p.id}>
                        {p.name}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto p-5 space-y-3">
              {lookupResults.map((item, idx) => (
                <div
                  key={`${item.tvdbId || idx}-${item.title}`}
                  className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex items-start justify-between gap-4"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-bold text-white">{item.title}</h4>
                      <span className="text-xs font-mono text-indigo-400">({item.year})</span>
                      {item.network && (
                        <span className="text-[11px] px-2 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300">
                          {item.network}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-slate-400 line-clamp-2">{item.overview}</p>
                    <div className="text-[11px] font-mono text-slate-500">
                      TVDB: {item.tvdbId} • Status: {item.status || 'continuing'}
                    </div>
                  </div>
                  <button
                    onClick={() => handleAddSeries(item)}
                    className="px-3.5 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shrink-0 transition"
                  >
                    + Add Series
                  </button>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
