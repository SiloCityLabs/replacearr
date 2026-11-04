import React, { useCallback, useEffect, useState } from 'react';
import { Ban, Trash2 } from 'lucide-react';
import { api, formatBytes } from '../../api';
import type { MediaNavType } from '../../nav';
import { mediaTypeApi } from '../../nav';
import type { BlocklistItem } from '../../types';
import { formatAppDate } from '../../uiPrefs';
import type { AppConfig } from '../../types';

export interface BlocklistPageProps {
  media: MediaNavType;
  appUi?: AppConfig['ui'] | null;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const BlocklistPage: React.FC<BlocklistPageProps> = ({ media, appUi, onNotify }) => {
  const [items, setItems] = useState<BlocklistItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [selected, setSelected] = useState<Set<number>>(new Set());

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const list = await api.getBlocklist(mediaTypeApi(media));
      setItems(list);
      setSelected(new Set());
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load blocklist', true);
    } finally {
      setLoading(false);
    }
  }, [media, onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  const removeOne = async (id: number) => {
    try {
      await api.deleteBlocklist(id);
      onNotify('Removed from blocklist');
      await load();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Delete failed', true);
    }
  };

  const removeSelected = async () => {
    if (!selected.size) return;
    try {
      await api.deleteBlocklistBulk([...selected]);
      onNotify(`Removed ${selected.size} item(s)`);
      await load();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Bulk delete failed', true);
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
            <Ban className="w-5 h-5 text-rose-300" />
            Blocklist
          </h2>
          <p className="text-xs text-slate-400 mt-1">
            Releases on this list are rejected when grabbing downloads.
          </p>
        </div>
        {selected.size > 0 ? (
          <button
            type="button"
            onClick={() => void removeSelected()}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs bg-rose-600/20 border border-rose-500/40 text-rose-200 hover:bg-rose-600/30"
          >
            <Trash2 className="w-3.5 h-3.5" />
            Remove selected ({selected.size})
          </button>
        ) : null}
      </div>

      {loading ? (
        <p className="text-sm text-slate-400">Loading…</p>
      ) : items.length === 0 ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-8 text-center text-sm text-slate-400">
          Blocklist is empty.
        </div>
      ) : (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead className="bg-slate-950/80 text-slate-400 text-left">
                <tr>
                  <th className="px-3 py-2 w-8">
                    <input
                      type="checkbox"
                      checked={selected.size === items.length}
                      onChange={(e) =>
                        setSelected(e.target.checked ? new Set(items.map((i) => i.id)) : new Set())
                      }
                    />
                  </th>
                  <th className="px-3 py-2">Source</th>
                  <th className="px-3 py-2">Indexer</th>
                  <th className="px-3 py-2">Size</th>
                  <th className="px-3 py-2">Message</th>
                  <th className="px-3 py-2">Date</th>
                  <th className="px-3 py-2 w-10" />
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id} className="border-t border-slate-800/80">
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        checked={selected.has(item.id)}
                        onChange={(e) => {
                          setSelected((prev) => {
                            const next = new Set(prev);
                            if (e.target.checked) next.add(item.id);
                            else next.delete(item.id);
                            return next;
                          });
                        }}
                      />
                    </td>
                    <td className="px-3 py-2 text-slate-200 max-w-xs truncate" title={item.sourceTitle}>
                      {item.sourceTitle}
                    </td>
                    <td className="px-3 py-2 text-slate-400">{item.indexer || '—'}</td>
                    <td className="px-3 py-2 text-slate-400">{item.size ? formatBytes(item.size) : '—'}</td>
                    <td className="px-3 py-2 text-slate-400 max-w-xs truncate">{item.message || '—'}</td>
                    <td className="px-3 py-2 text-slate-400 whitespace-nowrap">
                      {formatAppDate(item.createdAt, appUi, 'short')}
                    </td>
                    <td className="px-3 py-2">
                      <button
                        type="button"
                        title="Remove"
                        onClick={() => void removeOne(item.id)}
                        className="p-1.5 rounded-md text-slate-400 hover:text-rose-300 hover:bg-rose-500/10"
                      >
                        <Trash2 className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};
