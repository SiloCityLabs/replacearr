import React from 'react';
import type { AppConfig, QualityDefinition } from '../../types';
import { NumberInput, PageIntro, quietButtonClass, SaveBar, TextInput, useDraftState } from './fields';

interface QualityPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
}

export const QualityPage: React.FC<QualityPageProps> = ({ config, onSave, onDirtyChange }) => {
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(
    config.qualityDefinitions,
    onDirtyChange
  );

  const update = (id: number, patch: Partial<QualityDefinition>) => {
    setDraft((current) => current.map((row) => (row.id === id ? { ...row, ...patch } : row)));
  };

  const reset = () => {
    setDraft((current) =>
      current.map((row) => ({
        ...row,
        title: row.name,
        minSize: 0,
        preferredSize: 0,
        maxSize: 0,
      }))
    );
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const saved = await onSave({ ...config, qualityDefinitions: draft });
      commit(saved.qualityDefinitions);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  return (
    <div>
      <PageIntro title="Quality" caption="Sizes are MB per minute. 0 means unlimited. Stored only until the decision engine reads them." />
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 overflow-x-auto">
        <table className="w-full text-sm min-w-[720px]">
          <thead>
            <tr className="text-left text-xs text-slate-400">
              <th className="py-2 pr-3 font-medium">Name</th>
              <th className="py-2 pr-3 font-medium">Title</th>
              <th className="py-2 pr-3 font-medium">Resolution</th>
              <th className="py-2 pr-3 font-medium">Min</th>
              <th className="py-2 pr-3 font-medium">Preferred</th>
              <th className="py-2 font-medium">Max</th>
            </tr>
          </thead>
          <tbody>
            {draft.map((row) => (
              <tr key={row.id} className="border-t border-slate-800/80">
                <td className="py-2 pr-3 text-slate-300">{row.name}</td>
                <td className="py-2 pr-3">
                  <TextInput value={row.title} onChange={(value) => update(row.id, { title: value })} />
                </td>
                <td className="py-2 pr-3 text-slate-400">{row.resolution}</td>
                <td className="py-2 pr-3">
                  <NumberInput value={row.minSize} onChange={(value) => update(row.id, { minSize: value })} />
                </td>
                <td className="py-2 pr-3">
                  <NumberInput
                    value={row.preferredSize}
                    onChange={(value) => update(row.id, { preferredSize: value })}
                  />
                </td>
                <td className="py-2">
                  <NumberInput value={row.maxSize} onChange={(value) => update(row.id, { maxSize: value })} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        <div className="mt-4">
          <button type="button" className={quietButtonClass} onClick={reset}>
            Reset Definitions
          </button>
        </div>
      </div>
      <p className="mt-3 text-xs text-slate-500">
        Sizes are MB per minute. 0 means unlimited. Stored only until the decision engine reads them.
      </p>
      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />
    </div>
  );
};
