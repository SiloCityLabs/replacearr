import React, { useState } from 'react';
import {
  Film,
  Plus,
  Search,
  Trash2,
  CheckCircle2,
  Clock,
  Folder,
  X,
  Radio,
  Sparkles,
} from 'lucide-react';
import type { Movie, QualityProfile, RootFolder } from '../types';
import { api, formatBytes } from '../api';

interface MoviesTabProps {
  movies: Movie[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onSearchReleases: (query: string, type: string) => void;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const MoviesTab: React.FC<MoviesTabProps> = ({
  movies,
  profiles,
  rootFolders,
  onRefresh,
  onSearchReleases,
  onNotify,
}) => {
  const [filterText, setFilterText] = useState('');
  const [showAddModal, setShowAddModal] = useState(false);
  const [lookupTerm, setLookupTerm] = useState('');
  const [lookupResults, setLookupResults] = useState<Movie[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(profiles[0]?.id || 1);
  const [selectedRootFolder, setSelectedRootFolder] = useState<string>(
    rootFolders.find((r) => r.mediaType === 'movie')?.path || rootFolders[0]?.path || '/data/movies'
  );
  const [minAvailability, setMinAvailability] = useState('released');

  const filteredMovies = movies.filter(
    (m) =>
      m.title.toLowerCase().includes(filterText.toLowerCase()) ||
      (m.studio || '').toLowerCase().includes(filterText.toLowerCase())
  );

  const handleLookup = async (e?: React.FormEvent, customTerm?: string) => {
    if (e) e.preventDefault();
    const term = (customTerm ?? lookupTerm).trim();
    if (!term) return;
    setLookupLoading(true);
    try {
      const results = await api.lookupMovies(term);
      setLookupResults(results);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Movie lookup failed', true);
    } finally {
      setLookupLoading(false);
    }
  };

  const handleAddMovie = async (candidate: Partial<Movie>) => {
    try {
      const created = await api.addMovie({
        ...candidate,
        qualityProfileId: selectedProfileId || 1,
        rootFolderPath: selectedRootFolder || '/data/movies',
        minimumAvailability: minAvailability,
        monitored: true,
      });
      onNotify(`Added "${created.title}" to Movies library`);
      setShowAddModal(false);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add movie', true);
    }
  };

  const handleDeleteMovie = async (m: Movie) => {
    try {
      await api.deleteMovie(m.id);
      onNotify(`Removed "${m.title}" from library`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete movie', true);
    }
  };

  const getPosterUrl = (m: Movie): string | null => {
    const poster = m.images?.find((img) => img.coverType === 'poster') || m.images?.[0];
    return poster?.remoteUrl || poster?.url || null;
  };

  return (
    <div className="space-y-6">
      {/* Header & Controls */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
            <Film className="w-5 h-5" />
          </div>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-bold text-white">Movies Library</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 font-mono">
                Radarr v3 Compatible
              </span>
            </div>
            <p className="text-xs text-slate-400">
              {movies.length} movies monitored • TMDB & IMDb release automation
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
              placeholder="Filter movies..."
              className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white placeholder-slate-500 focus:outline-none focus:border-emerald-500"
            />
          </div>
          <button
            onClick={() => {
              setShowAddModal(true);
              if (lookupResults.length === 0) {
                setLookupTerm('Dune');
                void handleLookup(undefined, 'Dune');
              }
            }}
            className="inline-flex items-center justify-center gap-2 px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-lg shadow-lg shadow-emerald-600/20 transition shrink-0"
          >
            <Plus className="w-4 h-4" />
            <span>Add Movie</span>
          </button>
        </div>
      </div>

      {/* Movie Grid */}
      {filteredMovies.length === 0 ? (
        <div className="bg-slate-900/60 border border-dashed border-slate-800 rounded-2xl p-12 text-center">
          <Film className="w-10 h-10 text-slate-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-slate-200">No Movies in Library</h3>
          <p className="text-xs text-slate-400 mt-1 mb-5">
            Search TMDB via the Radarr lookup endpoint to add movies to Replacearr.
          </p>
          <button
            onClick={() => {
              setShowAddModal(true);
              setLookupTerm('Dune');
              void handleLookup(undefined, 'Dune');
            }}
            className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Lookup & Add Movie</span>
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
          {filteredMovies.map((m) => {
            const posterUrl = getPosterUrl(m);
            return (
              <div
                key={m.id}
                className="group bg-slate-900/90 border border-slate-800 hover:border-emerald-500/40 rounded-xl overflow-hidden flex flex-col justify-between transition shadow-lg"
              >
                <div className="p-4 flex gap-4">
                  {/* Poster */}
                  <div className="w-24 h-36 rounded-lg bg-gradient-to-br from-emerald-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center relative">
                    {posterUrl ? (
                      <img
                        src={posterUrl}
                        alt={m.title}
                        className="w-full h-full object-cover"
                        onError={(e) => {
                          (e.currentTarget as HTMLImageElement).style.display = 'none';
                        }}
                      />
                    ) : (
                      <Film className="w-8 h-8 text-emerald-400/60" />
                    )}
                    <span className="absolute bottom-1.5 right-1.5 text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-950/90 text-emerald-300 border border-emerald-500/30">
                      {m.year}
                    </span>
                  </div>

                  {/* Details */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-start justify-between gap-2">
                      <h3 className="text-base font-bold text-white truncate">{m.title}</h3>
                      {m.hasFile ? (
                        <span className="inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30 shrink-0">
                          <CheckCircle2 className="w-3 h-3" /> Downloaded
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30 shrink-0">
                          <Clock className="w-3 h-3" /> Monitored
                        </span>
                      )}
                    </div>

                    <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                      {m.studio && (
                        <span className="text-emerald-300 font-medium">{m.studio}</span>
                      )}
                      <span>•</span>
                      <span>{m.runtime || 120}m</span>
                      {m.sizeOnDisk > 0 && (
                        <>
                          <span>•</span>
                          <span>{formatBytes(m.sizeOnDisk)}</span>
                        </>
                      )}
                    </div>

                    <p className="mt-2 text-xs text-slate-400 line-clamp-3 leading-relaxed">
                      {m.overview || 'Monitored feature film in Replacearr unified database.'}
                    </p>

                    <div className="mt-3 flex items-center gap-1.5 text-[11px] font-mono text-slate-500 truncate">
                      <Folder className="w-3.5 h-3.5 text-slate-500 shrink-0" />
                      <span className="truncate">{m.path || `${m.rootFolderPath}/${m.title}`}</span>
                    </div>
                  </div>
                </div>

                {/* Footer */}
                <div className="px-4 py-3 bg-slate-950/70 border-t border-slate-800/80 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <span className="text-[11px] font-mono text-slate-500">
                      TMDB #{m.tmdbId || m.id}
                    </span>
                    <button
                      onClick={() => onSearchReleases(`${m.title} ${m.year}`, 'movie')}
                      className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border border-emerald-500/30 text-xs font-medium transition"
                    >
                      <Radio className="w-3.5 h-3.5" />
                      <span>Search Releases</span>
                    </button>
                  </div>

                  <button
                    onClick={() => handleDeleteMovie(m)}
                    title="Delete movie"
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

      {/* Add Movie Lookup Modal */}
      {showAddModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-3xl bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[calc(100dvh-2rem)] my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <div className="flex items-center gap-2.5">
                <Sparkles className="w-5 h-5 text-emerald-400" />
                <div>
                  <h3 className="text-base font-bold text-white">
                    Add Movie (/api/v1/movies/lookup)
                  </h3>
                  <p className="text-xs text-slate-400">
                    Search movie metadata and monitor releases automatically
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
                  placeholder="Enter movie title or TMDB/IMDb ID (e.g. Dune, Oppenheimer, Interstellar)..."
                  className="flex-1 px-3.5 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:border-emerald-500"
                />
                <button
                  type="submit"
                  disabled={lookupLoading}
                  className="px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-xl transition"
                >
                  {lookupLoading ? 'Searching...' : 'Lookup'}
                </button>
              </form>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
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
                <div>
                  <label className="block text-[11px] font-medium text-slate-400 mb-1">
                    Minimum Availability
                  </label>
                  <select
                    value={minAvailability}
                    onChange={(e) => setMinAvailability(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-900 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="released">Released</option>
                    <option value="inCinemas">In Cinemas</option>
                    <option value="announced">Announced</option>
                  </select>
                </div>
              </div>
            </div>

            <div className="flex-1 overflow-y-auto p-5 space-y-3">
              {lookupResults.map((item, idx) => (
                <div
                  key={`${item.tmdbId || idx}-${item.title}`}
                  className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex items-start justify-between gap-4"
                >
                  <div className="space-y-1">
                    <div className="flex items-center gap-2">
                      <h4 className="text-sm font-bold text-white">{item.title}</h4>
                      <span className="text-xs font-mono text-emerald-400">({item.year})</span>
                      {item.studio && (
                        <span className="text-[11px] px-2 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300">
                          {item.studio}
                        </span>
                      )}
                    </div>
                    <p className="text-xs text-slate-400 line-clamp-2">{item.overview}</p>
                    <div className="text-[11px] font-mono text-slate-500">
                      TMDB: {item.tmdbId} • IMDb: {item.imdbId || 'N/A'} • {item.runtime || 120} min
                    </div>
                  </div>
                  <button
                    onClick={() => handleAddMovie(item)}
                    className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shrink-0 transition"
                  >
                    + Add Movie
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
