import React, { useState } from 'react';
import { Folder, DownloadCloud, CheckCircle2, LogOut, ChevronRight, ChevronLeft } from 'lucide-react';
import type { AppConfig } from '../types';
import { api } from '../api';

interface SetupWizardProps {
  config: AppConfig;
  onComplete: (config: AppConfig) => void;
  onLogout: () => void;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SetupWizard: React.FC<SetupWizardProps> = ({
  config,
  onComplete,
  onLogout,
  onNotify,
}) => {
  const [step, setStep] = useState(0);
  const [tvPath, setTvPath] = useState('');
  const [moviesPath, setMoviesPath] = useState('');
  const [addClient, setAddClient] = useState(false);
  const [clientName, setClientName] = useState('qBittorrent');
  const [clientHost, setClientHost] = useState('localhost');
  const [clientPort, setClientPort] = useState(8080);
  const [downloadPathNote, setDownloadPathNote] = useState('/data/downloads');
  const [busy, setBusy] = useState(false);

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
        await api.addDownloadClient({
          name: clientName.trim() || 'qBittorrent',
          implementation: 'qBittorrent',
          implementationName: 'qBittorrent',
          configContract: 'QBittorrentSettings',
          protocol: 'torrent',
          enable: true,
          priority: 1,
          host: clientHost.trim() || 'localhost',
          port: clientPort || 8080,
          useSsl: false,
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
              Configure library roots before using Replacearr. No default media paths are assumed.
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
                  placeholder="/media/tv"
                  className="w-full px-3.5 py-2.5 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white font-mono placeholder-slate-500 focus:outline-none focus:border-indigo-500"
                />
                <p className="text-xs text-slate-500">
                  Absolute path where TV series folders will live. Leave empty until you know the path —
                  then enter it here.
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
                  placeholder="/media/movies"
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
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
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
                        <label className="block text-[11px] text-slate-400 mb-1">Host</label>
                        <input
                          type="text"
                          value={clientHost}
                          onChange={(e) => setClientHost(e.target.value)}
                          className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                        />
                      </div>
                      <div>
                        <label className="block text-[11px] text-slate-400 mb-1">Port</label>
                        <input
                          type="number"
                          value={clientPort}
                          onChange={(e) => setClientPort(Number(e.target.value))}
                          className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                        />
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
                      later under Settings if the client runs in another container.
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
