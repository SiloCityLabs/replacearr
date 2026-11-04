import React from 'react';
import {
  Tv,
  Film,
  Radio,
  DownloadCloud,
  Puzzle,
  Activity,
  ArrowUpRight,
  Clock,
  FolderGit2,
  Sparkles,
  HardDrive,
} from 'lucide-react';
import type { AppConfig, NavTab, OverviewResponse, QualityProfile, RootFolder } from '../types';
import { formatBytes } from '../api';
import { formatAppDate } from '../uiPrefs';

interface DashboardTabProps {
  overview: OverviewResponse | null;
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onNavigate: (tab: NavTab) => void;
  onAddSeries: () => void;
  onAddMovie: () => void;
  appUi?: AppConfig['ui'] | null;
}

export const DashboardTab: React.FC<DashboardTabProps> = ({
  overview,
  profiles,
  rootFolders,
  onNavigate,
  onAddSeries,
  onAddMovie,
  appUi,
}) => {

  const counts = overview?.counts ?? {
    series: 0,
    episodes: 0,
    movies: 0,
    indexers: 0,
    activeDownloads: 0,
    plugins: 0,
    cardigannDefinitions: 0,
  };

  const recentQueue = overview?.recentQueue ?? [];
  const recentHistory = overview?.recentHistory ?? [];

  return (
    <div className="space-y-6">
      {/* Hero: Add New first; Releases secondary */}
      <div className="relative rounded-2xl bg-gradient-to-r from-slate-900 via-indigo-950/40 to-slate-900 border border-slate-800 px-4 py-3 sm:p-6 overflow-hidden">
        <div className="flex flex-col sm:gap-1.5">
          <div className="hidden sm:flex sm:flex-col sm:gap-1.5">
            <div className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-indigo-500/10 border border-indigo-500/25 text-indigo-300 text-xs font-medium">
              <Sparkles className="w-3.5 h-3.5 text-indigo-400" />
              <span>Unified Media & Indexer Control Plane</span>
            </div>
            <h2 className="text-2xl font-bold text-white tracking-tight">
              Welcome to Replacearr
            </h2>
            <p className="text-sm text-slate-400 max-w-2xl">
              Add series and movies from TVDB / TMDB / IMDb. Use Interactive Search on a library item to find releases.
            </p>
          </div>
          <div className="flex flex-wrap gap-2 sm:pt-2">
            <button
              type="button"
              onClick={onAddSeries}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
            >
              Add Series
            </button>
            <button
              type="button"
              onClick={onAddMovie}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-lg transition"
            >
              Add Movie
            </button>
            <button
              type="button"
              onClick={() => onNavigate('series')}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 text-xs font-semibold rounded-lg transition"
            >
              Browse TV
            </button>
          </div>
        </div>
      </div>

      {/* Unified Metric Cards — compact 2-col tiles on mobile */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2 sm:gap-2.5">
        <button
          type="button"
          onClick={() => onNavigate('series')}
          className="text-left group bg-slate-900/90 hover:bg-slate-900 border border-slate-800 hover:border-indigo-500/40 rounded-xl p-2.5 sm:p-3 transition"
        >
          <div className="flex items-center justify-between gap-1 mb-1.5">
            <span className="text-[11px] sm:text-xs font-medium text-slate-400 truncate">TV Series</span>
            <div className="p-1 rounded-md bg-indigo-500/10 text-indigo-400 border border-indigo-500/20 group-hover:scale-105 transition shrink-0">
              <Tv className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl sm:text-2xl font-bold text-white leading-none">{counts.series}</div>
          <div className="mt-1 flex items-center justify-between gap-1 text-[10px] sm:text-xs text-slate-400">
            <span className="truncate">{counts.episodes} episodes</span>
            <ArrowUpRight className="w-3 h-3 text-indigo-400 opacity-0 group-hover:opacity-100 transition shrink-0" />
          </div>
        </button>

        <button
          type="button"
          onClick={() => onNavigate('movies')}
          className="text-left group bg-slate-900/90 hover:bg-slate-900 border border-slate-800 hover:border-emerald-500/40 rounded-xl p-2.5 sm:p-3 transition"
        >
          <div className="flex items-center justify-between gap-1 mb-1.5">
            <span className="text-[11px] sm:text-xs font-medium text-slate-400 truncate">Movies</span>
            <div className="p-1 rounded-md bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 group-hover:scale-105 transition shrink-0">
              <Film className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl sm:text-2xl font-bold text-white leading-none">{counts.movies}</div>
          <div className="mt-1 flex items-center justify-between gap-1 text-[10px] sm:text-xs text-slate-400">
            <span className="truncate">In library</span>
            <ArrowUpRight className="w-3 h-3 text-emerald-400 opacity-0 group-hover:opacity-100 transition shrink-0" />
          </div>
        </button>

        <button
          type="button"
          onClick={() => onNavigate('indexers')}
          className="text-left group bg-slate-900/90 hover:bg-slate-900 border border-slate-800 hover:border-amber-500/40 rounded-xl p-2.5 sm:p-3 transition"
        >
          <div className="flex items-center justify-between gap-1 mb-1.5">
            <span className="text-[11px] sm:text-xs font-medium text-slate-400 truncate">Indexers</span>
            <div className="p-1 rounded-md bg-amber-500/10 text-amber-400 border border-amber-500/20 group-hover:scale-105 transition shrink-0">
              <Radio className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl sm:text-2xl font-bold text-white leading-none">{counts.indexers}</div>
          <div className="mt-1 flex items-center justify-between gap-1 text-[10px] sm:text-xs text-slate-400">
            <span className="truncate">{counts.cardigannDefinitions} YAML defs</span>
            <ArrowUpRight className="w-3 h-3 text-amber-400 opacity-0 group-hover:opacity-100 transition shrink-0" />
          </div>
        </button>

        <button
          type="button"
          onClick={() => onNavigate('queue')}
          className="text-left group bg-slate-900/90 hover:bg-slate-900 border border-slate-800 hover:border-sky-500/40 rounded-xl p-2.5 sm:p-3 transition"
        >
          <div className="flex items-center justify-between gap-1 mb-1.5">
            <span className="text-[11px] sm:text-xs font-medium text-slate-400 truncate">Active Queue</span>
            <div className="p-1 rounded-md bg-sky-500/10 text-sky-400 border border-sky-500/20 group-hover:scale-105 transition shrink-0">
              <DownloadCloud className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl sm:text-2xl font-bold text-white leading-none">{counts.activeDownloads}</div>
          <div className="mt-1 flex items-center justify-between gap-1 text-[10px] sm:text-xs text-slate-400">
            <span className="truncate">Transfers</span>
            <ArrowUpRight className="w-3 h-3 text-sky-400 opacity-0 group-hover:opacity-100 transition shrink-0" />
          </div>
        </button>

        <button
          type="button"
          onClick={() => onNavigate('plugins')}
          className="text-left group bg-slate-900/90 hover:bg-slate-900 border border-slate-800 hover:border-purple-500/40 rounded-xl p-2.5 sm:p-3 transition"
        >
          <div className="flex items-center justify-between gap-1 mb-1.5">
            <span className="text-[11px] sm:text-xs font-medium text-slate-400 truncate">Go Plugins</span>
            <div className="p-1 rounded-md bg-purple-500/10 text-purple-400 border border-purple-500/20 group-hover:scale-105 transition shrink-0">
              <Puzzle className="w-3.5 h-3.5" />
            </div>
          </div>
          <div className="text-xl sm:text-2xl font-bold text-white leading-none">{counts.plugins}</div>
          <div className="mt-1 flex items-center justify-between gap-1 text-[10px] sm:text-xs text-slate-400">
            <span className="truncate">JSON-RPC SDK</span>
            <ArrowUpRight className="w-3 h-3 text-purple-400 opacity-0 group-hover:opacity-100 transition shrink-0" />
          </div>
        </button>
      </div>

      {/* Active Queue Preview + Recent History */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Active Queue Preview */}
        <div className="lg:col-span-7 bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <DownloadCloud className="w-4 h-4 text-sky-400" />
                <h3 className="text-sm font-semibold text-white">Active Download Queue</h3>
              </div>
              <button
                onClick={() => onNavigate('queue')}
                className="text-xs text-indigo-400 hover:text-indigo-300 font-medium"
              >
                Manage Queue →
              </button>
            </div>

            {recentQueue.length === 0 ? (
              <div className="py-10 text-center border border-dashed border-slate-800 rounded-xl">
                <DownloadCloud className="w-8 h-8 text-slate-600 mx-auto mb-2" />
                <p className="text-sm text-slate-400">Download queue is currently idle.</p>
                <p className="text-xs text-slate-500 mt-1">
                  Grab a release from Interactive Search on a library item to enqueue downloads.
                </p>
              </div>
            ) : (
              <div className="space-y-3">
                {recentQueue.slice(0, 5).map((item) => {
                  const pct =
                    item.size > 0
                      ? Math.min(100, Math.max(5, Math.round(((item.size - item.sizeleft) / item.size) * 100)))
                      : 65;
                  return (
                    <div
                      key={item.id}
                      className="p-3.5 rounded-xl bg-slate-950/80 border border-slate-800/80 space-y-2"
                    >
                      <div className="flex items-center justify-between gap-2">
                        <div className="flex items-center gap-2 min-w-0">
                          <span
                            className={`text-[10px] font-mono uppercase px-2 py-0.5 rounded font-semibold ${
                              item.protocol === 'usenet'
                                ? 'bg-amber-500/15 text-amber-300 border border-amber-500/30'
                                : 'bg-emerald-500/15 text-emerald-300 border border-emerald-500/30'
                            }`}
                          >
                            {item.protocol || 'torrent'}
                          </span>
                          <span className="text-xs font-medium text-slate-200 truncate">
                            {item.title}
                          </span>
                        </div>
                        <span className="text-xs font-mono text-slate-400 shrink-0">
                          {item.timeleft || '00:08:15'} left
                        </span>
                      </div>

                      <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-gradient-to-r from-indigo-500 to-emerald-400 rounded-full transition-all"
                          style={{ width: `${pct}%` }}
                        />
                      </div>

                      <div className="flex items-center justify-between text-[11px] text-slate-400">
                        <span>
                          Client: <strong className="text-slate-300">{item.downloadClient}</strong> •
                          Indexer: <strong className="text-slate-300">{item.indexer || 'Replacearr'}</strong>
                        </span>
                        <span>
                          {formatBytes(item.size)} ({pct}%)
                        </span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Root Folders Storage Summary */}
          <div className="mt-5 pt-4 border-t border-slate-800/80">
            <div className="flex items-center justify-between mb-2.5">
              <span className="text-xs font-medium text-slate-400 inline-flex items-center gap-1.5">
                <HardDrive className="w-3.5 h-3.5 text-slate-400" />
                Root Folders & Quality Profiles ({profiles.length} profiles)
              </span>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              {rootFolders.map((rf) => (
                <div
                  key={rf.id}
                  className="px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 flex items-center justify-between gap-2 text-xs min-w-0"
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <FolderGit2 className="w-3.5 h-3.5 text-indigo-400 shrink-0" />
                    <code className="font-mono text-slate-200 truncate">{rf.path}</code>
                  </div>
                  <span
                    className={`text-[11px] font-mono shrink-0 ${
                      rf.accessible ? 'text-emerald-400' : 'text-rose-300'
                    }`}
                  >
                    {rf.accessible ? `${formatBytes(rf.freeSpace)} free` : 'not accessible'}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>

        {/* Recent Activity & History */}
        <div className="lg:col-span-5 bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col justify-between">
          <div>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Activity className="w-4 h-4 text-emerald-400" />
                <h3 className="text-sm font-semibold text-white">Recent Media Activity</h3>
              </div>
              <span className="text-[11px] font-mono text-slate-400">
                Uptime: {overview?.system?.uptime || 'active'}
              </span>
            </div>

            {recentHistory.length === 0 ? (
              <div className="py-10 text-center border border-dashed border-slate-800 rounded-xl">
                <Clock className="w-8 h-8 text-slate-600 mx-auto mb-2" />
                <p className="text-sm text-slate-400">No recent grab or import events.</p>
              </div>
            ) : (
              <div className="space-y-2.5">
                {recentHistory.slice(0, 6).map((h) => (
                  <div
                    key={h.id}
                    className="p-3 rounded-lg bg-slate-950/70 border border-slate-800/80 flex items-start justify-between gap-3"
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                          {h.eventType || 'grabbed'}
                        </span>
                        <span className="text-[10px] uppercase text-slate-500 font-semibold">
                          {h.mediaType}
                        </span>
                      </div>
                      <p className="text-xs font-medium text-slate-200 truncate mt-1">
                        {h.sourceTitle}
                      </p>
                    </div>
                    <span className="text-[11px] text-slate-500 shrink-0">
                      {h.date ? formatAppDate(h.date, appUi) : 'recent'}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="mt-5 pt-4 border-t border-slate-800/80 flex items-center justify-between text-xs text-slate-400">
            <span>Version {overview?.system?.version || '1.0.0-go'}</span>
            <button
              onClick={() => onNavigate('plugins')}
              className="text-indigo-400 hover:text-indigo-300 font-medium"
            >
              View Plugins →
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
