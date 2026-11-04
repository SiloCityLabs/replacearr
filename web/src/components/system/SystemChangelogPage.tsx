import React, { useEffect, useState } from 'react';
import { FileText, ChevronDown, ChevronRight, ExternalLink } from 'lucide-react';
import { api } from '../../api';
import type { ChangelogSection } from '../../types';

export interface SystemChangelogPageProps {
  onNotify: (msg: string, isError?: boolean) => void;
}

export const SystemChangelogPage: React.FC<SystemChangelogPageProps> = ({ onNotify }) => {
  const [sections, setSections] = useState<ChangelogSection[]>([]);
  const [open, setOpen] = useState<Record<string, boolean>>({});

  useEffect(() => {
    api
      .getChangelog()
      .then((data) => {
        setSections(data);
        if (data[0]) setOpen({ [data[0].version]: true });
      })
      .catch((err) => onNotify(err instanceof Error ? err.message : 'Failed to load changelog', true));
  }, [onNotify]);

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
            <FileText className="w-5 h-5 text-indigo-300" />
            Changelog
          </h2>
          <p className="text-xs text-slate-400 mt-1">What changed in each Replacearr release.</p>
        </div>
        <a
          href="https://github.com/replacearr/replacearr/releases"
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 text-xs text-indigo-300 hover:underline"
        >
          GitHub Releases
          <ExternalLink className="w-3 h-3" />
        </a>
      </div>

      <div className="space-y-2">
        {sections.map((s) => {
          const isOpen = Boolean(open[s.version]);
          return (
            <div key={s.version} className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
              <button
                type="button"
                onClick={() => setOpen((prev) => ({ ...prev, [s.version]: !isOpen }))}
                className="w-full flex items-center justify-between px-4 py-3 text-left hover:bg-slate-800/40"
              >
                <div>
                  <span className="text-sm font-semibold text-white">{s.version}</span>
                  {s.date ? <span className="ml-2 text-xs text-slate-500">{s.date}</span> : null}
                </div>
                {isOpen ? (
                  <ChevronDown className="w-4 h-4 text-slate-500" />
                ) : (
                  <ChevronRight className="w-4 h-4 text-slate-500" />
                )}
              </button>
              {isOpen ? (
                <ul className="px-4 pb-4 space-y-1.5 border-t border-slate-800 pt-3">
                  {s.notes.map((n, i) => (
                    <li key={i} className="text-xs text-slate-300 flex gap-2">
                      <span className="text-indigo-400 shrink-0">•</span>
                      <span>{n}</span>
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
          );
        })}
        {sections.length === 0 ? (
          <p className="text-sm text-slate-400">No changelog entries.</p>
        ) : null}
      </div>
    </div>
  );
};
