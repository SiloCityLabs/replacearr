import React, { useEffect, useState } from 'react';
import {
  Film,
  Plus,
  Search,
  CheckCircle2,
  Clock,
  Folder,
  X,
  Sparkles,
} from 'lucide-react';
import type { Movie, QualityProfile, ReleaseSearchRequest, RootFolder, Tag } from '../types';
import { api, formatBytes, truncateText } from '../api';
import { useRovingFocus } from '../htpc/useRovingFocus';
import { ManualTorrentForm, type ManualTorrentTarget } from './ManualTorrentForm';
import { MovieDetailPanel } from './MovieDetailPanel';
import { useConfirm } from './modals/ConfirmModal';

interface MoviesTabProps {
  movies: Movie[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onSearchReleases: (req: ReleaseSearchRequest) => void;
  onNotify: (msg: string, isError?: boolean) => void;
  openAddPanel?: boolean;
  openAddNonce?: number;
  onAddPanelOpened?: () => void;
}

export const MoviesTab: React.FC<MoviesTabProps> = ({
  movies,
  profiles,
  rootFolders,
  onRefresh,
  onSearchReleases,
  onNotify,
  openAddPanel = false,
  openAddNonce = 0,
  onAddPanelOpened,
}) => {
  const { confirm, dialog } = useConfirm();
  const movieRoots = rootFolders.filter((r) => r.mediaType === 'movie');
  const availableRoots = movieRoots.length > 0 ? movieRoots : rootFolders;

  const [filterText, setFilterText] = useState('');
  const [showAddPanel, setShowAddPanel] = useState(false);
  const [lookupTerm, setLookupTerm] = useState('');
  const [lookupResults, setLookupResults] = useState<Movie[]>([]);
  const [lookupLoading, setLookupLoading] = useState(false);
  const [personFilter, setPersonFilter] = useState<{ name: string; tmdbId: number } | null>(null);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(profiles[0]?.id || 1);
  const [selectedRootFolder, setSelectedRootFolder] = useState<string>(
    availableRoots[0]?.path || ''
  );
  const [minAvailability, setMinAvailability] = useState('released');
  const [manualTarget, setManualTarget] = useState<ManualTorrentTarget | null>(null);
  const [activeMovie, setActiveMovie] = useState<Movie | null>(null);
  const [tags, setTags] = useState<Tag[]>([]);

  useEffect(() => {
    void api
      .getTags()
      .then(setTags)
      .catch(() => setTags([]));
  }, []);

  const filteredMovies = movies.filter(
    (m) =>
      m.title.toLowerCase().includes(filterText.toLowerCase()) ||
      (m.studio || '').toLowerCase().includes(filterText.toLowerCase())
  );

  const gridFocus = useRovingFocus({
    itemCount: filteredMovies.length,
    orientation: 'grid',
    columns: 3,
    onActivate: (index) => {
      const m = filteredMovies[index];
      if (m) setActiveMovie(m);
    },
  });

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

  const openAddForPerson = (person: { name: string; tmdbId: number }) => {
    if (!person.tmdbId) return;
    setActiveMovie(null);
    setShowAddPanel(true);
    setSelectedRootFolder(availableRoots[0]?.path || '');
    setPersonFilter({ name: person.name, tmdbId: person.tmdbId });
    setLookupTerm(person.name);
    setLookupResults([]);
    void handleLookup(undefined, `person:${person.tmdbId}`);
  };

  const handleAddMovie = async (candidate: Partial<Movie>) => {
    if (!selectedRootFolder) {
      onNotify('Add a movies root folder in setup or settings first', true);
      return;
    }
    try {
      const created = await api.addMovie({
        ...candidate,
        qualityProfileId: selectedProfileId || 1,
        rootFolderPath: selectedRootFolder,
        minimumAvailability: minAvailability,
        monitored: true,
      });
      onNotify(`Added "${created.title}" to Movies library`);
      setShowAddPanel(false);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add movie', true);
    }
  };

  const handleDeleteMovie = async (m: Movie) => {
    const fileCount = m.hasFile ? 1 : 0;
    const result = await confirm({
      title: 'Delete movie',
      message: `Are you sure you want to delete "${m.title}" from the Movies library?`,
      confirmLabel: 'Delete',
      checkboxes: [
        {
          id: 'addImportExclusion',
          label: 'Add List Exclusion',
          description: 'Prevent this movie from being re-added by import lists.',
          defaultChecked: false,
        },
        {
          id: 'deleteFiles',
          label: `Delete ${fileCount} Movie Files`,
          description: 'Permanently delete the movie files and folder from disk.',
          defaultChecked: false,
        },
      ],
    });
    if (!result) return;
    try {
      await api.deleteMovie(m.id, {
        addImportExclusion: result.addImportExclusion === true,
        deleteFiles: result.deleteFiles === true,
      });
      onNotify(`Removed "${m.title}" from library`);
      if (activeMovie?.id === m.id) {
        setActiveMovie(null);
      }
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete movie', true);
    }
  };

  const getPosterUrl = (m: Movie): string | null => {
    const poster = m.images?.find((img) => img.coverType === 'poster') || m.images?.[0];
    return poster?.url || poster?.remoteUrl || null;
  };

  const startAddPanel = () => {
    setShowAddPanel(true);
    setSelectedRootFolder(availableRoots[0]?.path || '');
    setPersonFilter(null);
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

  if (activeMovie) {
    return (
      <>
        {dialog}
        <MovieDetailPanel
          movie={activeMovie}
          posterUrl={getPosterUrl(activeMovie)}
          profiles={profiles}
          tags={tags}
          onBack={() => setActiveMovie(null)}
          onDelete={() => void handleDeleteMovie(activeMovie)}
          onSearchReleases={onSearchReleases}
          onManualTorrent={setManualTarget}
          onAddByPerson={openAddForPerson}
          onNotify={onNotify}
          onUpdated={(m) => {
            setActiveMovie(m);
            void onRefresh();
          }}
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
            <Sparkles className="w-5 h-5 text-emerald-400" />
            <div>
              <h3 className="text-base font-bold text-white">
                {personFilter ? `Add Movie — ${personFilter.name}` : 'Add Movie'}
              </h3>
              <p className="text-xs text-slate-400">
                {personFilter
                  ? 'Movies credited to this cast/crew member'
                  : 'Lookup on TMDB / IMDb — not torrent search'}
              </p>
            </div>
          </div>
          <button
            onClick={() => {
              setShowAddPanel(false);
              setPersonFilter(null);
            }}
            className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-5 border-b border-slate-800 space-y-4 bg-slate-950/50 shrink-0">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (personFilter) {
                void handleLookup(undefined, `person:${personFilter.tmdbId}`);
                return;
              }
              void handleLookup(e);
            }}
            className="flex gap-2"
          >
            <input
              type="text"
              value={lookupTerm}
              onChange={(e) => {
                setPersonFilter(null);
                setLookupTerm(e.target.value);
              }}
              placeholder="Movie title, person:12345, tmdb:12345, or imdb:tt…"
              className="flex-1 px-3.5 py-2.5 bg-slate-900 border border-slate-800 rounded-xl text-sm text-white placeholder-slate-500 focus:outline-none focus:border-emerald-500"
              autoFocus
            />
            <button
              type="submit"
              disabled={lookupLoading}
              className="px-4 py-2.5 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-xl transition"
            >
              {lookupLoading ? 'Looking up…' : 'Lookup'}
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
          {lookupLoading ? (
            <p className="text-sm text-slate-400 text-center py-12">Looking up…</p>
          ) : null}
          {lookupResults.length === 0 && !lookupLoading ? (
            <p className="text-sm text-slate-400 text-center py-12">
              {personFilter
                ? `No movies found for ${personFilter.name}.`
                : 'Search for a movie title to see lookup results.'}
            </p>
          ) : null}
          {lookupResults.map((item, idx) => (
            <div
              key={`${item.tmdbId || idx}-${item.title}`}
              className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex items-start justify-between gap-4"
            >
              <div className="space-y-1 min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <h4 className="text-sm font-bold text-white">{item.title}</h4>
                  <span className="text-xs font-mono text-emerald-400">({item.year})</span>
                  {item.studio && (
                    <span className="text-[11px] px-2 py-0.5 rounded bg-slate-900 border border-slate-800 text-slate-300">
                      {item.studio}
                    </span>
                  )}
                </div>
                <p className="text-xs text-slate-400">
                  {truncateText(item.overview || '')}
                </p>
                <div className="text-[11px] font-mono text-slate-500">
                  TMDB: {item.tmdbId} • IMDb: {item.imdbId || 'N/A'} • {item.runtime || 120} min
                </div>
              </div>
              <button
                onClick={() => void handleAddMovie(item)}
                className="px-3.5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shrink-0 transition"
              >
                + Add Movie
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
          <div className="p-2.5 rounded-xl bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
            <Film className="w-5 h-5" />
          </div>
          <div>
            <h2 className="text-lg font-bold text-white">
              Movies Library - {movies.length}{' '}
              {movies.length === 1 ? 'movie' : 'movies'}
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
              placeholder="Search movies..."
              className="w-full pl-9 pr-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white placeholder-slate-500 focus:outline-none focus:border-emerald-500"
            />
          </div>
          <button
            onClick={startAddPanel}
            className="inline-flex items-center justify-center gap-2 px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-lg shadow-lg shadow-emerald-600/20 transition shrink-0"
          >
            <Plus className="w-4 h-4" />
            <span>Add Movie</span>
          </button>
        </div>
      </div>

      {filteredMovies.length === 0 ? (
        <div className="bg-slate-900/60 border border-dashed border-slate-800 rounded-2xl p-12 text-center">
          <Film className="w-10 h-10 text-slate-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-slate-200">No Movies in Library</h3>
          <p className="text-xs text-slate-400 mt-1 mb-5">
            Look up a movie by title or TMDB/IMDb ID to add it to your library.
          </p>
          <button
            onClick={startAddPanel}
            className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Lookup & Add Movie</span>
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
          {filteredMovies.map((m, index) => {
            const posterUrl = getPosterUrl(m);
            return (
              <div
                key={m.id}
                {...gridFocus.itemProps(index)}
                role="button"
                onClick={() => setActiveMovie(m)}
                className="group bg-slate-900/90 border border-slate-800 hover:border-emerald-500/40 rounded-xl overflow-hidden flex flex-col justify-between transition shadow-lg outline-none cursor-pointer"
              >
                <div className="p-4 flex flex-row gap-3 sm:gap-4">
                  <div className="w-20 h-28 sm:w-24 sm:h-36 rounded-lg bg-gradient-to-br from-emerald-900/60 via-slate-900 to-slate-950 border border-slate-800 shrink-0 overflow-hidden flex items-center justify-center relative">
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

                  <div className="flex-1 min-w-0">
                    <h3 className="text-base font-bold text-white truncate">{m.title}</h3>
                    <div className="mt-1">
                      {m.hasFile ? (
                        <span className="inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                          <CheckCircle2 className="w-3 h-3" /> Downloaded
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30">
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

                    <p className="mt-2 text-xs text-slate-400 leading-relaxed hidden md:block">
                      {truncateText(
                        m.overview || 'Monitored feature film in Replacearr unified database.'
                      )}
                    </p>

                    <div className="mt-3 flex items-center gap-1.5 text-[11px] font-mono text-slate-500 truncate">
                      <Folder className="w-3.5 h-3.5 text-slate-500 shrink-0" />
                      <span className="truncate">{m.path || `${m.rootFolderPath}/${m.title}`}</span>
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
