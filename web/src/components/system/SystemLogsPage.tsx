import React, { useCallback, useEffect, useState } from 'react';
import { ScrollText, RefreshCw } from 'lucide-react';
import { api } from '../../api';
import type { AppConfig, LogEntry } from '../../types';
import { formatAppDate } from '../../uiPrefs';

const LEVELS = ['', 'debug', 'info', 'warn', 'error'] as const;

export interface SystemLogsPageProps {
  appUi?: AppConfig['ui'] | null;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SystemLogsPage: React.FC<SystemLogsPageProps> = ({ appUi, onNotify }) => {
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [level, setLevel] = useState('');
  const [auto, setAuto] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await api.getSystemLogs({ level: level || undefined, limit: 200 });
      setEntries(data);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load logs', true);
    }
  }, [level, onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!auto) return;
    const t = setInterval(() => void load(), 3000);
    return () => clearInterval(t);
  }, [auto, load]);

  const levelColor = (l: string) => {
    switch (l) {
      case 'error':
        return 'text-rose-300';
      case 'warn':
        return 'text-amber-300';
      case 'debug':
        return 'text-slate-500';
      default:
        return 'text-slate-300';
    }
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
            <ScrollText className="w-5 h-5 text-indigo-300" />
            Logs
          </h2>
          <p className="text-xs text-slate-400 mt-1">In-memory application log buffer.</p>
        </div>
        <div className="flex items-center gap-2">
          <select
            value={level}
            onChange={(e) => setLevel(e.target.value)}
            className="bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
          >
            {LEVELS.map((l) => (
              <option key={l || 'all'} value={l}>
                {l || 'all levels'}
              </option>
            ))}
          </select>
          <label className="inline-flex items-center gap-1.5 text-xs text-slate-400">
            <input type="checkbox" checked={auto} onChange={(e) => setAuto(e.target.checked)} />
            Auto-refresh
          </label>
          <button
            type="button"
            onClick={() => void load()}
            className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden max-h-[32rem] overflow-y-auto">
        <table className="w-full text-xs font-mono">
          <thead className="bg-slate-950/80 text-slate-400 text-left sticky top-0">
            <tr>
              <th className="px-3 py-2">Time</th>
              <th className="px-3 py-2">Level</th>
              <th className="px-3 py-2">Logger</th>
              <th className="px-3 py-2">Message</th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e, i) => (
              <tr key={`${e.time}-${i}`} className="border-t border-slate-800/60">
                <td className="px-3 py-1.5 text-slate-500 whitespace-nowrap">
                  {formatAppDate(e.time, appUi, 'short')}
                </td>
                <td className={`px-3 py-1.5 uppercase ${levelColor(e.level)}`}>{e.level}</td>
                <td className="px-3 py-1.5 text-slate-500">{e.logger || 'app'}</td>
                <td className="px-3 py-1.5 text-slate-200 break-all">{e.message}</td>
              </tr>
            ))}
            {entries.length === 0 ? (
              <tr>
                <td colSpan={4} className="px-3 py-6 text-center text-slate-500 font-sans">
                  No log entries.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
};
