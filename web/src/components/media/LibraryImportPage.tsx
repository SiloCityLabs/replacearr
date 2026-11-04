import React, { useMemo, useState } from 'react';
import { FolderSearch, Upload, RefreshCw } from 'lucide-react';
import type { Artist, Author, Book, Movie, RootFolder, Series } from '../../types';
import { api, formatBytes } from '../../api';
import type { MediaNavType } from '../../nav';
import { mediaTypeApi } from '../../nav';

export interface LibraryImportPageProps {
  media: MediaNavType;
  rootFolders: RootFolder[];
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
  seasonNumber?: number;
  episodeNumber?: number;
  selected: boolean;
  matchId: number;
}

export const LibraryImportPage: React.FC<LibraryImportPageProps> = ({
  media,
  rootFolders,
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

  const [folderId, setFolderId] = useState<number>(folders[0]?.id ?? 0);
  const [customPath, setCustomPath] = useState('');
  const [rows, setRows] = useState<ScanRow[]>([]);
  const [scanning, setScanning] = useState(false);
  const [importing, setImporting] = useState(false);

  const matchOptions = useMemo(() => {
    switch (media) {
      case 'series':
        return series.map((s) => ({ id: s.id, label: s.title }));
      case 'movies':
        return movies.map((m) => ({ id: m.id, label: m.title }));
      case 'music':
        return artists.map((a) => ({ id: a.id, label: a.title || String(a.id) }));
      case 'books':
        return books.map((b) => {
          const author = authors.find((a) => a.id === b.authorId);
          return { id: b.id, label: author ? `${b.title} — ${author.title}` : b.title };
        });
      default:
        return [];
    }
  }, [media, series, movies, artists, authors, books]);

  const scan = async () => {
    setScanning(true);
    try {
      const folder = folders.find((f) => f.id === folderId);
      const path = customPath.trim() || folder?.path || '';
      if (!path) {
        onNotify('Select a root folder or enter a path', true);
        return;
      }
      const result = await api.scanLibraryImport({ mediaType: apiMedia, path, rootFolderId: folderId || undefined });
      setRows(
        (result.items ?? []).map((item) => ({
          ...item,
          selected: true,
          matchId: matchOptions[0]?.id ?? 0,
        }))
      );
      onNotify(`Found ${result.items?.length ?? 0} unmapped file(s)`);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Scan failed', true);
    } finally {
      setScanning(false);
    }
  };

  const doImport = async () => {
    const selected = rows.filter((r) => r.selected && r.matchId > 0);
    if (!selected.length) {
      onNotify('Select at least one matched file', true);
      return;
    }
    setImporting(true);
    try {
      const result = await api.runLibraryImport({
        mediaType: apiMedia,
        items: selected.map((r) => ({
          path: r.path,
          matchId: r.matchId,
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

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white">Library Import</h2>
          <p className="text-xs text-slate-400 mt-1">
            Scan a folder for unmapped media files and import them into your library.
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
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
        </div>
        <button
          type="button"
          disabled={scanning}
          onClick={() => void scan()}
          className="inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs font-medium bg-indigo-600 hover:bg-indigo-500 text-white disabled:opacity-60"
        >
          <FolderSearch className={`w-3.5 h-3.5 ${scanning ? 'animate-pulse' : ''}`} />
          {scanning ? 'Scanning…' : 'Scan for unmapped files'}
        </button>
      </div>

      {rows.length > 0 ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="bg-slate-950/80 text-slate-400 text-left">
                <tr>
                  <th className="px-3 py-2 w-8">
                    <input
                      type="checkbox"
                      checked={rows.every((r) => r.selected)}
                      onChange={(e) =>
                        setRows((prev) => prev.map((r) => ({ ...r, selected: e.target.checked })))
                      }
                    />
                  </th>
                  <th className="px-3 py-2">File</th>
                  <th className="px-3 py-2">Size</th>
                  <th className="px-3 py-2">Guess</th>
                  <th className="px-3 py-2">Match</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row, idx) => (
                  <tr key={row.path} className="border-t border-slate-800/80">
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
                      {row.guessedTitle}
                      {row.seasonNumber != null && row.episodeNumber != null
                        ? ` S${row.seasonNumber}E${row.episodeNumber}`
                        : ''}
                    </td>
                    <td className="px-3 py-2">
                      <select
                        value={row.matchId}
                        onChange={(e) =>
                          setRows((prev) =>
                            prev.map((r, i) => (i === idx ? { ...r, matchId: Number(e.target.value) } : r))
                          )
                        }
                        className="bg-slate-950 border border-slate-800 rounded-md px-2 py-1 text-xs text-white max-w-[14rem]"
                      >
                        <option value={0}>— select —</option>
                        {matchOptions.map((o) => (
                          <option key={o.id} value={o.id}>
                            {o.label}
                          </option>
                        ))}
                      </select>
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
