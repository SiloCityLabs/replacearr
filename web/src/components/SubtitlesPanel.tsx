import React, { useCallback, useEffect, useState } from 'react';
import { Captions, Download, Loader2, Search, Sparkles } from 'lucide-react';
import { api } from '../api';
import type { SubtitleCandidate, SubtitlesStatus } from '../types';

interface SubtitlesPanelProps {
  kind: 'movie' | 'episode';
  id: number;
  hasFile: boolean;
  compact?: boolean;
  onNotify?: (msg: string, isError?: boolean) => void;
}

export const SubtitlesPanel: React.FC<SubtitlesPanelProps> = ({
  kind,
  id,
  hasFile,
  compact,
  onNotify,
}) => {
  const [status, setStatus] = useState<SubtitlesStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [searching, setSearching] = useState(false);
  const [ensuring, setEnsuring] = useState(false);
  const [candidates, setCandidates] = useState<SubtitleCandidate[]>([]);
  const [busyFileId, setBusyFileId] = useState<number | null>(null);

  const load = useCallback(async () => {
    if (!hasFile) {
      setStatus(null);
      return;
    }
    setLoading(true);
    try {
      const next =
        kind === 'movie' ? await api.getMovieSubtitles(id) : await api.getEpisodeSubtitles(id);
      setStatus(next);
    } catch (err) {
      onNotify?.(err instanceof Error ? err.message : 'Failed to load subtitles', true);
    } finally {
      setLoading(false);
    }
  }, [hasFile, id, kind, onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  if (!hasFile) {
    return null;
  }

  const search = async () => {
    setSearching(true);
    setCandidates([]);
    try {
      const next =
        kind === 'movie' ? await api.searchMovieSubtitles(id) : await api.searchEpisodeSubtitles(id);
      setCandidates(next);
      if (next.length === 0) onNotify?.('No subtitle matches found');
    } catch (err) {
      onNotify?.(err instanceof Error ? err.message : 'Subtitle search failed', true);
    } finally {
      setSearching(false);
    }
  };

  const ensure = async () => {
    setEnsuring(true);
    try {
      if (kind === 'movie') await api.ensureMovieSubtitles(id);
      else await api.ensureEpisodeSubtitles(id);
      await load();
      onNotify?.('Downloaded missing subtitles when available');
    } catch (err) {
      onNotify?.(err instanceof Error ? err.message : 'Auto-download failed', true);
    } finally {
      setEnsuring(false);
    }
  };

  const download = async (c: SubtitleCandidate) => {
    setBusyFileId(c.fileId);
    try {
      if (kind === 'movie') await api.downloadMovieSubtitle(id, c.fileId, c.language);
      else await api.downloadEpisodeSubtitle(id, c.fileId, c.language);
      setCandidates([]);
      await load();
      onNotify?.(`Downloaded ${c.language} subtitle`);
    } catch (err) {
      onNotify?.(err instanceof Error ? err.message : 'Download failed', true);
    } finally {
      setBusyFileId(null);
    }
  };

  return (
    <div
      className={
        compact
          ? 'mt-2 rounded-lg border border-slate-800 bg-slate-950/50 p-3'
          : 'mt-4 rounded-xl border border-slate-800 bg-slate-950/60 p-4'
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-2 mb-2">
        <div className="inline-flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wide text-slate-400">
          <Captions className="w-3.5 h-3.5 text-sky-400" />
          Subtitles
        </div>
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={() => void ensure()}
            disabled={ensuring || searching}
            className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-sky-500/15 text-sky-300 border border-sky-500/30 text-[11px] font-medium disabled:opacity-50"
          >
            {ensuring ? <Loader2 className="w-3 h-3 animate-spin" /> : <Sparkles className="w-3 h-3" />}
            Auto
          </button>
          <button
            type="button"
            onClick={() => void search()}
            disabled={searching || ensuring}
            className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-indigo-500/15 text-indigo-300 border border-indigo-500/30 text-[11px] font-medium disabled:opacity-50"
          >
            {searching ? <Loader2 className="w-3 h-3 animate-spin" /> : <Search className="w-3 h-3" />}
            Search
          </button>
        </div>
      </div>

      {loading ? (
        <p className="text-xs text-slate-500">Loading…</p>
      ) : (
        <>
          <div className="flex flex-wrap gap-1.5 mb-2">
            {(status?.tracks || []).length === 0 ? (
              <span className="text-xs text-slate-500">No sidecars yet</span>
            ) : (
              status?.tracks.map((t, i) => (
                <span
                  key={`${t.language}-${t.id || i}`}
                  className="text-[11px] px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30"
                  title={t.path}
                >
                  {t.language}
                  {t.hearingImpaired ? ' HI' : ''}
                  {t.provider && t.provider !== 'sidecar' ? ` · ${t.provider}` : ''}
                </span>
              ))
            )}
          </div>
          {(status?.missing || []).length > 0 && (
            <p className="text-[11px] text-amber-400/90 mb-2">
              Missing: {status?.missing.join(', ')}
            </p>
          )}
        </>
      )}

      {candidates.length > 0 && (
        <div className="mt-2 max-h-48 overflow-y-auto divide-y divide-slate-800/80 border border-slate-800 rounded-lg">
          {candidates.slice(0, 12).map((c) => (
            <div
              key={`${c.provider}-${c.fileId}`}
              className="px-3 py-2 flex items-center gap-2 hover:bg-slate-900/50"
            >
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="text-xs font-mono text-sky-300">{c.language}</span>
                  {c.hashMatch && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300">
                      hash
                    </span>
                  )}
                  {c.hearingImpaired && (
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-slate-700 text-slate-300">
                      HI
                    </span>
                  )}
                </div>
                <p className="text-[11px] text-slate-400 truncate" title={c.releaseName}>
                  {c.releaseName || 'Untitled release'}
                </p>
              </div>
              <button
                type="button"
                disabled={busyFileId === c.fileId}
                onClick={() => void download(c)}
                className="inline-flex items-center gap-1 px-2 py-1 rounded-lg bg-emerald-600/20 text-emerald-300 border border-emerald-500/30 text-[11px] font-medium disabled:opacity-50"
              >
                {busyFileId === c.fileId ? (
                  <Loader2 className="w-3 h-3 animate-spin" />
                ) : (
                  <Download className="w-3 h-3" />
                )}
                Get
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
