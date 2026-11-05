import React, { useCallback, useEffect, useState } from 'react';
import { Archive, Download, Trash2, Upload } from 'lucide-react';
import { api, formatBytes } from '../../api';
import type { AppConfig, BackupFile } from '../../types';
import { formatAppDate } from '../../uiPrefs';
import { useConfirm } from '../modals/ConfirmModal';

export interface SystemBackupPageProps {
  config: AppConfig | null;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onNotify: (msg: string, isError?: boolean) => void;
  appUi?: AppConfig['ui'] | null;
}

export const SystemBackupPage: React.FC<SystemBackupPageProps> = ({
  config,
  onSave,
  onNotify,
  appUi,
}) => {
  const { confirm, dialog } = useConfirm();
  const [files, setFiles] = useState<BackupFile[]>([]);
  const [folder, setFolder] = useState(config?.backup.folder ?? 'Backups');
  const [interval, setIntervalDays] = useState(config?.backup.interval ?? 7);
  const [retention, setRetention] = useState(config?.backup.retention ?? 28);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      setFiles(await api.listBackups());
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to list backups', true);
    }
  }, [onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (config) {
      setFolder(config.backup.folder);
      setIntervalDays(config.backup.interval);
      setRetention(config.backup.retention);
    }
  }, [config]);

  const saveSettings = async () => {
    if (!config) return;
    try {
      await onSave({
        ...config,
        backup: { folder, interval, retention },
      });
      onNotify('Backup settings saved');
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Save failed', true);
    }
  };

  const create = async () => {
    setBusy(true);
    try {
      await api.createBackup();
      onNotify('Backup created');
      await load();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Backup failed', true);
    } finally {
      setBusy(false);
    }
  };

  const remove = async (id: string) => {
    const ok = await confirm({
      title: 'Delete backup',
      message: 'Delete this backup?',
      confirmLabel: 'Delete',
    });
    if (!ok) return;
    try {
      await api.deleteBackup(id);
      onNotify('Backup deleted');
      await load();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Delete failed', true);
    }
  };

  const restore = async (id: string) => {
    const ok = await confirm({
      title: 'Restore backup',
      message: 'Restore this backup? The app may need a restart.',
      confirmLabel: 'Restore',
      danger: false,
    });
    if (!ok) return;
    try {
      await api.restoreBackup(id);
      onNotify('Restore queued — restart recommended');
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Restore failed', true);
    }
  };

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
          <Archive className="w-5 h-5 text-indigo-300" />
          Backup
        </h2>
        <p className="text-xs text-slate-400 mt-1">Create and restore configuration and database backups.</p>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <label className="text-xs space-y-1 block">
            <span className="text-slate-400">Folder</span>
            <input
              value={folder}
              onChange={(e) => setFolder(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
            />
          </label>
          <label className="text-xs space-y-1 block">
            <span className="text-slate-400">Interval (days)</span>
            <input
              type="number"
              value={interval}
              onChange={(e) => setIntervalDays(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
            />
          </label>
          <label className="text-xs space-y-1 block">
            <span className="text-slate-400">Retention (days)</span>
            <input
              type="number"
              value={retention}
              onChange={(e) => setRetention(Number(e.target.value))}
              className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-sm text-white"
            />
          </label>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => void saveSettings()}
            className="px-3 py-2 rounded-lg text-xs bg-slate-800 border border-slate-700 text-slate-200"
          >
            Save settings
          </button>
          <button
            type="button"
            disabled={busy}
            onClick={() => void create()}
            className="px-3 py-2 rounded-lg text-xs bg-indigo-600 text-white disabled:opacity-60"
          >
            Backup now
          </button>
        </div>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <table className="w-full text-xs">
          <thead className="bg-slate-950/80 text-slate-400 text-left">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Size</th>
              <th className="px-3 py-2">Created</th>
              <th className="px-3 py-2 w-40" />
            </tr>
          </thead>
          <tbody>
            {files.map((f) => (
              <tr key={f.id} className="border-t border-slate-800/80">
                <td className="px-3 py-2 text-slate-200 font-mono">{f.name}</td>
                <td className="px-3 py-2 text-slate-400">{formatBytes(f.size)}</td>
                <td className="px-3 py-2 text-slate-400">{formatAppDate(f.createdAt, appUi, 'short')}</td>
                <td className="px-3 py-2">
                  <div className="flex items-center gap-1">
                    <a
                      href={api.backupDownloadUrl(f.id)}
                      className="p-1.5 rounded-md text-slate-400 hover:text-indigo-300"
                      title="Download"
                    >
                      <Download className="w-3.5 h-3.5" />
                    </a>
                    <button type="button" title="Restore" onClick={() => void restore(f.id)} className="p-1.5 rounded-md text-slate-400 hover:text-emerald-300">
                      <Upload className="w-3.5 h-3.5" />
                    </button>
                    <button type="button" title="Delete" onClick={() => void remove(f.id)} className="p-1.5 rounded-md text-slate-400 hover:text-rose-300">
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            {files.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-slate-500">
                  No backups yet.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      {dialog}
    </div>
  );
};
