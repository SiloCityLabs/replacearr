import React, { useState } from 'react';
import {
  Link2,
  Copy,
  Check,
  Key,
  Tv,
  Film,
  Radio,
  Smartphone,
  Subtitles,
  Sparkles,
  CheckCircle2,
} from 'lucide-react';

interface CompatHubTabProps {
  masterApiKey: string;
}

export const CompatHubTab: React.FC<CompatHubTabProps> = ({ masterApiKey }) => {
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const origin =
    typeof window !== 'undefined' && window.location.origin
      ? window.location.origin
      : 'http://localhost:8989';

  const copy = (value: string, id: string) => {
    void navigator.clipboard.writeText(value);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const endpoints = [
    {
      id: 'sonarr-base',
      app: 'Sonarr v3 Emulation',
      accent: 'indigo',
      icon: Tv,
      baseUrl: `${origin}/sonarr`,
      apiStatusUrl: `${origin}/sonarr/api/v3/system/status`,
      description:
        'Drop-in Sonarr v3 API surface (`/sonarr/api/v3/series`, `/sonarr/api/v3/episode`, `/sonarr/api/v3/qualityprofile`, `/sonarr/api/v3/rootfolder`, `/sonarr/api/v3/queue`, `/sonarr/api/v3/command`).',
    },
    {
      id: 'radarr-base',
      app: 'Radarr v3 Emulation',
      accent: 'emerald',
      icon: Film,
      baseUrl: `${origin}/radarr`,
      apiStatusUrl: `${origin}/radarr/api/v3/system/status`,
      description:
        'Drop-in Radarr v3 API surface (`/radarr/api/v3/movie`, `/radarr/api/v3/movie/lookup`, `/radarr/api/v3/qualityprofile`, `/radarr/api/v3/rootfolder`, `/radarr/api/v3/queue`, `/radarr/api/v3/command`).',
    },
    {
      id: 'prowlarr-base',
      app: 'Prowlarr v1 + Torznab Feed',
      accent: 'amber',
      icon: Radio,
      baseUrl: `${origin}/prowlarr`,
      apiStatusUrl: `${origin}/prowlarr/api/v1/system/status`,
      torznabUrl: `${origin}/prowlarr/1/api?t=search&apikey=${masterApiKey || 'YOUR_API_KEY'}`,
      description:
        'Drop-in Prowlarr v1 API & Newznab/Torznab XML feed (`/prowlarr/{indexerId}/api?t=caps|search|tvsearch|movie`).',
    },
  ];

  return (
    <div className="space-y-6">
      {/* Master API Key Banner */}
      <div className="bg-gradient-to-r from-slate-900 via-indigo-950/40 to-slate-900 border border-slate-800 rounded-2xl p-6">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
          <div className="space-y-1">
            <div className="inline-flex items-center gap-2 text-xs font-medium text-indigo-300">
              <Key className="w-4 h-4 text-amber-400" />
              <span>Unified *arr Ecosystem Compatibility Hub</span>
            </div>
            <h2 className="text-xl font-bold text-white">
              Connect Overseerr, Jellyseerr, Bazarr & nzb360
            </h2>
            <p className="text-xs text-slate-400 max-w-2xl">
              Point any third-party *arr tool to Replacearr using the URLs below and your Master API
              Key (via <code className="font-mono text-slate-200">X-Api-Key</code> header or{' '}
              <code className="font-mono text-slate-200">?apikey=</code> query parameter).
            </p>
          </div>

          <div className="bg-slate-950 border border-slate-800 rounded-xl p-3.5 flex items-center gap-3 min-w-0 w-full lg:max-w-md">
            <div className="min-w-0 flex-1">
              <div className="text-[10px] font-mono uppercase tracking-wider text-slate-400">
                System Master API Key
              </div>
              <code className="block text-sm font-mono font-bold text-emerald-400 select-all truncate">
                {masterApiKey || 'replacearr-master-api-key'}
              </code>
            </div>
            <button
              onClick={() => copy(masterApiKey || 'replacearr-master-api-key', 'master-key')}
              className="px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold inline-flex items-center gap-1.5 transition shrink-0"
            >
              {copiedId === 'master-key' ? (
                <>
                  <Check className="w-3.5 h-3.5" />
                  <span>Copied</span>
                </>
              ) : (
                <>
                  <Copy className="w-3.5 h-3.5" />
                  <span>Copy Key</span>
                </>
              )}
            </button>
          </div>
        </div>
      </div>

      {/* Ready-to-Copy Base URLs */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {endpoints.map((ep) => {
          const Icon = ep.icon;
          return (
            <div
              key={ep.id}
              className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 flex flex-col justify-between space-y-4"
            >
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <div className="p-2 rounded-lg bg-slate-950 border border-slate-800 text-indigo-400">
                      <Icon className="w-4 h-4" />
                    </div>
                    <h3 className="text-sm font-bold text-white">{ep.app}</h3>
                  </div>
                  <span className="inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                    <CheckCircle2 className="w-3 h-3" /> Active
                  </span>
                </div>

                <p className="text-xs text-slate-400 leading-relaxed">{ep.description}</p>

                <div className="space-y-2 pt-1">
                  <div>
                    <div className="text-[11px] text-slate-400 mb-1">Base URL (for *arr clients):</div>
                    <div className="flex items-center justify-between gap-2 p-2 rounded-lg bg-slate-950 border border-slate-800">
                      <code className="text-xs font-mono text-indigo-300 truncate min-w-0">
                        {ep.baseUrl}
                      </code>
                      <button
                        onClick={() => copy(ep.baseUrl, ep.id)}
                        className="p-1 text-slate-400 hover:text-white"
                      >
                        {copiedId === ep.id ? (
                          <Check className="w-3.5 h-3.5 text-emerald-400" />
                        ) : (
                          <Copy className="w-3.5 h-3.5" />
                        )}
                      </button>
                    </div>
                  </div>

                  <div>
                    <div className="text-[11px] text-slate-400 mb-1">System Status Endpoint:</div>
                    <div className="flex items-center justify-between gap-2 p-2 rounded-lg bg-slate-950 border border-slate-800">
                      <code className="text-[11px] font-mono text-slate-300 truncate min-w-0">
                        {ep.apiStatusUrl}
                      </code>
                      <button
                        onClick={() => copy(ep.apiStatusUrl, `${ep.id}-status`)}
                        className="p-1 text-slate-400 hover:text-white"
                      >
                        {copiedId === `${ep.id}-status` ? (
                          <Check className="w-3.5 h-3.5 text-emerald-400" />
                        ) : (
                          <Copy className="w-3.5 h-3.5" />
                        )}
                      </button>
                    </div>
                  </div>

                  {ep.torznabUrl && (
                    <div>
                      <div className="text-[11px] text-slate-400 mb-1">Torznab XML Feed URL:</div>
                      <div className="flex items-center justify-between gap-2 p-2 rounded-lg bg-slate-950 border border-slate-800">
                        <code className="text-[11px] font-mono text-amber-300 truncate min-w-0">
                          {ep.torznabUrl}
                        </code>
                        <button
                          onClick={() => copy(ep.torznabUrl!, `${ep.id}-torznab`)}
                          className="p-1 text-slate-400 hover:text-white"
                        >
                          {copiedId === `${ep.id}-torznab` ? (
                            <Check className="w-3.5 h-3.5 text-emerald-400" />
                          ) : (
                            <Copy className="w-3.5 h-3.5" />
                          )}
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>

      {/* Step-by-Step Integration Guides */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-3">
          <div className="flex items-center gap-2 text-indigo-400">
            <Sparkles className="w-4 h-4" />
            <h4 className="text-sm font-bold text-white">Overseerr & Jellyseerr</h4>
          </div>
          <ol className="text-xs text-slate-400 space-y-2 list-decimal list-inside">
            <li>
              In Overseerr/Jellyseerr Settings → Services, click <strong>Add Sonarr Server</strong>.
            </li>
            <li>
              Set Hostname/IP to your Replacearr host, Port <code className="font-mono text-slate-200">8989</code>, and Base URL to{' '}
              <code className="font-mono text-indigo-300">/sonarr</code>.
            </li>
            <li>
              Repeat for <strong>Add Radarr Server</strong> using Base URL{' '}
              <code className="font-mono text-emerald-300">/radarr</code> and the same API Key.
            </li>
          </ol>
        </div>

        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-3">
          <div className="flex items-center gap-2 text-emerald-400">
            <Subtitles className="w-4 h-4" />
            <h4 className="text-sm font-bold text-white">Bazarr Subtitle Manager</h4>
          </div>
          <ol className="text-xs text-slate-400 space-y-2 list-decimal list-inside">
            <li>
              Open Bazarr Settings → Sonarr and enable Sonarr integration.
            </li>
            <li>
              Set Base URL to <code className="font-mono text-indigo-300">/sonarr</code> and paste your Replacearr API Key.
            </li>
            <li>
              Open Bazarr Settings → Radarr, set Base URL to{' '}
              <code className="font-mono text-emerald-300">/radarr</code>, and click Test.
            </li>
          </ol>
        </div>

        <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-3">
          <div className="flex items-center gap-2 text-amber-400">
            <Smartphone className="w-4 h-4" />
            <h4 className="text-sm font-bold text-white">nzb360 / LunaSea / Prowlarr</h4>
          </div>
          <ol className="text-xs text-slate-400 space-y-2 list-decimal list-inside">
            <li>
              Configure Sonarr (<code className="font-mono text-indigo-300 break-all">{origin}/sonarr</code>), Radarr (<code className="font-mono text-emerald-300 break-all">{origin}/radarr</code>), and Prowlarr (<code className="font-mono text-amber-300 break-all">{origin}/prowlarr</code>).
            </li>
            <li>
              Or use the root API directly (<code className="font-mono text-slate-200">/api/v3/*</code> and <code className="font-mono text-slate-200">/api/v1/*</code>) which automatically routes by resource path.
            </li>
          </ol>
        </div>
      </div>

      <div className="bg-slate-900/70 border border-slate-800 rounded-xl p-4 flex items-start gap-2 text-xs text-slate-400">
        <Link2 className="w-4 h-4 text-indigo-400 shrink-0 mt-0.5" />
        <span>
          All compatibility endpoints accept <code className="font-mono text-slate-200">X-Api-Key</code> header, <code className="font-mono text-slate-200">?apikey=</code> query parameter, or Bearer JWT.
        </span>
      </div>
    </div>
  );
};
