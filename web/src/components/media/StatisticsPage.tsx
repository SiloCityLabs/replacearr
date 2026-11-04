import React, { useEffect, useState } from 'react';
import { AlertTriangle, BarChart3 } from 'lucide-react';
import { api, formatBytes } from '../../api';
import type { MediaNavType } from '../../nav';
import { mediaTypeApi } from '../../nav';
import type { MediaStatistics } from '../../types';
import { BarChart, DonutChart } from './StatCharts';

export interface StatisticsPageProps {
  media: MediaNavType;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const StatisticsPage: React.FC<StatisticsPageProps> = ({ media, onNotify }) => {
  const [stats, setStats] = useState<MediaStatistics | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    api
      .getStatistics(mediaTypeApi(media))
      .then((data) => {
        if (!cancelled) setStats(data);
      })
      .catch((err) => {
        if (!cancelled) onNotify(err instanceof Error ? err.message : 'Failed to load statistics', true);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [media, onNotify]);

  if (loading) {
    return <p className="text-sm text-slate-400">Loading statistics…</p>;
  }
  if (!stats) {
    return <p className="text-sm text-slate-400">No statistics available.</p>;
  }

  const cards = [
    { label: 'Library items', value: stats.itemCount },
    { label: 'Monitored', value: stats.monitoredCount },
    { label: 'With files', value: stats.withFileCount },
    { label: 'Missing', value: stats.missingCount },
    { label: 'Unreleased', value: stats.unreleasedCount },
    { label: 'Size on disk', value: formatBytes(stats.sizeOnDisk) },
    { label: 'Avg file size', value: formatBytes(stats.averageFileSize || 0) },
  ];

  const qualityBars =
    stats.qualityBreakdown?.map((q) => ({ name: q.name, count: q.count })) ?? [];

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
          <BarChart3 className="w-5 h-5 text-indigo-300" />
          Statistics
        </h2>
        <p className="text-xs text-slate-400 mt-1">Library totals and breakdowns for this media type.</p>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-7 gap-3">
        {cards.map((c) => (
          <div key={c.label} className="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
            <div className="text-[10px] uppercase tracking-wide text-slate-500">{c.label}</div>
            <div className="text-xl font-bold text-white mt-1">{c.value}</div>
          </div>
        ))}
      </div>

      {media === 'series' ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <DonutChart title="Series by status" data={stats.statusBreakdown ?? []} />
          <DonutChart title="Series type" data={stats.seriesTypeBreakdown ?? []} />
          <DonutChart title="Episodes" data={stats.episodeStatusBreakdown ?? []} />
          <BarChart title="Quality profiles" data={stats.qualityProfiles ?? []} />
          <div className="lg:col-span-2">
            <BarChart title="Episode qualities" data={qualityBars} />
          </div>
        </div>
      ) : null}

      {media === 'movies' ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <DonutChart title="Movies by status" data={stats.statusBreakdown ?? []} />
          <BarChart title="Quality profiles" data={stats.qualityProfiles ?? []} />
          <div className="lg:col-span-2">
            <BarChart title="Genres" data={stats.genres ?? []} />
          </div>
        </div>
      ) : null}

      {media === 'books' ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <DonutChart title="Authors by status" data={stats.statusBreakdown ?? []} />
          <BarChart title="Genres" data={stats.genres ?? []} />
          <div className="lg:col-span-2">
            <BarChart title="File types" data={stats.fileTypes ?? []} />
          </div>
        </div>
      ) : null}

      {media === 'music' ? (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <DonutChart title="Artists by status" data={stats.statusBreakdown ?? []} />
          <BarChart title="Genres" data={stats.genres ?? []} />
        </div>
      ) : null}

      {stats.missingItems && stats.missingItems.length > 0 ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
          <div className="px-4 py-3 border-b border-slate-800 flex items-center gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-400" />
            <h3 className="text-sm font-semibold text-white">Missing</h3>
          </div>
          <div className="overflow-x-auto max-h-80">
            <table className="w-full text-xs">
              <thead className="bg-slate-950/80 text-slate-400 text-left sticky top-0">
                <tr>
                  <th className="px-3 py-2">Title</th>
                  <th className="px-3 py-2">Detail</th>
                </tr>
              </thead>
              <tbody>
                {stats.missingItems.map((m, i) => (
                  <tr key={`${m.title}-${i}`} className="border-t border-slate-800/80">
                    <td className="px-3 py-2 text-slate-200">{m.title}</td>
                    <td className="px-3 py-2 text-slate-400">{m.detail || '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : null}
    </div>
  );
};
