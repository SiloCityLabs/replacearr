import React, { useState } from 'react';
import type { AppConfig, MetadataConsumer } from '../../types';
import {
  Checkbox,
  dangerButtonClass,
  FieldRow,
  Modal,
  PageIntro,
  primaryButtonClass,
  quietButtonClass,
  SaveBar,
  Section,
  Select,
  StoredOnlyNote,
  TextInput,
  useDraftState,
} from './fields';

interface MetadataPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
}

function blankConsumer(): MetadataConsumer {
  return {
    id: 0,
    name: '',
    implementation: 'Kodi',
    enable: true,
    seriesImages: false,
    seasonImages: false,
    episodeImages: false,
    movieImages: false,
    writeNfo: false,
  };
}

export const MetadataPage: React.FC<MetadataPageProps> = ({ config, onSave, onDirtyChange }) => {
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(config.metadata, onDirtyChange);
  const [form, setForm] = useState<{ index: number | null; value: MetadataConsumer } | null>(null);
  const [busy, setBusy] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      const saved = await onSave({ ...config, metadata: draft });
      commit(saved.metadata);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const saveConsumer = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    const next = [...config.metadataConsumers];
    if (form.index == null) next.push(form.value);
    else next[form.index] = form.value;
    setBusy(true);
    try {
      await onSave({ ...config, metadataConsumers: next });
      setForm(null);
    } catch {
      // Parent already reported the error.
    } finally {
      setBusy(false);
    }
  };

  const deleteConsumer = async (index: number) => {
    const row = config.metadataConsumers[index];
    if (!window.confirm(`Delete metadata consumer "${row?.name || 'consumer'}"?`)) return;
    try {
      await onSave({
        ...config,
        metadataConsumers: config.metadataConsumers.filter((_, itemIndex) => itemIndex !== index),
      });
    } catch {
      // Parent already reported the error.
    }
  };

  return (
    <div>
      <PageIntro title="Metadata" caption="Preferred language and metadata consumers." />
      <StoredOnlyNote />
      <Section title="Options">
        <FieldRow label="Preferred Language">
          <Select
            value={draft.preferredLanguage}
            onChange={(value) => setDraft({ ...draft, preferredLanguage: value })}
            options={[
              { value: 'english', label: 'English' },
              { value: 'french', label: 'French' },
              { value: 'german', label: 'German' },
              { value: 'spanish', label: 'Spanish' },
              { value: 'japanese', label: 'Japanese' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Cleanup Metadata Images">
          <Checkbox
            checked={draft.cleanupMetadataImages}
            onChange={(value) => setDraft({ ...draft, cleanupMetadataImages: value })}
          />
        </FieldRow>
      </Section>
      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />

      <Section title="Metadata Consumers">
        <div className="flex justify-end py-3">
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() => setForm({ index: null, value: blankConsumer() })}
          >
            Add Consumer
          </button>
        </div>
        {config.metadataConsumers.length === 0 ? (
          <p className="text-xs text-slate-400 pb-3">No metadata consumers yet.</p>
        ) : (
          <div className="space-y-2 pb-3">
            {config.metadataConsumers.map((row, index) => (
              <div
                key={`${row.id}-${index}`}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-3"
              >
                <div>
                  <p className="text-sm text-white">{row.name || 'Untitled'}</p>
                  <p className="text-xs text-slate-400">
                    {row.implementation} · {row.enable ? 'Enabled' : 'Disabled'}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button type="button" className={quietButtonClass} onClick={() => setForm({ index, value: row })}>
                    Edit
                  </button>
                  <button type="button" className={dangerButtonClass} onClick={() => void deleteConsumer(index)}>
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      {form ? (
        <Modal title={form.index == null ? 'Add Consumer' : 'Edit Consumer'} onClose={() => setForm(null)}>
          <form onSubmit={(event) => void saveConsumer(event)}>
            <FieldRow label="Name">
              <TextInput
                value={form.value.name}
                onChange={(value) => setForm({ ...form, value: { ...form.value, name: value } })}
              />
            </FieldRow>
            <FieldRow label="Implementation">
              <Select
                value={form.value.implementation}
                onChange={(value) => setForm({ ...form, value: { ...form.value, implementation: value } })}
                options={[
                  { value: 'Kodi', label: 'Kodi' },
                  { value: 'Emby', label: 'Emby' },
                  { value: 'Roksbox', label: 'Roksbox' },
                  { value: 'WDTV', label: 'WDTV' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Enable">
              <Checkbox
                checked={form.value.enable}
                onChange={(value) => setForm({ ...form, value: { ...form.value, enable: value } })}
              />
            </FieldRow>
            {(
              [
                ['seriesImages', 'Series Images'],
                ['seasonImages', 'Season Images'],
                ['episodeImages', 'Episode Images'],
                ['movieImages', 'Movie Images'],
                ['writeNfo', 'Write NFO'],
              ] as const
            ).map(([key, label]) => (
              <FieldRow key={key} label={label}>
                <Checkbox
                  checked={form.value[key]}
                  onChange={(value) => setForm({ ...form, value: { ...form.value, [key]: value } })}
                />
              </FieldRow>
            ))}
            <div className="pt-3 flex justify-end">
              <button type="submit" disabled={busy} className={primaryButtonClass}>
                Save
              </button>
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  );
};
