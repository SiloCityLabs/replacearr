import React, { useEffect, useState } from 'react';
import { Activity, ExternalLink, AlertTriangle, CheckCircle2, XCircle, Github, Heart } from 'lucide-react';
import type { OverviewResponse, HealthCheck } from '../../types';
import { api } from '../../api';

const LINKS = [
  { label: 'Website', href: 'https://replacearr.com' },
  { label: 'Docker Hub', href: 'https://hub.docker.com/r/ldrrp/replacearr' },
  { label: 'Discord', href: 'https://discord.gg/WUrKJuE5XB' },
  { label: 'Feature requests', href: 'https://github.com/replacearr/replacearr/issues' },
  { label: 'Source', href: 'https://github.com/replacearr/replacearr' },
];

/** Brand marks for funding links (kept in sync with .github/FUNDING.yml). */
const PayPalIcon: React.FC<{ className?: string }> = ({ className }) => (
  <svg viewBox="0 0 24 24" className={className} aria-hidden="true" fill="currentColor">
    <path d="M7.076 21.337H2.47a.641.641 0 0 1-.633-.74L4.944.901C5.026.382 5.474 0 5.998 0h7.46c2.57 0 4.578.543 5.69 1.81 1.01 1.15 1.304 2.42 1.012 4.287-.023.143-.047.29-.077.437-.983 5.05-4.349 6.797-8.647 6.797h-2.19c-.524 0-.968.382-1.05.9l-1.12 7.106zm14.146-14.42a11.86 11.86 0 0 1-.133.96c-1.288 6.62-5.687 8.97-11.21 8.97H7.79l-1.35 8.55a.641.641 0 0 0 .633.74h3.98c.46 0 .85-.334.922-.788l.038-.198.728-4.612.047-.255a.922.922 0 0 1 .922-.788h.58c3.76 0 6.705-1.528 7.565-5.946.36-1.847.174-3.388-.773-4.473-.29-.332-.65-.61-1.06-.84z" />
  </svg>
);

const CashAppIcon: React.FC<{ className?: string }> = ({ className }) => (
  <svg viewBox="0 0 24 24" className={className} aria-hidden="true" fill="currentColor">
    <path d="M23.59 3.47a5.1 5.1 0 0 0-3.05-3.05c-2.68-1-13.4-.98-16.1 0a5.1 5.1 0 0 0-3.05 3.05c-1 2.68-1 13.4 0 16.08a5.1 5.1 0 0 0 3.05 3.05c2.68 1 13.4.98 16.1 0a5.1 5.1 0 0 0 3.05-3.05c.98-2.68.98-13.37 0-16.08zM9.67 18.13c-1.94 0-3.44-.57-4.5-1.48l.9-1.73c.9.72 2.05 1.2 3.34 1.2 1.4 0 2.3-.64 2.3-1.62 0-.9-.66-1.4-2.3-1.95l-1.4-.48c-2.3-.78-3.44-1.95-3.44-3.95 0-2.2 1.8-3.8 4.4-3.8 1.72 0 3.05.5 4.02 1.25l-.95 1.7c-.82-.58-1.85-.98-3-.98-1.35 0-2.12.66-2.12 1.5 0 .86.66 1.3 2.25 1.84l1.4.48c2.5.86 3.55 2.05 3.55 4.12-.02 2.4-1.85 4-4.75 4z" />
  </svg>
);

const SPONSORS = [
  {
    label: 'GitHub Sponsors',
    href: 'https://github.com/sponsors/ldrrp',
    icon: <Github className="w-3.5 h-3.5" />,
    className: 'hover:border-slate-400/50 hover:text-white',
  },
  {
    label: 'PayPal',
    href: 'https://paypal.me/ldrrp',
    icon: <PayPalIcon className="w-3.5 h-3.5 text-[#0070ba]" />,
    className: 'hover:border-[#0070ba]/50 hover:text-white',
  },
  {
    label: 'Cash App',
    href: 'https://cash.app/$ldrrp',
    icon: <CashAppIcon className="w-3.5 h-3.5 text-[#00D632]" />,
    className: 'hover:border-[#00D632]/50 hover:text-white',
  },
] as const;

