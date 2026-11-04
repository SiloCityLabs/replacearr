import React, { useEffect, useState } from 'react';
import { RefreshCw, ExternalLink } from 'lucide-react';
import { api } from '../../api';
import type { UpdateStatus } from '../../types';

export interface SystemUpdatesPageProps {
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SystemUpdatesPage: React.FC<SystemUpdatesPageProps> = ({ onNotify }) => {
  const [status, setStatus] = useState<UpdateStatus | null>(null);
  const [checking, setChecking] = useState(false);

  const check = async () => {
    setChecking(true);
    try {
      setStatus(await api.getUpdateStatus());
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to check updates', true);
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    api
      .getUpdateStatus()
      .then(setStatus)
      .catch((err) => onNotify(err instanceof Error ? err.message : 'Failed to check updates', true));
  }, [onNotify]);

  return (
    <div className="space-y-5">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
            <RefreshCw className="w-5 h-5 text-indigo-300" />
            Updates
          </h2>
          <p className="text-xs text-slate-400 mt-1">
            Check Docker image update status. Apply updates yourself via Docker Hub or GitHub.
          </p>
        </div>
        <button
          type="button"
          onClick={() => void check()}
          disabled={checking}
          className="shrink-0 px-3 py-2 rounded-lg text-xs bg-slate-800 border border-slate-700 text-slate-200 disabled:opacity-50"
        >
          {checking ? 'Checking…' : 'Check again'}
        </button>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-sm">
          <div>
            <div className="text-[10px] uppercase text-slate-500">Current</div>
            <div className="text-white font-semibold">{status?.currentVersion ?? '—'}</div>
          </div>
          <div>
            <div className="text-[10px] uppercase text-slate-500">Latest</div>
            <div className="text-white font-semibold">{status?.latestVersion ?? '—'}</div>
          </div>
          <div>
            <div className="text-[10px] uppercase text-slate-500">Available</div>
            <div className={`font-semibold ${status?.updateAvailable ? 'text-amber-300' : 'text-emerald-300'}`}>
              {status == null ? '—' : status.updateAvailable ? 'Yes' : 'Up to date'}
            </div>
          </div>
        </div>
        {status?.checkedAt ? (
          <p className="text-[11px] text-slate-500">
            Checked {new Date(status.checkedAt).toLocaleString()}
          </p>
        ) : null}
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 space-y-3">
        <p className="text-xs text-slate-400">
          Replacearr does not apply updates in-app. Pull a new image and recreate the container, or grab a
          release binary from GitHub.
        </p>
        <code className="block text-xs bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-indigo-200 font-mono">
          docker pull ldrrp/replacearr:latest
        </code>
        <div className="flex flex-wrap gap-4">
          <a
            href="https://hub.docker.com/r/ldrrp/replacearr"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 text-xs text-indigo-300 hover:underline"
          >
            Docker Hub
            <ExternalLink className="w-3 h-3" />
          </a>
          <a
            href="https://github.com/replacearr/replacearr/releases"
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 text-xs text-indigo-300 hover:underline"
          >
            GitHub Releases
            <ExternalLink className="w-3 h-3" />
          </a>
        </div>
      </div>
    </div>
  );
};
