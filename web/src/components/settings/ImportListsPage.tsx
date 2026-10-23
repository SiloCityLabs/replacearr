import React, { useState } from 'react';
import type { AppConfig, ImportListConfig, QualityProfile, RootFolder } from '../../types';
import {
  Checkbox,
  dangerButtonClass,
  FieldRow,
  Modal,
  NumberInput,
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

interface ImportListsPageProps {
  config: AppConfig;
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
}

function blankList(config: AppConfig): ImportListConfig {
  return {
    id: 0,
    name: '',
    implementation: '',
    enable: true,
    qualityProfileId: config.importLists[0]?.qualityProfileId ?? 0,
    rootFolderPath: '',
    monitor: 'all',
    searchOnAdd: false,
    url: '',
    tags: [],
  };
}

export const ImportListsPage: React.FC<ImportListsPageProps> = ({
  config,
  profiles,
  rootFolders,
  onSave,
  onDirtyChange,
}) => {
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(
    config.importListOptions,
    onDirtyChange
  );
  const [form, setForm] = useState<{ index: number | null; value: ImportListConfig } | null>(null);
  const [busy, setBusy] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      const saved = await onSave({ ...config, importListOptions: draft });
      commit(saved.importListOptions);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const saveList = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    const next = [...config.importLists];
    if (form.index == null) next.push(form.value);
    else next[form.index] = form.value;
    setBusy(true);
    try {
      await onSave({ ...config, importLists: next });
      setForm(null);
    } catch {
      // Parent already reported the error.
    } finally {
      setBusy(false);
    }
  };

  const deleteList = async (index: number) => {
    const row = config.importLists[index];
    if (!window.confirm(`Delete import list "${row?.name || 'list'}"?`)) return;
    try {
      await onSave({
        ...config,
        importLists: config.importLists.filter((_, itemIndex) => itemIndex !== index),
      });
    } catch {
      // Parent already reported the error.
    }
  };

  return (
    <div>
      <PageIntro title="Import Lists" caption="List sync options are stored only." />
      <StoredOnlyNote />
      <Section title="List Options">
        <FieldRow label="List Sync Level">
          <Select
            value={draft.listSyncLevel}
            onChange={(value) => setDraft({ ...draft, listSyncLevel: value })}
            options={[
              { value: 'disabled', label: 'Disabled' },
              { value: 'logOnly', label: 'Log Only' },
              { value: 'keepAndUnmonitor', label: 'Keep and Unmonitor' },
              { value: 'keepAndTag', label: 'Keep and Tag' },
            ]}
          />
        </FieldRow>
        <FieldRow label="List Sync Tag">
          <NumberInput
            value={draft.listSyncTag}
            onChange={(value) => setDraft({ ...draft, listSyncTag: value })}
          />
        </FieldRow>
      </Section>
      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />

      <Section title="Import Lists">
        <div className="flex justify-end py-3">
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() => setForm({ index: null, value: blankList(config) })}
          >
            Add Import List
          </button>
        </div>
        {config.importLists.length === 0 ? (
          <p className="text-xs text-slate-400 pb-3">No import lists yet.</p>
        ) : (
          <div className="space-y-2 pb-3">
            {config.importLists.map((row, index) => (
              <div
                key={`${row.id}-${index}`}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-3"
              >
                <div className="min-w-0">
                  <p className="text-sm text-white truncate">{row.name || 'Untitled'}</p>
                  <p className="text-xs text-slate-400 truncate">
                    {row.implementation || 'Custom'} · {row.enable ? 'Enabled' : 'Disabled'}
                  </p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <button type="button" className={quietButtonClass} onClick={() => setForm({ index, value: row })}>
                    Edit
                  </button>
                  <button type="button" className={dangerButtonClass} onClick={() => void deleteList(index)}>
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      {form ? (
        <Modal title={form.index == null ? 'Add Import List' : 'Edit Import List'} onClose={() => setForm(null)}>
          <form onSubmit={(event) => void saveList(event)}>
            <FieldRow label="Name">
              <TextInput
                value={form.value.name}
                onChange={(value) => setForm({ ...form, value: { ...form.value, name: value } })}
              />
            </FieldRow>
            <FieldRow label="Implementation">
              <TextInput
                value={form.value.implementation}
                onChange={(value) => setForm({ ...form, value: { ...form.value, implementation: value } })}
              />
            </FieldRow>
            <FieldRow label="Enable">
              <Checkbox
                checked={form.value.enable}
                onChange={(value) => setForm({ ...form, value: { ...form.value, enable: value } })}
              />
            </FieldRow>
            <FieldRow label="Quality Profile">
              <Select
                value={form.value.qualityProfileId}
                onChange={(value) =>
                  setForm({ ...form, value: { ...form.value, qualityProfileId: Number(value) } })
                }
                options={profiles.map((profile) => ({ value: profile.id, label: profile.name }))}
              />
            </FieldRow>
            <FieldRow label="Root Folder">
              <Select
                value={form.value.rootFolderPath}
                onChange={(value) => setForm({ ...form, value: { ...form.value, rootFolderPath: value } })}
                options={[
                  { value: '', label: 'Select a folder' },
                  ...rootFolders.map((folder) => ({ value: folder.path, label: folder.path })),
                ]}
              />
            </FieldRow>
            <FieldRow label="Monitor">
              <TextInput
                value={form.value.monitor}
                onChange={(value) => setForm({ ...form, value: { ...form.value, monitor: value } })}
              />
            </FieldRow>
            <FieldRow label="Search on Add">
              <Checkbox
                checked={form.value.searchOnAdd}
                onChange={(value) => setForm({ ...form, value: { ...form.value, searchOnAdd: value } })}
              />
            </FieldRow>
            <FieldRow label="URL">
              <TextInput
                value={form.value.url}
                onChange={(value) => setForm({ ...form, value: { ...form.value, url: value } })}
              />
            </FieldRow>
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
