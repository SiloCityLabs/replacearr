import React, { useEffect, useRef, useState } from 'react';
import { Folder, DownloadCloud, CheckCircle2, LogOut, ChevronRight, ChevronLeft, RefreshCw } from 'lucide-react';
import type { AppConfig, DiscoveredDownloadClient } from '../types';
import { api } from '../api';

interface SetupWizardProps {
  config: AppConfig;
  onComplete: (config: AppConfig) => void;
  onLogout: () => void;
  onNotify: (msg: string, isError?: boolean) => void;
}

function protocolForImplementation(implementation: string): string {
  switch (implementation) {
    case 'SABnzbd':
    case 'NZBGet':
      return 'usenet';
    default:
      return 'torrent';
  }
}

export const SetupWizard: React.FC<SetupWizardProps> = ({
  config,
  onComplete,
  onLogout,
  onNotify,
}) => {
  const [step, setStep] = useState(0);
  const [tvPath, setTvPath] = useState('/data/TV');
  const [moviesPath, setMoviesPath] = useState('/data/Movies');
  const [addClient, setAddClient] = useState(false);
  const [clientName, setClientName] = useState('qBittorrent');
  const [clientImplementation, setClientImplementation] = useState('qBittorrent');
  const [clientBaseUrl, setClientBaseUrl] = useState('http://localhost:8080');
  const [downloadPathNote, setDownloadPathNote] = useState('/data/downloads');
  const [busy, setBusy] = useState(false);
  const [discovered, setDiscovered] = useState<DiscoveredDownloadClient[]>([]);
  const [discovering, setDiscovering] = useState(false);
  const [discoverDone, setDiscoverDone] = useState(false);
  const discoverOnceRef = useRef(false);

  const runDiscover = async () => {
    setDiscovering(true);
    try {
      const hits = await api.discoverDownloadClients();
      setDiscovered(hits);
      setDiscoverDone(true);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Client discovery failed', true);
      setDiscoverDone(true);
    } finally {
      setDiscovering(false);
    }
  };

  useEffect(() => {
    if (step !== 2 || discoverOnceRef.current) return;
    discoverOnceRef.current = true;
    void runDiscover();
  }, [step]);

  const applyDiscovered = (hit: DiscoveredDownloadClient) => {
    setAddClient(true);
    setClientName(hit.nameHint || hit.implementation);
    setClientImplementation(hit.implementation);
    setClientBaseUrl(hit.baseUrl);
  };

  const finish = async () => {
    if (!tvPath.trim() || !moviesPath.trim()) {
      onNotify('TV and movies root paths are required', true);
      return;
    }
    setBusy(true);
    try {
      await api.addRootFolder({ path: tvPath.trim(), mediaType: 'tv' });
      await api.addRootFolder({ path: moviesPath.trim(), mediaType: 'movie' });

      if (addClient) {
        const impl = clientImplementation.trim() || 'qBittorrent';
        await api.addDownloadClient({
          name: clientName.trim() || impl,
          implementation: impl,
          implementationName: impl,
          configContract: `${impl}Settings`,
          protocol: protocolForImplementation(impl),
          enable: true,
          priority: 1,
          baseUrl: clientBaseUrl.trim() || 'http://localhost:8080',
          username: '',
          category: 'replacearr',
          removeCompletedDownloads: true,
          removeFailedDownloads: true,
          fields: downloadPathNote.trim()
            ? [{ name: 'downloadPath', label: 'Download Path', type: 'text', value: downloadPathNote.trim() }]
            : [],
          tags: [],
        });
      }

      const saved = await api.saveConfig({
        ...config,
        setupComplete: true,
      });
      onNotify('Setup complete — welcome to Replacearr');
      onComplete(saved);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Setup failed', true);
    } finally {
      setBusy(false);
    }
  };

  const steps = ['TV library path', 'Movies library path', 'Download client'];

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col">
      <header className="h-14 border-b border-slate-800 bg-slate-900/60 px-4 sm:px-6 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <img src="/icon-192.png" alt="Replacearr" className="w-7 h-7 object-contain" />
          <span className="font-bold text-white">Replacearr setup</span>
        </div>
        <button
          type="button"
          onClick={() => void onLogout()}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-300 hover:text-rose-300"
        >
          <LogOut className="w-3.5 h-3.5" />
          Sign out
        </button>
      </header>

      <main className="flex-1 flex items-center justify-center p-4 sm:p-8">
        <div className="w-full max-w-xl bg-slate-900 border border-slate-800 rounded-2xl shadow-xl overflow-hidden">
          <div className="p-5 border-b border-slate-800">
            <h1 className="text-lg font-bold text-white">First-run wizard</h1>
            <p className="text-xs text-slate-400 mt-1">
              Configure library roots before using Replacearr.
            </p>
            <div className="mt-4 flex gap-2">
              {steps.map((label, i) => (
                <div
                  key={label}
                  className={`flex-1 h-1.5 rounded-full ${
                    i <= step ? 'bg-indigo-500' : 'bg-slate-800'
                  }`}
                  title={label}
                />
              ))}
            </div>
            <p className="mt-2 text-[11px] font-mono text-slate-500">
              Step {step + 1} of {steps.length}: {steps[step]}
            </p>
          </div>

          <div className="p-5 space-y-4">
            {step === 0 && (
              <div className="space-y-3">
                <div className="flex items-center gap-2 text-indigo-300">
                  <Folder className="w-4 h-4" />
                  <span className="text-sm font-semibold text-white">TV series root folder</span>
                </div>
                <input
                  type="text"
                  value={tvPath}
                  onChange={(e) => setTvPath(e.target.value)}
                  placeholder="/data/TV"
                  className="w-full px-3.5 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white font-mono placeholder-slate-500 focus:outline-none focus:border-indigo-500"
                />
                <p className="text-xs text-slate-500">
                  Absolute path where TV series folders will live.
                </p>
              </div>
            )}

            {step === 1 && (
              <div className="space-y-3">
                <div className="flex items-center gap-2 text-emerald-300">
                  <Folder className="w-4 h-4" />
                  <span className="text-sm font-semibold text-white">Movies root folder</span>
                </div>
                <input
                  type="text"
                  value={moviesPath}
                  onChange={(e) => setMoviesPath(e.target.value)}
                  placeholder="/data/Movies"
                  className="w-full px-3.5 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white font-mono placeholder-slate-500 focus:outline-none focus:border-emerald-500"
                />
              </div>
            )}

            {step === 2 && (
              <div className="space-y-4">
                <div className="flex items-center gap-2 text-sky-300">
                  <DownloadCloud className="w-4 h-4" />
                  <span className="text-sm font-semibold text-white">
                    Download client (optional)
                  </span>
                </div>

                <div className="rounded-lg border border-slate-800 bg-slate-950/80 p-3 space-y-2">
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs text-slate-400">
                      {discovering
                        ? 'Looking for nearby download clients…'
                        : discoverDone && discovered.length === 0
                          ? 'No clients found on local or SSDP hosts.'
                          : discovered.length > 0
                            ? 'Suggested clients — click to use'
                            : 'Scanning for nearby clients…'}
                    </p>
                    <button
                      type="button"
                      disabled={discovering}
                      onClick={() => void runDiscover()}
                      className="inline-flex items-center gap-1 text-[11px] text-sky-300 hover:text-sky-200 disabled:opacity-40"
                    >
                      <RefreshCw className={`w-3 h-3 ${discovering ? 'animate-spin' : ''}`} />
                      Scan again
                    </button>
                  </div>
                  {discovered.length > 0 && (
                    <ul className="space-y-1.5">
                      {discovered.map((hit) => (
                        <li key={`${hit.implementation}:${hit.baseUrl}`}>
                          <button
                            type="button"
                            onClick={() => applyDiscovered(hit)}
                            className="w-full text-left px-2.5 py-2 rounded-md border border-slate-800 hover:border-sky-500/40 hover:bg-sky-500/5 transition"
                          >
                            <div className="text-xs font-semibold text-white">
                              {hit.nameHint || hit.implementation}
                              <span className="ml-2 text-[10px] font-mono uppercase text-slate-500">
                                {hit.protocol}
                              </span>
                            </div>
                            <div className="text-[11px] font-mono text-slate-400 break-all">
                              {hit.baseUrl}
                            </div>
                            {hit.detail ? (
                              <div className="text-[10px] text-slate-500 mt-0.5">{hit.detail}</div>
                            ) : null}
                          </button>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>

                <label className="flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={addClient}
                    onChange={(e) => setAddClient(e.target.checked)}
                    className="rounded border-slate-700 bg-slate-950 text-indigo-500"
                  />
                  Configure a download client now
                </label>
                {addClient && (
                  <div className="space-y-3 pl-1">
                    <div className="space-y-3">
                      <div>
                        <label className="block text-[11px] text-slate-400 mb-1">Type</label>
                        <select
                          value={clientImplementation}
                          onChange={(e) => {
                            setClientImplementation(e.target.value);
                            if (!clientName.trim() || clientName === clientImplementation) {
                              setClientName(e.target.value);
                            }
                          }}
                          className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                        >
                          <option value="qBittorrent">qBittorrent</option>
                          <option value="Transmission">Transmission</option>
                          <option value="Deluge">Deluge</option>
                          <option value="SABnzbd">SABnzbd</option>
                          <option value="NZBGet">NZBGet</option>
                        </select>
                      </div>
                      <div>
                        <label className="block text-[11px] text-slate-400 mb-1">Name</label>
                        <input
                          type="text"
                          value={clientName}
                          onChange={(e) => setClientName(e.target.value)}
                          className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] text-slate-400 mb-1">Base URL</label>
                        <input
                          type="url"
                          value={clientBaseUrl}
                          onChange={(e) => setClientBaseUrl(e.target.value)}
                          placeholder="http://localhost:8080"
                          className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                        />
                        <p className="mt-1 text-[11px] text-slate-500">
                          Include http/https, port, and any reverse-proxy path.
                        </p>
                      </div>
                      <div>
                        <label className="block text-[11px] text-slate-400 mb-1">
                          Download path (note)
                        </label>
                        <input
                          type="text"
                          value={downloadPathNote}
                          onChange={(e) => setDownloadPathNote(e.target.value)}
                          placeholder="/data/downloads"
                          className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                        />
                      </div>
                    </div>
                    <p className="text-[11px] text-slate-500">
                      Download path is stored as a client note for your reference. Map remote paths
                      later under Settings if the client runs in another container. Add credentials
                      under Queue &amp; Clients after setup if needed.
                    </p>
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="p-5 border-t border-slate-800 flex items-center justify-between gap-2">
            <button
              type="button"
              disabled={step === 0 || busy}
              onClick={() => setStep((s) => Math.max(0, s - 1))}
              className="inline-flex items-center gap-1 px-3 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium disabled:opacity-40"
            >
              <ChevronLeft className="w-3.5 h-3.5" />
              Back
            </button>

            {step < steps.length - 1 ? (
              <button
                type="button"
                disabled={
                  busy ||
                  (step === 0 && !tvPath.trim()) ||
                  (step === 1 && !moviesPath.trim())
                }
                onClick={() => setStep((s) => s + 1)}
                className="inline-flex items-center gap-1 px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 text-white text-xs font-semibold"
              >
                Next
                <ChevronRight className="w-3.5 h-3.5" />
              </button>
            ) : (
              <button
                type="button"
                disabled={busy || !tvPath.trim() || !moviesPath.trim()}
                onClick={() => void finish()}
                className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 disabled:opacity-40 text-white text-xs font-semibold"
              >
                <CheckCircle2 className="w-3.5 h-3.5" />
                {busy ? 'Saving…' : 'Finish setup'}
              </button>
            )}
          </div>
        </div>
      </main>
    </div>
  );
};