export interface SystemStatusPageProps {
  overview: OverviewResponse | null;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SystemStatusPage: React.FC<SystemStatusPageProps> = ({ overview, onNotify }) => {
  const [checks, setChecks] = useState<HealthCheck[]>([]);

  useEffect(() => {
    api
      .getHealthChecks()
      .then(setChecks)
      .catch((err) => onNotify(err instanceof Error ? err.message : 'Health checks failed', true));
  }, [onNotify]);

  const sys = overview?.system;

  const statusIcon = (t: string) => {
    if (t === 'error') return <XCircle className="w-3.5 h-3.5 text-rose-400" />;
    if (t === 'warning') return <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />;
    return <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />;
  };

  return (
    <div className="space-y-5">
      <div>
        <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
          <Activity className="w-5 h-5 text-indigo-300" />
          Status
        </h2>
        <p className="text-xs text-slate-400 mt-1">Runtime info, health checks, and project links.</p>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {[
          { label: 'Status', value: 'ONLINE' },
          { label: 'Version', value: sys?.version ?? '—' },
          { label: 'Database', value: sys?.sqliteWal === false ? 'SQLite' : 'SQLite WAL' },
          { label: 'Uptime', value: sys?.uptime ?? '—' },
        ].map((c) => (
          <div key={c.label} className="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
            <div className="text-[10px] uppercase text-slate-500">{c.label}</div>
            <div className="text-sm font-semibold text-white mt-1 break-all">{c.value}</div>
          </div>
        ))}
      </div>

      {sys?.compatEndpoints ? (
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
          <h3 className="text-sm font-semibold text-white mb-2">Compat endpoints</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-1 text-xs font-mono text-slate-400">
            {Array.isArray(sys.compatEndpoints)
              ? sys.compatEndpoints.map((v) => (
                  <div key={v}>{v}</div>
                ))
              : Object.entries(sys.compatEndpoints).map(([k, v]) => (
                  <div key={k}>
                    <span className="text-slate-500">{k}:</span> {String(v)}
                  </div>
                ))}
          </div>
        </div>
      ) : null}

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <div className="px-4 py-3 border-b border-slate-800">
          <h3 className="text-sm font-semibold text-white">Health</h3>
        </div>
        {checks.length === 0 ? (
          <p className="p-4 text-xs text-slate-400">No health issues reported.</p>
        ) : (
          <table className="w-full text-xs">
            <thead className="bg-slate-950/80 text-slate-400 text-left">
              <tr>
                <th className="px-3 py-2 w-8" />
                <th className="px-3 py-2">Source</th>
                <th className="px-3 py-2">Type</th>
                <th className="px-3 py-2">Message</th>
              </tr>
            </thead>
            <tbody>
              {checks.map((c, i) => (
                <tr key={`${c.source}-${i}`} className="border-t border-slate-800/80">
                  <td className="px-3 py-2">{statusIcon(c.type)}</td>
                  <td className="px-3 py-2 text-slate-200">{c.source}</td>
                  <td className="px-3 py-2 capitalize text-slate-400">{c.type}</td>
                  <td className="px-3 py-2 text-slate-300">{c.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <h3 className="text-sm font-semibold text-white mb-3 inline-flex items-center gap-1.5">
          <Heart className="w-3.5 h-3.5 text-rose-400" />
          Sponsor
        </h3>
        <p className="text-xs text-slate-400 mb-3">Support Replacearr development via GitHub Sponsors, PayPal, or Cash App.</p>
        <div className="flex flex-wrap gap-2">
          {SPONSORS.map((s) => (
            <a
              key={s.href}
              href={s.href}
              target="_blank"
              rel="noreferrer"
              className={`inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs bg-slate-950 border border-slate-800 text-slate-300 ${s.className}`}
            >
              {s.icon}
              {s.label}
              <ExternalLink className="w-3 h-3 opacity-60" />
            </a>
          ))}
        </div>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <h3 className="text-sm font-semibold text-white mb-3">More info</h3>
        <div className="flex flex-wrap gap-2">
          {LINKS.map((l) => (
            <a
              key={l.href}
              href={l.href}
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg text-xs bg-slate-950 border border-slate-800 text-slate-300 hover:border-indigo-500/40 hover:text-white"
            >
              {l.label}
              <ExternalLink className="w-3 h-3" />
            </a>
          ))}
        </div>
      </div>
    </div>
  );
};
