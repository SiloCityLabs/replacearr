import React, { useEffect, useMemo, useState } from 'react';
import {
  ArrowLeft,
  Calendar,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Folder,
  Magnet,
  Radio,
  Search,
  Trash2,
  Tv,
} from 'lucide-react';
import type { AppConfig, Episode, ReleaseSearchRequest, Series } from '../types';
import { formatAppDate } from '../uiPrefs';
import type { ManualTorrentTarget } from './ManualTorrentForm';

interface SeriesDetailPanelProps {
  series: Series;
  episodes: Episode[];
  loading: boolean;
  appUi?: AppConfig['ui'] | null;
  posterUrl: string | null;
  onBack: () => void;
  onDelete: () => void;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onManualTorrent: (target: ManualTorrentTarget) => void;
}

function seasonLabel(n: number): string {
  return n === 0 ? 'Specials' : `Season ${n}`;
}

export const SeriesDetailPanel: React.FC<SeriesDetailPanelProps> = ({
  series,
  episodes,
  loading,
  appUi,
  posterUrl,
  onBack,
  onDelete,
  onSearchReleases,
  onManualTorrent,
}) => {
  const seasons = useMemo(() => {
    const map = new Map<number, Episode[]>();
    for (const ep of episodes) {
      const list = map.get(ep.seasonNumber) || [];
      list.push(ep);
      map.set(ep.seasonNumber, list);
    }
    return [...map.entries()]
      .sort((a, b) => a[0] - b[0])
      .map(([seasonNumber, eps]) => ({
        seasonNumber,
        episodes: eps.sort((a, b) => a.episodeNumber - b.episodeNumber),
        have: eps.filter((e) => e.hasFile).length,
        total: eps.length,
      }));
  }, [episodes]);

  const [expanded, setExpanded] = useState<Record<number, boolean>>({});
  const [expandedReady, setExpandedReady] = useState(false);

  useEffect(() => {
    if (expandedReady || seasons.length === 0) return;
    const initial: Record<number, boolean> = {};
    for (const s of seasons) {
      if (s.seasonNumber > 0) initial[s.seasonNumber] = true;
    }
    if (Object.keys(initial).length === 0) {
      initial[seasons[0].seasonNumber] = true;
    }
    setExpanded(initial);
    setExpandedReady(true);
  }, [seasons, expandedReady]);

  const have = episodes.filter((e) => e.hasFile).length;
  const total = episodes.length;

  const searchSeries = () => {
    onSearchReleases({
      query: series.title,
      type: 'tvsearch',
      seriesId: series.id,
      tvdbId: series.tvdbId || undefined,
      contextLabel: series.title,
    });
  };

  const searchSeason = (seasonNumber: number) => {
    // Pass title + season only; backend plans pack vs episode queries and season aliases.
    onSearchReleases({
      query: series.title,
      type: 'tvsearch',
      seriesId: series.id,
      tvdbId: series.tvdbId || undefined,
      season: seasonNumber,
      contextLabel: `${series.title} ${seasonLabel(seasonNumber)}`,
    });
  };

  const searchEpisode = (ep: Episode) => {
    const code = `S${String(ep.seasonNumber).padStart(2, '0')}E${String(ep.episodeNumber).padStart(2, '0')}`;
    onSearchReleases({
      query: `${series.title} ${code}`,
      type: 'tvsearch',
      seriesId: series.id,
      episodeId: ep.id,
      tvdbId: series.tvdbId || undefined,
      season: ep.seasonNumber,
      episode: ep.episodeNumber,
      contextLabel: `${series.title} ${code}`,
    });
  };

  return (
    <div className="min-h-[70vh] flex flex-col bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
      <div className="p-4 sm:p-5 border-b border-slate-800 flex flex-wrap items-center justify-between gap-3 shrink-0">
        <button
          onClick={onBack}
          className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 text-slate-200 hover:text-white text-xs font-semibold transition"
        >
          <ArrowLeft className="w-4 h-4" />
          Back to library
        </button>
        <div className="flex flex-wrap items-center gap-2">
          <button
            onClick={() =>
              onManualTorrent({
                title: series.title,
                mediaType: 'tv',
                seriesId: series.id,
              })
            }
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/15 text-amber-300 border border-amber-500/30 text-xs font-semibold transition"
          >
            <Magnet className="w-3.5 h-3.5" />
            Add torrent
          </button>
          <button
            onClick={searchSeries}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold transition"
          >
            <Search className="w-3.5 h-3.5" />
            Search series
          </button>
          <button
            onClick={onDelete}
            className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
            title="Delete series"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div className="p-5 border-b border-slate-800 flex flex-col sm:flex-row gap-5">
        <div className="w-36 h-52 mx-auto sm:mx-0 rounded-lg bg-gradient-to-br from-indigo-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center">
          {posterUrl ? (
            <img src={posterUrl} alt={series.title} className="w-full h-full object-cover" />
          ) : (
            <Tv className="w-10 h-10 text-indigo-400/60" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 mb-2">
            <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
              TVDB #{series.tvdbId || series.id}
            </span>
            {series.imdbId && (
              <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
                {series.imdbId}
              </span>
            )}
            {series.monitored && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                Monitored
              </span>
            )}
            <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
              {series.status || 'continuing'}
            </span>
            {total > 0 && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                {have}/{total} episodes
              </span>
            )}
            {series.network && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-indigo-300 border border-slate-700">
                {series.network}
              </span>
            )}
            {series.year > 0 && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                {series.year}
              </span>
            )}
          </div>
          <h2 className="text-2xl font-bold text-white">{series.title}</h2>
          <p className="mt-3 text-sm text-slate-300 leading-relaxed">
            {series.overview || 'No overview available.'}
          </p>
          <div className="mt-3 flex items-center gap-1.5 text-xs font-mono text-slate-500 break-all">
            <Folder className="w-3.5 h-3.5 shrink-0" />
            <span>{series.path || `${series.rootFolderPath}/${series.title}`}</span>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-5 space-y-3">
        {loading ? (
          <div className="py-12 text-center text-sm text-slate-400">Loading episode guide…</div>
        ) : seasons.length === 0 ? (
          <div className="py-12 text-center text-sm text-slate-400">
            No episodes recorded for this series yet.
          </div>
        ) : (
          seasons.map((season) => {
            const open = expanded[season.seasonNumber] ?? false;
            return (
              <div
                key={season.seasonNumber}
                className="rounded-xl border border-slate-800 bg-slate-950/60 overflow-hidden"
              >
                <div className="flex items-center gap-2 px-4 py-3 bg-slate-900/80">
                  <button
                    type="button"
                    onClick={() =>
                      setExpanded((prev) => ({
                        ...prev,
                        [season.seasonNumber]: !open,
                      }))
                    }
                    className="flex-1 flex items-center gap-2 text-left min-w-0"
                  >
                    {open ? (
                      <ChevronDown className="w-4 h-4 text-slate-400 shrink-0" />
                    ) : (
                      <ChevronRight className="w-4 h-4 text-slate-400 shrink-0" />
                    )}
                    <span className="text-sm font-semibold text-white">
                      {seasonLabel(season.seasonNumber)}
                    </span>
                    <span className="text-xs text-slate-400">
                      {season.have}/{season.total}
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={() => searchSeason(season.seasonNumber)}
                    className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-indigo-500/15 hover:bg-indigo-500/25 text-indigo-300 border border-indigo-500/30 text-[11px] font-medium transition"
                  >
                    <Radio className="w-3 h-3" />
                    Search season
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      onManualTorrent({
                        title: `${series.title} S${String(season.seasonNumber).padStart(2, '0')}`,
                        mediaType: 'tv',
                        seriesId: series.id,
                        seasonNumber: season.seasonNumber,
                      })
                    }
                    className="p-1.5 rounded-lg bg-amber-500/15 text-amber-300 border border-amber-500/30"
                    title="Add torrent for season"
                  >
                    <Magnet className="w-3.5 h-3.5" />
                  </button>
                </div>

                {open && (
                  <div className="divide-y divide-slate-800/80">
                    {season.episodes.map((ep) => (
                      <div
                        key={ep.id}
                        className="px-4 py-3 flex items-center justify-between gap-3 hover:bg-slate-900/40"
                      >
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <span className="text-xs font-mono font-semibold text-indigo-300 w-8">
                              {ep.episodeNumber}
                            </span>
                            <span className="text-sm font-medium text-white truncate">
                              {ep.title || `Episode ${ep.episodeNumber}`}
                            </span>
                            {ep.hasFile ? (
                              <span className="inline-flex items-center gap-1 text-[11px] text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded">
                                <CheckCircle2 className="w-3 h-3" /> Downloaded
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1 text-[11px] text-amber-400 bg-amber-500/10 px-2 py-0.5 rounded">
                                <Clock className="w-3 h-3" /> Missing
                              </span>
                            )}
                          </div>
                          <p className="text-xs text-slate-500 mt-1 line-clamp-1 pl-8">
                            {ep.overview ||
                              (ep.airDate
                                ? `Aired ${formatAppDate(ep.airDate, appUi)}`
                                : 'Air date TBA')}
                          </p>
                        </div>
                        <div className="flex items-center gap-2 shrink-0">
                          <span className="text-xs font-mono text-slate-500 hidden md:inline-flex items-center gap-1">
                            <Calendar className="w-3 h-3" />
                            {ep.airDate ? formatAppDate(ep.airDate, appUi) : 'TBA'}
                          </span>
                          <button
                            onClick={() => searchEpisode(ep)}
                            className="px-2.5 py-1.5 rounded-lg bg-indigo-600/20 hover:bg-indigo-600 text-indigo-300 hover:text-white border border-indigo-500/30 text-xs font-medium transition"
                          >
                            Search
                          </button>
                          <button
                            onClick={() =>
                              onManualTorrent({
                                title: `${series.title} S${String(ep.seasonNumber).padStart(2, '0')}E${String(ep.episodeNumber).padStart(2, '0')}`,
                                mediaType: 'tv',
                                seriesId: series.id,
                                episodeId: ep.id,
                                seasonNumber: ep.seasonNumber,
                              })
                            }
                            className="p-1.5 rounded-lg bg-amber-500/15 hover:bg-amber-500/25 text-amber-300 border border-amber-500/30"
                            title="Add torrent"
                          >
                            <Magnet className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
};
