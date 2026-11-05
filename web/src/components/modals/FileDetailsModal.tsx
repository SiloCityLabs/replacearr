import React, { useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import type { FileDetails } from '../../types';
import { Modal, quietButtonClass } from '../settings/fields';

interface FileDetailsModalProps {
  title?: string;
  load: () => Promise<FileDetails>;
  onClose: () => void;
  onNotify?: (msg: string, isError?: boolean) => void;
}

export const FileDetailsModal: React.FC<FileDetailsModalProps> = ({
  title = 'File details',
  load,
  onClose,
  onNotify,
}) => {
  const [details, setDetails] = useState<FileDetails | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    void load()
      .then((payload) => {
        if (!cancelled) setDetails(payload);
      })
      .catch((err) => {
        onNotify?.(err instanceof Error ? err.message : 'Failed to load file details', true);
        if (!cancelled) onClose();
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // Load once when the modal opens; parent remounts via key when the target changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Modal title={title} onClose={onClose}>
      {loading || !details ? (
        <div className="flex items-center gap-2 text-sm text-slate-400 py-6">
          <Loader2 className="w-4 h-4 animate-spin" />
          Probing file…
        </div>
      ) : (
        <div>
          {details.source && (
            <p className="text-[11px] uppercase tracking-wide text-slate-500 mb-3">
              Source: {details.source}
            </p>
          )}
          <dl className="divide-y divide-slate-800/80 border border-slate-800 rounded-lg overflow-hidden">
            {(details.properties || []).map((prop) => (
              <div
                key={`${prop.label}-${prop.value}`}
                className="grid grid-cols-1 sm:grid-cols-[11rem_1fr] gap-1 sm:gap-4 px-3 py-2.5 bg-slate-950/40"
              >
                <dt className="text-xs font-medium text-slate-400">{prop.label}</dt>
                <dd className="text-xs text-slate-200 font-mono break-all">{prop.value || '—'}</dd>
              </div>
            ))}
          </dl>
          {(details.properties || []).length === 0 && (
            <p className="text-sm text-slate-500 py-4">No details available for this file.</p>
          )}
          <div className="mt-5 flex justify-end">
            <button type="button" className={quietButtonClass} onClick={onClose}>
              Close
            </button>
          </div>
        </div>
      )}
    </Modal>
  );
};
