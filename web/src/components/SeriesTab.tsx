import React, { useEffect, useState } from 'react';
import {
  Tv,
  Plus,
  Search,
  Folder,
  X,
  Sparkles,
} from 'lucide-react';
import type { AppConfig, Episode, QualityProfile, ReleaseSearchRequest, RootFolder, Series } from '../types';
import { api, truncateText } from '../api';
import { useRovingFocus } from '../htpc/useRovingFocus';
import { ManualTorrentForm, type ManualTorrentTarget } from './ManualTorrentForm';
import { SeriesDetailPanel } from './SeriesDetailPanel';
import { useConfirm } from './modals/ConfirmModal';

interface SeriesTabProps {
  series: Series[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onNotify: (msg: string, isError?: boolean) => void;
  appUi?: AppConfig['ui'] | null;
  openAddPanel?: boolean;
  openAddNonce?: number;
  onAddPanelOpened?: () => void;
}

export const SeriesTab: React.FC<SeriesTabProps> = ({
  series,
  profiles,
  rootFolders,
  onRefresh,
  onSearchReleases,
  onNotify,
  appUi,
  openAddPanel = false,
  openAddNonce = 0,
  onAddPanelOpened,
}) => {
  const { confirm, dialog } = useConfirm();
  const tvRoots = rootFolders.filter((r) => r.mediaType === 'tv');
  const availableRoots = tvRoots.length > 0 ? tvRoots : rootFolders;

  const [filterText, setFilterText] = useState('');
  const [showAddPanel, setShowAddPanel] = useState(false);
  const [lookupTerm, setLookupTerm] = useState('');
  const [lookupResults, setLookupResults] = useState<Series[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(profiles[0]?.id || 1);
  const [selectedRootFolder, setSelectedRootFolder] = useState<string>(
    availableRoots[0]?.path || ''
  );

  const [activeSeries, setActiveSeries] = useState<Series | null>(null);
  const [episodes, setEpisodes] = useState<Episode[]>([]);
  const [episodesLoading, setEpisodesLoading] = useState(false);
  const [manualTarget, setManualTarget] = useState<ManualTorrentTarget | null>(null);

  const filteredSeries = series.filter(
    (s) =>
      s.title.toLowerCase().includes(filterText.toLowerCase()) ||
      (s.network || '').toLowerCase().includes(filterText.toLowerCase())
  );

  const openSeriesDetail = async (s: Series) => {
    setActiveSeries(s);
    setEpisodes([]);
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

  const gridFocus = useRovingFocus({
    itemCount: filteredSeries.length,
    orientation: 'grid',
    columns: 3,
    onActivate: (index) => {
      const s = filteredSeries[index];
      if (s) void openSeriesDetail(s);
    },
  });

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
    if (!selectedRootFolder) {
      onNotify('Add a TV root folder in setup or settings first', true);
      return;
    }
    try {
      const created = await api.addSeries({
        ...candidate,
        qualityProfileId: selectedProfileId || 1,
        rootFolderPath: selectedRootFolder,
        monitored: true,
        seasonFolder: true,
      });
      onNotify(`Added "${created.title}" to TV Series library`);
      setShowAddPanel(false);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add series', true);
    }
  };

  const handleDeleteSeries = async (s: Series) => {
    const ok = await confirm({
      title: 'Remove series',
      message: `Remove "${s.title}" from the TV library?`,
      confirmLabel: 'Remove',
    });
    if (!ok) return;
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

  const getPosterUrl = (s: Series): string | null => {
    const poster = s.images?.find((img) => img.coverType === 'poster') || s.images?.[0];
    return poster?.url || poster?.remoteUrl || null;
  };

  const startAddPanel = () => {
    setShowAddPanel(true);
    setSelectedRootFolder(availableRoots[0]?.path || '');
    setLookupTerm('');
    setLookupResults([]);
  };

  useEffect(() => {
    if (openAddPanel) {
      startAddPanel();
      onAddPanelOpened?.();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [openAddNonce]);

  if (activeSeries) {
    return (
      <>
        {dialog}
        <SeriesDetailPanel
          series={activeSeries}
          episodes={episodes}
          loading={episodesLoading}
          appUi={appUi}
          posterUrl={getPosterUrl(activeSeries)}
          onBack={() => {
            setActiveSeries(null);
            setEpisodes([]);
          }}
          onDelete={() => void handleDeleteSeries(activeSeries)}
          onSearchReleases={onSearchReleases}
          onManualTorrent={setManualTarget}
          onNotify={onNotify}
        />
        {manualTarget && (
          <ManualTorrentForm
            target={manualTarget}
            onClose={() => setManualTarget(null)}
            onNotify={onNotify}
            onSuccess={onRefresh}
          />
        )}
      </>
    );
  }

  if (showAddPanel) {
    return (
      <div className="min-h-[70vh] flex flex-col bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <div className="p-5 border-b border-slate-800 flex items-center justify-between shrink-0">
          <div className="flex items-center gap-2.5">
            <Sparkles className="w-5 h-5 text-indigo-400" />
            <div>
              <h3 className="text-base font-bold text-white">Add TV Series</h3>
              <p className="text-xs text-slate-400">
                Lookup on TVDB / TMDB / IMDb — not torrent search
              </p>
            </div>
          </div>
          <button
            onClick={() => setShowAddPanel(false)}
            className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 border-b border-slate-800 space-y-4 bg-slate-950/50 shrink-0">
          <form onSubmit={(e) => void handleLookup(e)} className="flex gap-2">
            <input
              type="text"
              value={lookupTerm}
              onChange={(e) => setLookupTerm(e.target.value)}
              placeholder="Series title, tvdb:12345, or imdb:tt…"
              className="flex-1 px-3.5 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500"
              autoFocus
            />
            <button
              type="submit"
              disabled={lookupLoading}
              className="px-4 py-2.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-xl transition"
            >
              {lookupLoading ? 'Looking up…' : 'Lookup'}
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
                {availableRoots.length === 0 ? (
                  <option value="">No root folders configured</option>
                ) : (
                  availableRoots.map((rf) => (
                    <option key={rf.id} value={rf.path}>
                      {rf.path} ({rf.mediaType})
                    </option>
                  ))
                )}
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
          {lookupResults.length === 0 && !lookupLoading ? (
            <p className="text-sm text-slate-400 text-center py-12">
              Search for a series title to see lookup results.
            </p>
          ) : null}
          {lookupResults.map((item, idx) => (
            <div
              key={`${item.tvdbId || idx}-${item.title}`}
              className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex items-start justify-between gap-4"
            >
              <div className="space-y-1 min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h4 className="text-sm font-bold text-white">{item.title}</h4>
                  <span className="text-xs font-mono text-indigo-400">({item.year})</span>
                  {item.network && (
                    <span className="text-[11px] px-2 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300">
                      {item.network}
                    </span>
                  )}
                </div>
                <p className="text-xs text-slate-400">
                  {truncateText(item.overview || '')}
                </p>
                <div className="text-[11px] font-mono text-slate-500">
                  TVDB: {item.tvdbId} • Status: {item.status || 'continuing'}
                </div>
              </div>
              <button
                onClick={() => void handleAddSeries(item)}
                className="px-3.5 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shrink-0 transition"
              >
                + Add Series
              </button>
            </div>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-indigo-500/15 text-indigo-400 border border-indigo-500/30">
            <Tv className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-white">
              TV Library - {series.length} series
            </h2>
          </div>
        </div>

        <div className="flex flex-col min-[480px]:flex-row min-[480px]:items-center gap-2">
          <div className="relative flex-1 min-w-0">
            <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={filterText}
              onChange={(e) => setFilterText(e.target.value)}
              placeholder="Search library..."
              className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white placeholder-slate-500 focus:outline-none focus:border-indigo-500"
            />
          </div>
          <button
            onClick={startAddPanel}
            className="inline-flex items-center justify-center gap-2 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg shadow-lg shadow-indigo-600/20 transition shrink-0"
          >
            <Plus className="w-4 h-4" />
            <span>Add Series</span>
          </button>
        </div>
      </div>

      {filteredSeries.length === 0 ? (
        <div className="bg-slate-900/60 border border-dashed border-slate-800 rounded-2xl p-12 text-center">
          <Tv className="w-10 h-10 text-slate-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-slate-200">No TV Series Found</h3>
          <p className="text-xs text-slate-400 mt-1 mb-5">
            Look up a series by title or TVDB ID to start monitoring episodes.
          </p>
          <button
            onClick={startAddPanel}
            className="inline-flex items-center gap-2 px-4 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Lookup & Add Series</span>
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
          {filteredSeries.map((s, index) => {
            const posterUrl = getPosterUrl(s);
            return (
              <div
                key={s.id}
                {...gridFocus.itemProps(index)}
                role="button"
                onClick={() => void openSeriesDetail(s)}
                className="group bg-slate-900/90 border border-slate-800 hover:border-indigo-500/40 rounded-xl overflow-hidden flex flex-col justify-between transition shadow-lg outline-none cursor-pointer"
              >
                <div className="p-4 flex flex-row gap-3 sm:gap-4">
                  <div className="w-20 h-28 sm:w-24 sm:h-36 rounded-lg bg-gradient-to-br from-indigo-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center relative">
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

                  <div className="flex-1 min-w-0">
                    <h3 className="text-base font-bold text-white truncate">{s.title}</h3>
                    <div className="mt-1">
                      <span
                        className={`inline-flex text-[10px] font-mono uppercase px-2 py-0.5 rounded-full ${
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

                    <p className="mt-2 text-xs text-slate-400 leading-relaxed hidden md:block">
                      {truncateText(
                        s.overview || 'Monitored TV series in Replacearr unified database.'
                      )}
                    </p>

                    <div className="mt-3 flex items-center gap-1.5 text-[11px] font-mono text-slate-500 truncate">
                      <Folder className="w-3.5 h-3.5 text-slate-500 shrink-0" />
                      <span className="truncate">{s.path || `${s.rootFolderPath}/${s.title}`}</span>
                    </div>
                  </div>
                </div>

              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
