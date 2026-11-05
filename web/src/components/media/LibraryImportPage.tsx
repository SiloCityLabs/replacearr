import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { FolderSearch, Upload, RefreshCw, Search, X } from 'lucide-react';
import type {
  Artist,
  Author,
  Book,
  LibraryImportMatch,
  Movie,
  QualityProfile,
  RootFolder,
  Series,
} from '../../types';
import { api, formatBytes } from '../../api';
import type { MediaNavType } from '../../nav';
import { mediaTypeApi } from '../../nav';

export interface LibraryImportPageProps {
  media: MediaNavType;
  rootFolders: RootFolder[];
  profiles: QualityProfile[];
  series: Series[];
  movies: Movie[];
  artists: Artist[];
  authors: Author[];
  books: Book[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

interface ScanRow {
  path: string;
  relativePath: string;
  size: number;
  guessedTitle: string;
  guessedYear?: number;
  seasonNumber?: number;
  episodeNumber?: number;
  selected: boolean;
  match: LibraryImportMatch | null;
  matches: LibraryImportMatch[];
  query: string;
}

function matchLabel(m: LibraryImportMatch | null | undefined): string {
  if (!m?.title) return '';
  return m.year ? `${m.title} (${m.year})` : m.title;
}

function movieToMatch(m: Movie, libraryId?: number): LibraryImportMatch {
  return {
    libraryId,
    tmdbId: m.tmdbId || undefined,
    imdbId: m.imdbId || undefined,
    title: m.title,
    year: m.year || undefined,
    overview: m.overview,
    inLibrary: Boolean(libraryId),
  };
}

function seriesToMatch(s: Series, libraryId?: number): LibraryImportMatch {
  return {
    libraryId,
    tvdbId: s.tvdbId || undefined,
    imdbId: s.imdbId || undefined,
    title: s.title,
    year: s.year || undefined,
    overview: s.overview,
    inLibrary: Boolean(libraryId),
  };
}

const MatchSearchField: React.FC<{
  query: string;
  match: LibraryImportMatch | null;
  open: boolean;
  loading: boolean;
  results: LibraryImportMatch[];
  onQueryChange: (value: string) => void;
  onFocus: () => void;
  onClear: () => void;
  onSelect: (match: LibraryImportMatch) => void;
  onClose: () => void;
}> = ({ query, match, open, loading, results, onQueryChange, onFocus, onClear, onSelect, onClose }) => {
  const inputRef = useRef<HTMLInputElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const [menuBox, setMenuBox] = useState<{ top: number; left: number; width: number } | null>(null);

  const updateMenuBox = () => {
    const el = inputRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    setMenuBox({
      top: rect.bottom + 4,
      left: rect.left,
      width: Math.max(rect.width, 280),
    });
  };

  useLayoutEffect(() => {
    if (!open) {
      setMenuBox(null);
      return;
    }
    updateMenuBox();
    const onReposition = () => updateMenuBox();
    window.addEventListener('resize', onReposition);
    window.addEventListener('scroll', onReposition, true);
    return () => {
      window.removeEventListener('resize', onReposition);
      window.removeEventListener('scroll', onReposition, true);
    };
  }, [open, results.length, loading, query]);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node;
      if (inputRef.current?.contains(target) || menuRef.current?.contains(target)) return;
      onClose();
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open, onClose]);

  const menu =
    open && menuBox
      ? createPortal(
          <div
            ref={menuRef}
            style={{ top: menuBox.top, left: menuBox.left, width: menuBox.width }}
            className="fixed z-[80] max-h-64 overflow-y-auto rounded-md border border-slate-700 bg-slate-950 shadow-2xl"
          >
            {loading ? (
              <div className="px-3 py-2 text-xs text-slate-400">Searching…</div>
            ) : results.length === 0 ? (
              <div className="px-3 py-2 text-xs text-slate-400">No results</div>
            ) : (
              results.map((hit) => (
                <button
                  key={`${hit.tmdbId || hit.tvdbId || hit.title}-${hit.year || 0}`}
                  type="button"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => onSelect(hit)}
                  className="w-full text-left px-3 py-2 hover:bg-slate-900 border-b border-slate-800/80 last:border-0"
                >
                  <div className="text-xs text-slate-100 font-medium">
                    {matchLabel(hit)}
                    {hit.inLibrary ? (
                      <span className="ml-2 text-[10px] uppercase tracking-wide text-emerald-400">
                        library
                      </span>
                    ) : null}
                  </div>
                  <div className="text-[11px] text-slate-500 truncate">
                    {hit.tmdbId ? `tmdb:${hit.tmdbId}` : ''}
                    {hit.tvdbId ? `tvdb:${hit.tvdbId}` : ''}
                    {hit.overview ? ` — ${hit.overview}` : ''}
                  </div>
                </button>
              ))
            )}
          </div>,
          document.body
        )
      : null;

  return (
    <div className="min-w-[16rem]">
      <div className="relative">
        <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2 top-1/2 -translate-y-1/2 pointer-events-none" />
        <input
          ref={inputRef}
          value={query}
          onChange={(e) => onQueryChange(e.target.value)}
          onFocus={onFocus}
          placeholder="Search TMDB / TVDB…"
          className="w-full pl-7 pr-7 py-1.5 bg-slate-950 border border-slate-800 rounded-md text-xs text-white"
        />
        {query ? (
          <button
            type="button"
            className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-slate-500 hover:text-white"
            onClick={onClear}
          >
            <X className="w-3 h-3" />
          </button>
        ) : null}
      </div>
      {match ? (
        <div className="mt-1 text-[11px] text-emerald-300/90 truncate" title={match.overview}>
          {matchLabel(match)}
          {match.tmdbId ? ` · tmdb:${match.tmdbId}` : ''}
          {match.tvdbId ? ` · tvdb:${match.tvdbId}` : ''}
          {match.inLibrary ? ' · in library' : ''}
        </div>
      ) : (
        <div className="mt-1 text-[11px] text-amber-300/80">No match — type to search</div>
      )}
      {menu}
    </div>
  );
};

