import React from 'react';
import {
  ArrowLeft,
  CheckCircle2,
  Clock,
  Film,
  Folder,
  Magnet,
  Search,
  Trash2,
} from 'lucide-react';
import type { Movie, ReleaseSearchRequest } from '../types';
import { formatBytes } from '../api';
import type { ManualTorrentTarget } from './ManualTorrentForm';

interface MovieDetailPanelProps {
  movie: Movie;
  posterUrl: string | null;
  onBack: () => void;
  onDelete: () => void;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onManualTorrent: (target: ManualTorrentTarget) => void;
}

export const MovieDetailPanel: React.FC<MovieDetailPanelProps> = ({
  movie,
  posterUrl,
  onBack,
  onDelete,
  onSearchReleases,
  onManualTorrent,
}) => {
  const searchMovie = () => {
    onSearchReleases({
      query: `${movie.title} ${movie.year || ''}`.trim(),
      type: 'movie',
      movieId: movie.id,
      tmdbId: movie.tmdbId || undefined,
      contextLabel: movie.title,
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
                title: movie.title,
                mediaType: 'movie',
                movieId: movie.id,
              })
            }
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-amber-500/15 text-amber-300 border border-amber-500/30 text-xs font-semibold transition"
          >
            <Magnet className="w-3.5 h-3.5" />
            Add torrent
          </button>
          <button
            onClick={searchMovie}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold transition"
          >
            <Search className="w-3.5 h-3.5" />
            Interactive Search
          </button>
          <button
            onClick={onDelete}
            className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
            title="Delete movie"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div className="p-5 flex flex-col sm:flex-row gap-5">
        <div className="w-40 h-60 mx-auto sm:mx-0 rounded-lg bg-gradient-to-br from-emerald-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center">
          {posterUrl ? (
            <img src={posterUrl} alt={movie.title} className="w-full h-full object-cover" />
          ) : (
            <Film className="w-10 h-10 text-emerald-400/60" />
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 mb-2">
            {movie.tmdbId > 0 && (
              <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                TMDB #{movie.tmdbId}
              </span>
            )}
            {movie.imdbId && (
              <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-slate-800 text-slate-300 border border-slate-700">
                {movie.imdbId}
              </span>
            )}
            {movie.monitored && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                Monitored
              </span>
            )}
            {movie.hasFile ? (
              <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                <CheckCircle2 className="w-3 h-3" /> Downloaded
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 text-xs px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30">
                <Clock className="w-3 h-3" /> Missing
              </span>
            )}
            {movie.year > 0 && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                {movie.year}
              </span>
            )}
            {movie.studio && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-emerald-300 border border-slate-700">
                {movie.studio}
              </span>
            )}
            {movie.sizeOnDisk > 0 && (
              <span className="text-xs px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                {formatBytes(movie.sizeOnDisk)}
              </span>
            )}
          </div>
          <h2 className="text-2xl font-bold text-white">{movie.title}</h2>
          {movie.originalTitle && movie.originalTitle !== movie.title && (
            <p className="text-sm text-slate-400 mt-1">{movie.originalTitle}</p>
          )}
          <p className="mt-3 text-sm text-slate-300 leading-relaxed">
            {movie.overview || 'No overview available.'}
          </p>
          <div className="mt-3 flex items-center gap-1.5 text-xs font-mono text-slate-500 break-all">
            <Folder className="w-3.5 h-3.5 shrink-0" />
            <span>{movie.path || `${movie.rootFolderPath}/${movie.title}`}</span>
          </div>
        </div>
      </div>
    </div>
  );
};
