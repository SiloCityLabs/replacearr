import React, { useEffect, useState } from 'react';
import { Search, Download, ArrowUpDown, X } from 'lucide-react';
import type { ReleaseResult, ReleaseSearchRequest } from '../types';
import { api, formatBytes } from '../api';

interface InteractiveSearchModalProps {
  request: ReleaseSearchRequest;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const InteractiveSearchModal: React.FC<InteractiveSearchModalProps> = ({
  request,
  onClose,
  onRefresh,
  onNotify,
}) => {
  const seriesId = request.seriesId;
  const movieId = request.movieId;
  const artistId = request.artistId;
  const albumId = request.albumId;
  const authorId = request.authorId;
  const bookId = request.bookId;
  const editionId = request.editionId;
  const hasLibrary =
    (seriesId != null && seriesId > 0) ||
    (movieId != null && movieId > 0) ||
    (artistId != null && artistId > 0) ||
    (albumId != null && albumId > 0) ||
    (authorId != null && authorId > 0) ||
    (bookId != null && bookId > 0) ||
    (editionId != null && editionId > 0);

  const searchType =
    request.type ||
    (editionId || bookId ? 'book' : artistId || albumId ? 'music' : movieId ? 'movie' : 'tvsearch');
  const [query] = useState(request.query || request.contextLabel || '');
  const [results, setResults] = useState<ReleaseResult[]>([]);
  const [searching, setSearching] = useState(false);
  const [grabbingGuid, setGrabbingGuid] = useState<string | null>(null);

  useEffect(() => {
    if (!hasLibrary || !query.trim()) {
      onNotify('Interactive Search requires a library item', true);
      onClose();
      return;
    }
    let cancelled = false;
    const run = async () => {
      setSearching(true);
      try {
        const res = await api.searchReleases(query.trim(), searchType, {
          season: request.season,
          episode: request.episode,
          tvdbId: request.tvdbId,
          tmdbId: request.tmdbId,
          artistId,
          albumId,
          authorId,
          bookId,
          editionId,
        });
        if (!cancelled) setResults(res);
      } catch (err) {
        if (!cancelled) {
          onNotify(err instanceof Error ? err.message : 'Release search failed', true);
        }
      } finally {
        if (!cancelled) setSearching(false);
      }
    };
    void run();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [request.query, request.seriesId, request.movieId, request.artistId, request.albumId, request.bookId, request.editionId]);

  const handleGrab = async (rel: ReleaseResult) => {
    setGrabbingGuid(rel.guid || rel.title);
    const label = request.contextLabel || query;
    const mediaType = editionId || bookId
      ? searchType === 'audiobook'
        ? 'audiobook'
        : 'ebook'
      : artistId || albumId
        ? 'music'
        : movieId
          ? 'movie'
          : 'tv';
    try {
      await api.grabRelease({
        title: rel.title,
        downloadUrl: rel.downloadUrl,
        protocol: rel.protocol || 'torrent',
        indexer: rel.indexer || 'Replacearr',
        size: rel.size || 1500000000,
        mediaType,
        seriesId,
        movieId,
        episodeId: request.episodeId,
        seasonNumber: request.season,
        artistId,
        albumId,
        authorId,
        bookId,
        editionId,
      });
      onNotify(`Queued "${rel.title}" for ${label}`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to grab release', true);
    } finally {
      setGrabbingGuid(null);
    }
  };

  if (!hasLibrary) return null;

  const title = request.contextLabel || query;

  return (
    <div className="fixed inset-0 z-50 flex items-end sm:items-center justify-center bg-black/70 p-0 sm:p-4">
      <div className="w-full sm:max-w-5xl max-h-[92vh] flex flex-col bg-slate-900 border border-slate-800 rounded-t-2xl sm:rounded-2xl overflow-hidden shadow-2xl">
        <div className="p-4 border-b border-slate-800 flex items-start justify-between gap-3 shrink-0">
          <div className="flex items-center gap-3 min-w-0">
            <div className="p-2.5 rounded-xl bg-sky-500/15 text-sky-400 border border-sky-500/30 shrink-0">
              <Search className="w-5 h-5" />
            </div>
            <div className="min-w-0">
              <h2 className="text-lg font-bold text-white truncate">
                Interactive Search — {title}
              </h2>
              <p className="text-xs text-slate-400">
                Indexer releases for this library item
                {request.tvdbId ? ` · tvdb:${request.tvdbId}` : ''}
                {request.tmdbId ? ` · tmdb:${request.tmdbId}` : ''}
                {request.season != null ? ` · S${String(request.season).padStart(2, '0')}` : ''}
                {request.episode != null ? `E${String(request.episode).padStart(2, '0')}` : ''}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white shrink-0"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="px-4 py-2 border-b border-slate-800 flex items-center justify-between gap-2 text-xs text-slate-400 shrink-0">
          <span className="inline-flex items-center gap-1.5">
            <ArrowUpDown className="w-3.5 h-3.5 text-sky-400" />
            {searching ? 'Searching indexers…' : `${results.length} release results`}
          </span>
          <button
            type="button"
            disabled={searching}
            onClick={() => {
              setSearching(true);
              void api
                .searchReleases(query.trim(), searchType, {
                  season: request.season,
                  episode: request.episode,
                  tvdbId: request.tvdbId,
                  tmdbId: request.tmdbId,
                })
                .then(setResults)
                .catch((err) =>
                  onNotify(err instanceof Error ? err.message : 'Release search failed', true)
                )
                .finally(() => setSearching(false));
            }}
            className="px-2.5 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
          >
            Refresh
          </button>
        </div>

        <div className="flex-1 overflow-auto">
          {results.length === 0 ? (
            <p className="py-10 text-center text-sm text-slate-400">
              {searching ? 'Searching…' : 'No releases found for this item.'}
            </p>
          ) : (
            <>
              <ul className="sm:hidden divide-y divide-slate-800/70">
                {results.map((rel, i) => {
                  const key = `${rel.guid || i}-${rel.title}`;
                  const busy = grabbingGuid === (rel.guid || rel.title);
                  return (
                    <li key={key} className="p-4 space-y-2">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span
                          className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded font-semibold ${
                            rel.protocol === 'usenet'
                              ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                              : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                          }`}
                        >
                          {rel.protocol || 'torrent'}
                        </span>
                        <span className="px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30 font-mono text-[11px]">
                          {rel.quality || '—'}
                        </span>
                        <span className="text-[11px] font-mono text-slate-400">{formatBytes(rel.size)}</span>
                      </div>
                      <p className="text-sm font-medium text-slate-100 break-words">{rel.title}</p>
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-[11px] font-mono text-slate-400 truncate">{rel.indexer}</span>
                        <button
                          type="button"
                          onClick={() => void handleGrab(rel)}
                          disabled={busy}
                          className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 text-white text-xs font-semibold shrink-0"
                        >
                          <Download className="w-3.5 h-3.5" />
                          {busy ? 'Grabbing…' : 'Grab'}
                        </button>
                      </div>
                    </li>
                  );
                })}
              </ul>
              <table className="hidden sm:table w-full text-left border-collapse">
                <thead className="sticky top-0 bg-slate-950/95">
                  <tr className="border-b border-slate-800 text-[11px] font-semibold uppercase tracking-wider text-slate-400">
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
                  {results.map((rel, i) => (
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
                          type="button"
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
                  ))}
                </tbody>
              </table>
            </>
          )}
        </div>
      </div>
    </div>
  );
};
