import React, { useCallback, useEffect, useState } from 'react';
import { History, Ban } from 'lucide-react';
import { api } from '../../api';
import type { AppConfig, HistoryRecord } from '../../types';
import { formatAppDate } from '../../uiPrefs';

export interface SystemEventsPageProps {
  appUi?: AppConfig['ui'] | null;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SystemEventsPage: React.FC<SystemEventsPageProps> = ({ appUi, onNotify }) => {
  const [items, setItems] = useState<HistoryRecord[]>([]);
  const [page, setPage] = useState(1);
  const [total, setTotal] = useState(0);
  const [mediaType, setMediaType] = useState('');
  const pageSize = 50;

  const load = useCallback(async () => {
    try {
      const res = await api.getHistory({ page, pageSize, mediaType: mediaType || undefined });
      setItems(res.records);
      setTotal(res.totalRecords);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load events', true);
    }
  }, [page, mediaType, onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  const block = async (item: HistoryRecord) => {
    try {
      await api.addBlocklist({
        mediaType: item.mediaType === 'movie' ? 'movies' : item.mediaType === 'tv' ? 'tv' : item.mediaType,
        sourceTitle: item.sourceTitle,
        seriesId: item.seriesId,
        movieId: item.movieId,
        artistId: item.artistId,
        authorId: item.authorId,
        message: 'Blocked from history',
      });
      onNotify('Added to blocklist');
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Block failed', true);
    }
  };

  const pages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
            <History className="w-5 h-5 text-indigo-300" />
            Events
          </h2>
          <p className="text-xs text-slate-400 mt-1">Download and import history.</p>
        </div>
        <select
          value={mediaType}
          onChange={(e) => {
            setPage(1);
            setMediaType(e.target.value);
          }}
          className="bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
        >
          <option value="">All media</option>
          <option value="tv">TV</option>
          <option value="movie">Movies</option>
          <option value="music">Music</option>
          <option value="ebook">Books</option>
          <option value="audiobook">Audiobooks</option>
        </select>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <table className="w-full text-xs">
          <thead className="bg-slate-950/80 text-slate-400 text-left">
            <tr>
              <th className="px-3 py-2">When</th>
              <th className="px-3 py-2">Event</th>
              <th className="px-3 py-2">Media</th>
              <th className="px-3 py-2">Source</th>
              <th className="px-3 py-2 w-10" />
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.id} className="border-t border-slate-800/80">
                <td className="px-3 py-2 text-slate-400 whitespace-nowrap">
                  {formatAppDate(item.date, appUi, 'short')}
                </td>
                <td className="px-3 py-2 text-slate-200">{item.eventType}</td>
                <td className="px-3 py-2 text-slate-400">{item.mediaType}</td>
                <td className="px-3 py-2 text-slate-300 max-w-sm truncate" title={item.sourceTitle}>
                  {item.sourceTitle}
                </td>
                <td className="px-3 py-2">
                  <button
                    type="button"
                    title="Add to blocklist"
                    onClick={() => void block(item)}
                    className="p-1.5 rounded-md text-slate-400 hover:text-rose-300"
                  >
                    <Ban className="w-3.5 h-3.5" />
                  </button>
                </td>
              </tr>
            ))}
            {items.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-3 py-6 text-center text-slate-500">
                  No events.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between text-xs text-slate-400">
        <span>
          Page {page} / {pages} · {total} total
        </span>
        <div className="flex gap-2">
          <button
            type="button"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
            className="px-2 py-1 rounded border border-slate-800 disabled:opacity-40"
          >
            Prev
          </button>
          <button
            type="button"
            disabled={page >= pages}
            onClick={() => setPage((p) => p + 1)}
            className="px-2 py-1 rounded border border-slate-800 disabled:opacity-40"
          >
            Next
          </button>
        </div>
      </div>
    </div>
  );
};
