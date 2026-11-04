import React, { useCallback, useEffect, useState } from 'react';
import { ListTodo, Play } from 'lucide-react';
import { api } from '../../api';
import type { SystemTask } from '../../types';
import { formatAppDate } from '../../uiPrefs';
import type { AppConfig } from '../../types';

export interface SystemTasksPageProps {
  appUi?: AppConfig['ui'] | null;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SystemTasksPage: React.FC<SystemTasksPageProps> = ({ appUi, onNotify }) => {
  const [tasks, setTasks] = useState<SystemTask[]>([]);

  const load = useCallback(async () => {
    try {
      setTasks(await api.getSystemTasks());
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load tasks', true);
    }
  }, [onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  const run = async (name: string) => {
    try {
      await api.runSystemTask(name);
      onNotify(`Started task ${name}`);
      await load();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to run task', true);
    }
  };

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
          <ListTodo className="w-5 h-5 text-indigo-300" />
          Tasks
        </h2>
        <p className="text-xs text-slate-400 mt-1">Scheduled background jobs.</p>
      </div>
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <table className="w-full text-xs">
          <thead className="bg-slate-950/80 text-slate-400 text-left">
            <tr>
              <th className="px-3 py-2">Name</th>
              <th className="px-3 py-2">Interval</th>
              <th className="px-3 py-2">Last run</th>
              <th className="px-3 py-2">Next run</th>
              <th className="px-3 py-2">Status</th>
              <th className="px-3 py-2 w-24" />
            </tr>
          </thead>
          <tbody>
            {tasks.map((t) => (
              <tr key={t.name} className="border-t border-slate-800/80">
                <td className="px-3 py-2 text-slate-200">{t.label || t.name}</td>
                <td className="px-3 py-2 text-slate-400">{t.interval || '—'}</td>
                <td className="px-3 py-2 text-slate-400">
                  {t.lastRunAt ? formatAppDate(t.lastRunAt, appUi, 'short') : '—'}
                  {t.lastDurationMs != null ? ` (${t.lastDurationMs}ms)` : ''}
                </td>
                <td className="px-3 py-2 text-slate-400">
                  {t.nextRunAt ? formatAppDate(t.nextRunAt, appUi, 'short') : '—'}
                </td>
                <td className="px-3 py-2 capitalize text-slate-300">{t.status || 'idle'}</td>
                <td className="px-3 py-2">
                  <button
                    type="button"
                    onClick={() => void run(t.name)}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-indigo-600/20 border border-indigo-500/30 text-indigo-200 hover:bg-indigo-600/30"
                  >
                    <Play className="w-3 h-3" />
                    Run
                  </button>
                </td>
              </tr>
            ))}
            {tasks.length === 0 ? (
              <tr>
                <td colSpan={6} className="px-3 py-6 text-center text-slate-500">
                  No tasks registered.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
};
