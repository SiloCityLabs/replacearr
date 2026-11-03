import React, { useState } from 'react';
import { X, Magnet } from 'lucide-react';
import { api } from '../api';

export interface ManualTorrentTarget {
  title?: string;
  mediaType: 'tv' | 'movie';
  seriesId?: number;
  episodeId?: number;
  movieId?: number;
  seasonNumber?: number;
}

interface ManualTorrentFormProps {
  target: ManualTorrentTarget;
  onClose: () => void;
  onNotify: (msg: string, isError?: boolean) => void;
  onSuccess?: () => Promise<void> | void;
}

function readFileAsBase64(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = String(reader.result || '');
      const base64 = result.includes(',') ? result.split(',')[1] : result;
      resolve(base64 || '');
    };
    reader.onerror = () => reject(reader.error || new Error('Failed to read torrent file'));
    reader.readAsDataURL(file);
  });
}

export const ManualTorrentForm: React.FC<ManualTorrentFormProps> = ({
  target,
  onClose,
  onNotify,
  onSuccess,
}) => {
  const [magnetUrl, setMagnetUrl] = useState('');
  const [torrentBase64, setTorrentBase64] = useState('');
  const [fileName, setFileName] = useState('');
  const [busy, setBusy] = useState(false);

  const handleFile = async (file: File | null) => {
    if (!file) {
      setTorrentBase64('');
      setFileName('');
      return;
    }
    try {
      const b64 = await readFileAsBase64(file);
      setTorrentBase64(b64);
      setFileName(file.name);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to read .torrent file', true);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!magnetUrl.trim() && !torrentBase64) {
      onNotify('Provide a magnet URL or a .torrent file', true);
      return;
    }
    setBusy(true);
    try {
      await api.manualRelease({
        title: target.title || fileName || 'Manual torrent',
        magnetUrl: magnetUrl.trim() || undefined,
        torrentBase64: torrentBase64 || undefined,
        mediaType: target.mediaType,
        seriesId: target.seriesId,
        episodeId: target.episodeId,
        movieId: target.movieId,
        seasonNumber: target.seasonNumber,
      });
      onNotify(`Queued manual torrent${target.title ? ` for "${target.title}"` : ''}`);
      await onSuccess?.();
      onClose();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to add manual torrent', true);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
      <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl my-auto">
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Magnet className="w-4 h-4 text-amber-400" />
            <h3 className="text-sm font-bold text-white">Add torrent</h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <form onSubmit={(e) => void handleSubmit(e)} className="p-4 space-y-4">
          {target.title ? (
            <p className="text-xs text-slate-400">
              Target: <span className="text-slate-200">{target.title}</span>
            </p>
          ) : null}
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Magnet URL</label>
            <input
              type="text"
              value={magnetUrl}
              onChange={(e) => setMagnetUrl(e.target.value)}
              placeholder="magnet:?xt=urn:btih:..."
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Or .torrent file
            </label>
            <input
              type="file"
              accept=".torrent,application/x-bittorrent"
              onChange={(e) => void handleFile(e.target.files?.[0] || null)}
              className="w-full text-xs text-slate-300 file:mr-3 file:px-3 file:py-1.5 file:rounded-lg file:border-0 file:bg-slate-800 file:text-slate-200"
            />
            {fileName ? (
              <p className="mt-1 text-[11px] font-mono text-slate-500">{fileName} (base64 ready)</p>
            ) : null}
          </div>
          <div className="pt-2 flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-3 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={busy}
              className="px-4 py-2 rounded-lg bg-amber-500 hover:bg-amber-400 disabled:opacity-50 text-slate-950 text-xs font-bold"
            >
              {busy ? 'Sending…' : 'Add to queue'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
