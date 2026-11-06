import React, { useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { FolderSearch, Upload, RefreshCw, Search, X } from 'lucide-react';
import type {
  Artist,
  Author,
  Book,
  LibraryImportMatch,
  LibraryImportScanItem,
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

function artistToMatch(a: Artist, libraryId?: number): LibraryImportMatch {
  return {
    libraryId,
    foreignId: a.foreignArtistId || undefined,
    title: a.title,
    overview: a.overview,
    disambiguation: a.disambiguation,
    inLibrary: Boolean(libraryId),
  };
}

function bookToMatch(b: Book, libraryId?: number, format?: string): LibraryImportMatch {
  const year = Number.parseInt((b.releaseDate || '').slice(0, 4), 10);
  const authorName = b.overview?.startsWith('by ') ? b.overview.slice(3) : undefined;
  return {
    libraryId,
    foreignId: b.foreignBookId || undefined,
    title: b.title,
    year: Number.isFinite(year) && year > 0 ? year : undefined,
    overview: b.overview,
    authorName,
    format,
    inLibrary: Boolean(libraryId),
  };
}

function guessBookFormat(path: string): 'ebook' | 'audiobook' {
  const ext = path.split('.').pop()?.toLowerCase() || '';
  if (['m4b', 'mp3', 'flac', 'm4a', 'aac'].includes(ext)) return 'audiobook';
  return 'ebook';
}

function matchSourceHint(media: MediaNavType): string {
  switch (media) {
    case 'movies':
      return 'Search TMDB…';
    case 'series':
      return 'Search TVDB…';
    case 'music':
      return 'Search MusicBrainz…';
    case 'books':
      return 'Search OpenLibrary…';
    default:
      return 'Search metadata…';
  }
}

const MatchSearchField: React.FC<{
  query: string;
  match: LibraryImportMatch | null;
  open: boolean;
  loading: boolean;
  results: LibraryImportMatch[];
  placeholder?: string;
  onQueryChange: (value: string) => void;
  onFocus: () => void;
  onClear: () => void;
  onSelect: (match: LibraryImportMatch) => void;
  onClose: () => void;
}> = ({
  query,
  match,
  open,
  loading,
  results,
  placeholder = 'Search metadata…',
  onQueryChange,
  onFocus,
  onClear,
  onSelect,
  onClose,
}) => {
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
                  key={`${hit.tmdbId || hit.tvdbId || hit.foreignId || hit.title}-${hit.year || 0}`}
                  type="button"
                  onMouseDown={(e) => e.preventDefault()}
                  onClick={() => onSelect(hit)}
                  className="w-full text-left px-3 py-2 hover:bg-slate-900 border-b border-slate-800/80 last:border-0"
                >
                  <div className="text-xs text-slate-100 font-medium">
                    {matchLabel(hit)}
                    {hit.disambiguation ? (
                      <span className="ml-1 text-slate-400 font-normal">({hit.disambiguation})</span>
                    ) : null}
                    {hit.inLibrary ? (
                      <span className="ml-2 text-[10px] uppercase tracking-wide text-emerald-400">
                        library
                      </span>
                    ) : null}
                  </div>
                  <div className="text-[11px] text-slate-500 truncate">
                    {hit.tmdbId ? `tmdb:${hit.tmdbId}` : ''}
                    {hit.tvdbId ? `tvdb:${hit.tvdbId}` : ''}
                    {hit.foreignId && !hit.tmdbId && !hit.tvdbId ? hit.foreignId : ''}
                    {hit.authorName ? ` · ${hit.authorName}` : ''}
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
          placeholder={placeholder}
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
          {match.foreignId && !match.tmdbId && !match.tvdbId ? ` · ${match.foreignId}` : ''}
          {match.authorName ? ` · ${match.authorName}` : ''}
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
  authors: _authors,
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
    if (apiMedia === 'music') {
      return profiles.filter((p) => !p.mediaType || p.mediaType === 'music');
    }
    if (apiMedia === 'books') {
      return profiles.filter((p) => !p.mediaType || p.mediaType === 'ebook' || p.mediaType === 'audiobook');
    }
    return profiles;
  }, [profiles, apiMedia]);

  const [folderId, setFolderId] = useState<number>(folders[0]?.id ?? 0);
  const [customPath, setCustomPath] = useState('');
  const [profileId, setProfileId] = useState<number>(mediaProfiles[0]?.id ?? 1);
  const [rows, setRows] = useState<ScanRow[]>([]);
  const [scanId, setScanId] = useState<string | null>(null);
  const [total, setTotal] = useState(0);
  const [offset, setOffset] = useState(0);
  const [pageSize, setPageSize] = useState(100);
  const [scanning, setScanning] = useState(false);
  const [matchingPage, setMatchingPage] = useState(false);
  const [importing, setImporting] = useState(false);
  const [activeSearchIdx, setActiveSearchIdx] = useState<number | null>(null);
  const [searchResults, setSearchResults] = useState<LibraryImportMatch[]>([]);
  const [searchLoading, setSearchLoading] = useState(false);
  const searchTimer = useRef<number | null>(null);

  const pageFrom = total === 0 ? 0 : offset + 1;
  const pageTo = Math.min(offset + rows.length, total);
  const canPrev = offset > 0 && Boolean(scanId);
  const canNext = offset + pageSize < total && Boolean(scanId);

  const mapScanItems = (items: LibraryImportScanItem[] | undefined): ScanRow[] =>
    (items ?? []).map((item) => {
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
    });

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
    if (!q) {
      setSearchResults([]);
      return;
    }
    setSearchLoading(true);
    try {
      if (media === 'movies') {
        const results = await api.lookupMovies(q);
        const byTmdb = new Map(movies.filter((m) => m.tmdbId > 0).map((m) => [m.tmdbId, m.id]));
        setSearchResults(results.map((m) => movieToMatch(m, byTmdb.get(m.tmdbId))));
      } else if (media === 'series') {
        const results = await api.lookupSeries(q);
        const byTvdb = new Map(series.filter((s) => s.tvdbId > 0).map((s) => [s.tvdbId, s.id]));
        setSearchResults(results.map((s) => seriesToMatch(s, byTvdb.get(s.tvdbId))));
      } else if (media === 'music') {
        const results = await api.lookupArtists(q);
        const byForeign = new Map(
          artists.filter((a) => a.foreignArtistId).map((a) => [a.foreignArtistId, a.id])
        );
        setSearchResults(results.map((a) => artistToMatch(a, byForeign.get(a.foreignArtistId))));
      } else {
        const results = await api.lookupBooks(q);
        const byForeign = new Map(
          books.filter((b) => b.foreignBookId).map((b) => [b.foreignBookId, b.id])
        );
        const format = rows[idx] ? guessBookFormat(rows[idx].path) : 'ebook';
        setSearchResults(results.map((b) => bookToMatch(b, byForeign.get(b.foreignBookId), format)));
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

  const applyScanPage = (result: {
    scanId?: string;
    total: number;
    offset: number;
    pageSize: number;
    items: LibraryImportScanItem[];
  }) => {
    if (result.scanId) setScanId(result.scanId);
    setTotal(result.total ?? 0);
    setOffset(result.offset ?? 0);
    if (result.pageSize > 0) setPageSize(result.pageSize);
    setRows(mapScanItems(result.items));
    setActiveSearchIdx(null);
    setSearchResults([]);
  };

  const loadPage = async (nextOffset: number, size = pageSize) => {
    if (!scanId) return;
    setMatchingPage(true);
    setActiveSearchIdx(null);
    setSearchResults([]);
    try {
      const result = await api.getLibraryImportScanPage(scanId, {
        offset: nextOffset,
        limit: size,
      });
      applyScanPage(result);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load page', true);
    } finally {
      setMatchingPage(false);
    }
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
        limit: pageSize,
      });
      applyScanPage(result);
      onNotify(`Found ${result.total ?? 0} unmapped file(s)`);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Scan failed', true);
    } finally {
      setScanning(false);
    }
  };

  const doImport = async () => {
    const selected = rows.filter((r) => {
      if (!r.selected || !r.match) return false;
      return Boolean(
        r.match.tmdbId || r.match.tvdbId || r.match.foreignId || r.match.libraryId
      );
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
        scanId: scanId || undefined,
        items: selected.map((r) => ({
          path: r.path,
          matchId: r.match?.libraryId || undefined,
          tmdbId: r.match?.tmdbId || undefined,
          tvdbId: r.match?.tvdbId || undefined,
          foreignId: r.match?.foreignId || undefined,
          format: r.match?.format || undefined,
          qualityProfileId: profileId,
          seasonNumber: r.seasonNumber,
          episodeNumber: r.episodeNumber,
        })),
      });
      const failed = (result.results ?? []).filter((r) => !r.ok);
      if (failed.length) {
        onNotify(
          `Imported ${result.imported ?? 0}; ${failed.length} failed` +
            (failed[0]?.error ? ` (${failed[0].error})` : ''),
          true
        );
      } else {
        onNotify(`Imported ${result.imported ?? 0} file(s)`);
      }
      await onRefresh();
      if (scanId) {
        setMatchingPage(true);
        try {
          let page = await api.getLibraryImportScanPage(scanId, {
            offset,
            limit: pageSize,
          });
          if ((page.items?.length ?? 0) === 0 && (page.total ?? 0) > 0 && offset > 0) {
            const last = Math.max(0, Math.floor((page.total - 1) / pageSize) * pageSize);
            page = await api.getLibraryImportScanPage(scanId, {
              offset: last,
              limit: pageSize,
            });
          }
          applyScanPage(page);
        } catch (err) {
          onNotify(err instanceof Error ? err.message : 'Failed to refresh page', true);
        } finally {
          setMatchingPage(false);
        }
      } else {
        const okPaths = new Set(
          (result.results ?? []).filter((r) => r.ok).map((r) => r.path)
        );
        setRows((prev) => prev.filter((r) => !okPaths.has(r.path)));
        setTotal((t) => Math.max(0, t - (result.imported ?? 0)));
      }
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
              matches: [
                match,
                ...r.matches.filter((m) => {
                  if (match.tmdbId) return m.tmdbId !== match.tmdbId;
                  if (match.tvdbId) return m.tvdbId !== match.tvdbId;
                  if (match.foreignId) return m.foreignId !== match.foreignId;
                  return m.title !== match.title;
                }),
              ],
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
            Scan folders and match files by name against TMDB, TVDB, MusicBrainz, or OpenLibrary, then
            import into your library.
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
        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            disabled={scanning}
            onClick={() => void scan()}
            className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-60"
          >
            <FolderSearch className={`w-3.5 h-3.5 ${scanning ? 'animate-pulse' : ''}`} />
            {scanning ? 'Scanning…' : 'Scan for unmapped files'}
          </button>
          <label className="inline-flex items-center gap-2 text-xs text-slate-400">
            Page size
            <select
              value={pageSize}
              disabled={scanning || matchingPage}
              onChange={(e) => {
                const next = Number(e.target.value) || 100;
                setPageSize(next);
                if (scanId) void loadPage(0, next);
              }}
              className="bg-slate-950 border border-slate-800 rounded-lg px-2 py-1.5 text-xs text-white"
            >
              <option value={50}>50</option>
              <option value={100}>100</option>
              <option value={200}>200</option>
            </select>
          </label>
        </div>
      </div>

      {total > 0 || rows.length > 0 ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl">
          <div className="px-3 py-2 border-b border-slate-800 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-400">
            <span>
              {matchingPage
                ? 'Matching page…'
                : total > 0
                  ? `Showing ${pageFrom}–${pageTo} of ${total}`
                  : 'No remaining files'}
            </span>
            <div className="flex items-center gap-2">
              <button
                type="button"
                disabled={!canPrev || matchingPage || importing}
                onClick={() => void loadPage(Math.max(0, offset - pageSize))}
                className="px-2.5 py-1 rounded-md border border-slate-700 text-slate-300 hover:bg-slate-800 disabled:opacity-40"
              >
                Prev
              </button>
              <button
                type="button"
                disabled={!canNext || matchingPage || importing}
                onClick={() => void loadPage(offset + pageSize)}
                className="px-2.5 py-1 rounded-md border border-slate-700 text-slate-300 hover:bg-slate-800 disabled:opacity-40"
              >
                Next
              </button>
            </div>
          </div>
          {rows.length > 0 ? (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead className="bg-slate-950/80 text-slate-400 text-left">
                  <tr>
                    <th className="px-3 py-2 w-8">
                      <input
                        type="checkbox"
                        title="Select all on this page"
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
                        <MatchSearchField
                          query={row.query}
                          match={row.match}
                          open={activeSearchIdx === idx}
                          loading={searchLoading && activeSearchIdx === idx}
                          placeholder={matchSourceHint(media)}
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
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <div className="px-3 py-6 text-xs text-slate-500 text-center">
              {matchingPage ? 'Matching…' : 'No files on this page.'}
            </div>
          )}
          <div className="p-3 border-t border-slate-800 flex justify-end">
            <button
              type="button"
              disabled={importing || matchingPage || rows.length === 0}
              onClick={() => void doImport()}
              className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium bg-emerald-600 hover:bg-emerald-500 text-white disabled:opacity-60"
            >
              <Upload className="w-3.5 h-3.5" />
              {importing ? 'Importing…' : 'Import selected (this page)'}
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
};
