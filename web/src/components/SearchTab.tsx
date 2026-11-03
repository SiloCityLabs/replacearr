import React, { useEffect, useState } from 'react';
import { Search, Download, ArrowUpDown } from 'lucide-react';
import type { ReleaseResult } from '../types';
import { api, formatBytes } from '../api';

interface SearchTabProps {
  initialQuery?: string;
  initialType?: string;
  initialTvdbId?: number;
  initialTmdbId?: number;
  initialSeason?: number;
  initialEpisode?: number;
  contextLabel?: string;
  searchNonce?: number;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SearchTab: React.FC<SearchTabProps> = ({
  initialQuery = '',
  initialType = 'search',
  initialTvdbId,
  initialTmdbId,
  initialSeason,
  initialEpisode,
  contextLabel = '',
  searchNonce = 0,
  onRefresh,
  onNotify,
}) => {
  const [query, setQuery] = useState(initialQuery);
  const [searchType, setSearchType] = useState(initialType || 'search');
  const [results, setResults] = useState<ReleaseResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [grabbingGuid, setGrabbingGuid] = useState<string | null>(null);
  const [season, setSeason] = useState(
    initialSeason != null && initialSeason >= 0 ? String(initialSeason) : ''
  );
  const [episode, setEpisode] = useState(
    initialEpisode != null && initialEpisode >= 0 ? String(initialEpisode) : ''
  );
  const [tvdbId, setTvdbId] = useState<number | undefined>(initialTvdbId);
  const [tmdbId, setTmdbId] = useState<number | undefined>(initialTmdbId);
  const [context, setContext] = useState(contextLabel);

  const runSearch = async (
    q: string,
    t: string,
    opts?: { tvdbId?: number; tmdbId?: number; season?: number; episode?: number }
  ) => {
    const trimmed = q.trim();
    if (!trimmed) return;
    setSearching(true);
    try {
      const seasonNum =
        opts?.season != null
          ? opts.season
          : season.trim() === ''
            ? undefined
            : Number(season);
      const episodeNum =
        opts?.episode != null
          ? opts.episode
          : episode.trim() === ''
            ? undefined
            : Number(episode);
      const res = await api.searchReleases(trimmed, t, {
        season: Number.isFinite(seasonNum as number) ? seasonNum : undefined,
        episode: Number.isFinite(episodeNum as number) ? episodeNum : undefined,
        tvdbId: opts?.tvdbId ?? tvdbId,
        tmdbId: opts?.tmdbId ?? tmdbId,
      });
      setResults(res);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Release search failed', true);
    } finally {
      setSearching(false);
    }
  };

  useEffect(() => {
    if (initialQuery) {
      setQuery(initialQuery);
      setSearchType(initialType || 'search');
      setTvdbId(initialTvdbId);
      setTmdbId(initialTmdbId);
      setContext(contextLabel || '');
      if (initialSeason != null && initialSeason >= 0) setSeason(String(initialSeason));
      if (initialEpisode != null && initialEpisode >= 0) setEpisode(String(initialEpisode));
      void runSearch(initialQuery, initialType || 'search', {
        tvdbId: initialTvdbId,
        tmdbId: initialTmdbId,
        season: initialSeason,
        episode: initialEpisode,
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initialQuery, initialType, initialTvdbId, initialTmdbId, searchNonce]);

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
      onNotify(`Queued "${rel.title}" with your download client`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to grab release', true);
    } finally {
      setGrabbingGuid(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-sky-500/15 text-sky-400 border border-sky-500/30">
            <Search className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-white">Release Search</h2>
            <p className="text-xs text-slate-400">
              Indexer releases only — use Add Series / Add Movie for TVDB / TMDB / IMDb lookup
            </p>
            {context ? (
              <p className="mt-1 inline-flex items-center px-2 py-0.5 rounded-md bg-sky-500/15 border border-sky-500/30 text-[11px] text-sky-300">
                Searching releases for {context}
                {tvdbId ? ` · tvdb:${tvdbId}` : ''}
                {tmdbId ? ` · tmdb:${tmdbId}` : ''}
              </p>
            ) : null}
          </div>
        </div>
      </div>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void runSearch(query, searchType);
        }}
        className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 flex flex-col gap-3"
      >
        <div className="flex flex-col sm:flex-row gap-3">
          <div className="relative flex-1">
            <Search className="w-4 h-4 text-slate-400 absolute left-3.5 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search releases across configured indexers..."
              className="w-full pl-10 pr-4 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white placeholder-slate-500 focus:outline-none focus:border-sky-500"
            />
          </div>

          <select
            value={searchType}
            onChange={(e) => setSearchType(e.target.value)}
            className="px-3.5 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-slate-200 focus:outline-none focus:border-sky-500"
          >
            <option value="search">All Categories (search)</option>
            <option value="tvsearch">TV Episodes / Packs (tvsearch)</option>
            <option value="movie">Movies (movie)</option>
          </select>

          <button
            type="submit"
            disabled={searching || !query.trim()}
            className="px-5 py-2.5 bg-sky-500 hover:bg-sky-400 disabled:opacity-50 text-slate-950 font-bold text-xs rounded-lg transition shrink-0"
          >
            {searching ? 'Querying Indexers...' : 'Search'}
          </button>
        </div>

        {searchType === 'tvsearch' && (
          <div className="flex flex-wrap gap-3">
            <label className="inline-flex items-center gap-2 text-xs text-slate-400">
              <span>Season</span>
              <input
                type="number"
                min={0}
                value={season}
                onChange={(e) => setSeason(e.target.value)}
                placeholder="S"
                className="w-20 px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
              />
            </label>
            <label className="inline-flex items-center gap-2 text-xs text-slate-400">
              <span>Episode</span>
              <input
                type="number"
                min={0}
                value={episode}
                onChange={(e) => setEpisode(e.target.value)}
                placeholder="E"
                className="w-20 px-2 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
              />
            </label>
          </div>
        )}
      </form>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-xs text-slate-400 min-w-0">
          <span className="inline-flex items-center gap-1.5 shrink-0">
            <ArrowUpDown className="w-3.5 h-3.5 text-sky-400" />
            Showing {results.length} release results
          </span>
          <span className="font-mono text-[11px] truncate min-w-0">
            GET /api/v1/search?q={encodeURIComponent(query || '')}&type={searchType}
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
                    {searching
                      ? 'Searching…'
                      : 'No releases yet. Enter a query and search across your indexers.'}
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
                        {rel.quality || '—'}
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
                        onClick={() => void handleGrab(rel)}
                        disabled={grabbingGuid === (rel.guid || rel.title)}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-semibold transition"
                      >
                        <Download className="w-3.5 h-3.5" />
                        <span>
                          {grabbingGuid === (rel.guid || rel.title) ? 'Grabbing...' : 'Grab'}
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
  );
};