export const LibraryImportPage: React.FC<LibraryImportPageProps> = ({
  media,
  rootFolders,
  profiles,
  series,
  movies,
  artists,
  authors,
  books,
  onRefresh,
  onNotify,
}) => {
  const apiMedia = mediaTypeApi(media);
  const folders = useMemo(
    () =>
      rootFolders.filter((f) => {
        if (f.mediaType === 'all' || !f.mediaType) return true;
        if (apiMedia === 'tv') return f.mediaType === 'tv';
        if (apiMedia === 'movies') return f.mediaType === 'movie' || f.mediaType === 'movies';
        if (apiMedia === 'music') return f.mediaType === 'music';
        if (apiMedia === 'books') return f.mediaType === 'ebook' || f.mediaType === 'audiobook' || f.mediaType === 'books';
        return true;
      }),
    [rootFolders, apiMedia]
  );

  const mediaProfiles = useMemo(() => {
    if (apiMedia === 'movies' || apiMedia === 'tv') {
      return profiles.filter((p) => !p.mediaType || p.mediaType === 'video' || p.mediaType === apiMedia);
    }
    return profiles;
  }, [profiles, apiMedia]);

  const [folderId, setFolderId] = useState<number>(folders[0]?.id ?? 0);
  const [customPath, setCustomPath] = useState('');
  const [profileId, setProfileId] = useState<number>(mediaProfiles[0]?.id ?? 1);
  const [rows, setRows] = useState<ScanRow[]>([]);
  const [scanning, setScanning] = useState(false);
  const [importing, setImporting] = useState(false);
  const [activeSearchIdx, setActiveSearchIdx] = useState<number | null>(null);
  const [searchResults, setSearchResults] = useState<LibraryImportMatch[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const searchTimer = useRef<number | null>(null);

  const usesMetadataMatch = media === 'movies' || media === 'series';

  const libraryMatchOptions = useMemo(() => {
    switch (media) {
      case 'music':
        return artists.map((a) => ({
          libraryId: a.id,
          title: a.title || String(a.id),
          inLibrary: true,
        }));
      case 'books':
        return books.map((b) => {
          const author = authors.find((a) => a.id === b.authorId);
          return {
            libraryId: b.id,
            title: author ? `${b.title} — ${author.title}` : b.title,
            inLibrary: true,
          };
        });
      default:
        return [];
    }
  }, [media, artists, authors, books]);

  useEffect(() => {
    if (mediaProfiles[0]?.id && !mediaProfiles.some((p) => p.id === profileId)) {
      setProfileId(mediaProfiles[0].id);
    }
  }, [mediaProfiles, profileId]);

  useEffect(() => {
    return () => {
      if (searchTimer.current) window.clearTimeout(searchTimer.current);
    };
  }, []);

  const runMetadataSearch = async (idx: number, term: string) => {
    const q = term.trim();
    if (!q || !usesMetadataMatch) {
      setSearchResults([]);
      return;
    }
    setSearchLoading(true);
    try {
      if (media === 'movies') {
        const results = await api.lookupMovies(q);
        const byTmdb = new Map(movies.filter((m) => m.tmdbId > 0).map((m) => [m.tmdbId, m.id]));
        setSearchResults(results.map((m) => movieToMatch(m, byTmdb.get(m.tmdbId))));
      } else {
        const results = await api.lookupSeries(q);
        const byTvdb = new Map(series.filter((s) => s.tvdbId > 0).map((s) => [s.tvdbId, s.id]));
        setSearchResults(results.map((s) => seriesToMatch(s, byTvdb.get(s.tvdbId))));
      }
      setActiveSearchIdx(idx);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Lookup failed', true);
      setSearchResults([]);
    } finally {
      setSearchLoading(false);
    }
  };

  const scheduleSearch = (idx: number, term: string) => {
    if (searchTimer.current) window.clearTimeout(searchTimer.current);
    searchTimer.current = window.setTimeout(() => {
      void runMetadataSearch(idx, term);
    }, 280);
  };

  const scan = async () => {
    setScanning(true);
    setActiveSearchIdx(null);
    setSearchResults([]);
    try {
      const folder = folders.find((f) => f.id === folderId);
      const path = customPath.trim() || folder?.path || '';
      if (!path) {
        onNotify('Select a root folder or enter a path', true);
        return;
      }
      const result = await api.scanLibraryImport({
        mediaType: apiMedia,
        path,
        rootFolderId: folderId || undefined,
      });
      setRows(
        (result.items ?? []).map((item) => {
          const match = item.match ?? item.matches?.[0] ?? null;
          return {
            path: item.path,
            relativePath: item.relativePath,
            size: item.size,
            guessedTitle: item.guessedTitle,
            guessedYear: item.guessedYear,
            seasonNumber: item.seasonNumber,
            episodeNumber: item.episodeNumber,
            selected: Boolean(match),
            match,
            matches: item.matches ?? [],
            query: matchLabel(match) || item.guessedTitle || '',
          };
        })
      );
      onNotify(`Found ${result.items?.length ?? 0} unmapped file(s)`);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Scan failed', true);
    } finally {
      setScanning(false);
    }
  };

  const doImport = async () => {
    const selected = rows.filter((r) => {
      if (!r.selected || !r.match) return false;
      if (usesMetadataMatch) return Boolean(r.match.tmdbId || r.match.tvdbId || r.match.libraryId);
      return Boolean(r.match.libraryId);
    });
    if (!selected.length) {
      onNotify('Select at least one matched file', true);
      return;
    }
    setImporting(true);
    try {
      const folder = folders.find((f) => f.id === folderId);
      const result = await api.runLibraryImport({
        mediaType: apiMedia,
        rootFolderPath: customPath.trim() || folder?.path || undefined,
        qualityProfileId: profileId,
        items: selected.map((r) => ({
          path: r.path,
          matchId: r.match?.libraryId || undefined,
          tmdbId: r.match?.tmdbId || undefined,
          tvdbId: r.match?.tvdbId || undefined,
          qualityProfileId: profileId,
          seasonNumber: r.seasonNumber,
          episodeNumber: r.episodeNumber,
        })),
      });
      onNotify(`Imported ${result.imported ?? 0} file(s)`);
      setRows((prev) => prev.filter((r) => !r.selected));
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Import failed', true);
    } finally {
      setImporting(false);
    }
  };

  const rescan = async () => {
    try {
      await api.rescanLibrary({ mediaType: apiMedia });
      onNotify('Library rescan queued');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Rescan failed', true);
    }
  };

  const applyMatch = (idx: number, match: LibraryImportMatch) => {
    setRows((prev) =>
      prev.map((r, i) =>
        i === idx
          ? {
              ...r,
              match,
              selected: true,
              query: matchLabel(match),
              matches: [match, ...r.matches.filter((m) => m.tmdbId !== match.tmdbId || m.tvdbId !== match.tvdbId)],
            }
          : r
      )
    );
    setActiveSearchIdx(null);
    setSearchResults([]);
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white">Library Import</h2>
          <p className="text-xs text-slate-400 mt-1">
            {usesMetadataMatch
              ? 'Scan folders and match files to TMDB / TVDB by folder name, then import into your library.'
              : 'Scan a folder for unmapped media files and import them into your library.'}
          </p>
        </div>
        <button
          type="button"
          onClick={() => void rescan()}
          className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs bg-slate-900 border border-slate-800 text-slate-300 hover:bg-slate-800"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          Rescan library
        </button>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <label className="block text-xs space-y-1">
            <span className="text-slate-400">Root folder</span>
            <select
              value={folderId}
              onChange={(e) => setFolderId(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
            >
              <option value={0}>—</option>
              {folders.map((f) => (
                <option key={f.id} value={f.id}>
                  {f.path} ({f.mediaType || 'all'})
                </option>
              ))}
            </select>
          </label>
          <label className="block text-xs space-y-1">
            <span className="text-slate-400">Or custom path</span>
            <input
              value={customPath}
              onChange={(e) => setCustomPath(e.target.value)}
              placeholder="/data/..."
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
            />
          </label>
          <label className="block text-xs space-y-1">
            <span className="text-slate-400">Quality profile</span>
            <select
              value={profileId}
              onChange={(e) => setProfileId(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
            >
              {mediaProfiles.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name}
                </option>
              ))}
            </select>
          </label>
        </div>
        <button
          type="button"
          disabled={scanning}
          onClick={() => void scan()}
          className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-60"
        >
          <FolderSearch className={`w-3.5 h-3.5 ${scanning ? 'animate-pulse' : ''}`} />
          {scanning ? 'Scanning & matching…' : 'Scan for unmapped files'}
        </button>
      </div>

      {rows.length > 0 ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl">
          <div className="overflow-x-auto rounded-xl">
            <table className="w-full text-xs">
              <thead className="bg-slate-950/80 text-slate-400 text-left">
                <tr>
                  <th className="px-3 py-2 w-8">
                    <input
                      type="checkbox"
                      checked={rows.length > 0 && rows.every((r) => r.selected)}
                      onChange={(e) =>
                        setRows((prev) => prev.map((r) => ({ ...r, selected: e.target.checked })))
                      }
                    />
                  </th>
                  <th className="px-3 py-2">File</th>
                  <th className="px-3 py-2">Size</th>
                  <th className="px-3 py-2">Guess</th>
                  <th className="px-3 py-2 min-w-[16rem]">Match</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row, idx) => (
                  <tr key={row.path} className="border-t border-slate-800/80 align-top">
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        checked={row.selected}
                        onChange={(e) =>
                          setRows((prev) =>
                            prev.map((r, i) => (i === idx ? { ...r, selected: e.target.checked } : r))
                          )
                        }
                      />
                    </td>
                    <td className="px-3 py-2 text-slate-200 font-mono max-w-xs truncate" title={row.path}>
                      {row.relativePath || row.path}
                    </td>
                    <td className="px-3 py-2 text-slate-400">{formatBytes(row.size)}</td>
                    <td className="px-3 py-2 text-slate-300">
                      {row.guessedTitle || '—'}
                      {row.guessedYear ? ` (${row.guessedYear})` : ''}
                      {row.seasonNumber != null && row.episodeNumber != null
                        ? ` S${row.seasonNumber}E${row.episodeNumber}`
                        : ''}
                    </td>
                    <td className="px-3 py-2">
                      {usesMetadataMatch ? (
                        <MatchSearchField
                          query={row.query}
                          match={row.match}
                          open={activeSearchIdx === idx}
                          loading={searchLoading && activeSearchIdx === idx}
                          results={
                            activeSearchIdx === idx
                              ? searchResults.length > 0
                                ? searchResults
                                : row.matches
                              : []
                          }
                          onQueryChange={(value) => {
                            setRows((prev) =>
                              prev.map((r, i) => (i === idx ? { ...r, query: value, match: null } : r))
                            );
                            setActiveSearchIdx(idx);
                            scheduleSearch(idx, value);
                          }}
                          onFocus={() => {
                            setActiveSearchIdx(idx);
                            if (row.matches.length) {
                              setSearchResults(row.matches);
                            } else if (row.query.trim()) {
                              scheduleSearch(idx, row.query);
                            }
                          }}
                          onClear={() => {
                            setRows((prev) =>
                              prev.map((r, i) => (i === idx ? { ...r, query: '', match: null } : r))
                            );
                            setSearchResults([]);
                            setActiveSearchIdx(idx);
                          }}
                          onSelect={(hit) => applyMatch(idx, hit)}
                          onClose={() => {
                            if (activeSearchIdx === idx) setActiveSearchIdx(null);
                          }}
                        />
                      ) : (
                        <select
                          value={row.match?.libraryId ?? 0}
                          onChange={(e) => {
                            const id = Number(e.target.value);
                            const opt = libraryMatchOptions.find((o) => o.libraryId === id) || null;
                            setRows((prev) =>
                              prev.map((r, i) =>
                                i === idx
                                  ? {
                                      ...r,
                                      match: opt,
                                      selected: Boolean(opt),
                                      query: opt?.title || '',
                                    }
                                  : r
                              )
                            );
                          }}
                          className="bg-slate-950 border border-slate-800 rounded-md px-2 py-1 text-xs text-white max-w-[14rem]"
                        >
                          <option value={0}>— select —</option>
                          {libraryMatchOptions.map((o) => (
                            <option key={o.libraryId} value={o.libraryId}>
                              {o.title}
                            </option>
                          ))}
                        </select>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="p-3 border-t border-slate-800 flex justify-end">
            <button
              type="button"
              disabled={importing}
              onClick={() => void doImport()}
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium bg-emerald-600 hover:bg-emerald-500 text-white disabled:opacity-60"
            >
              <Upload className="w-3.5 h-3.5" />
              {importing ? 'Importing…' : 'Import selected'}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
};
