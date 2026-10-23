import React, { useState } from 'react';
import type { AppConfig, NotificationConfig } from '../../types';
import {
  Checkbox,
  dangerButtonClass,
  FieldRow,
  Modal,
  PageIntro,
  primaryButtonClass,
  quietButtonClass,
  Section,
  Select,
  TextInput,
} from './fields';

interface ConnectPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onNotify: (msg: string, isError?: boolean) => void;
}

const FLAGS: Array<{ key: keyof NotificationConfig; label: string }> = [
  { key: 'onGrab', label: 'On Grab' },
  { key: 'onImport', label: 'On Import' },
  { key: 'onUpgrade', label: 'On Upgrade' },
  { key: 'onRename', label: 'On Rename' },
  { key: 'onSeriesDelete', label: 'On Series Delete' },
  { key: 'onMovieDelete', label: 'On Movie Delete' },
  { key: 'onHealthIssue', label: 'On Health Issue' },
  { key: 'onHealthRestored', label: 'On Health Restored' },
  { key: 'onApplicationUpdate', label: 'On Application Update' },
  { key: 'includeHealthWarnings', label: 'Include Health Warnings' },
];

function blankNotification(): NotificationConfig {
  return {
    id: 0,
    name: '',
    implementation: 'Webhook',
    enable: true,
    onGrab: false,
    onImport: false,
    onUpgrade: false,
    onRename: false,
    onSeriesDelete: false,
    onMovieDelete: false,
    onHealthIssue: false,
    onHealthRestored: false,
    onApplicationUpdate: false,
    includeHealthWarnings: false,
    webhookUrl: '',
    pluginId: '',
    tags: [],
  };
}

export const ConnectPage: React.FC<ConnectPageProps> = ({ config, onSave }) => {
  const [form, setForm] = useState<{ index: number | null; value: NotificationConfig } | null>(null);
  const [busy, setBusy] = useState(false);

  const persist = async (notifications: NotificationConfig[]) => {
    setBusy(true);
    try {
      await onSave({ ...config, notifications });
      setForm(null);
    } catch {
      // Parent already reported the error.
    } finally {
      setBusy(false);
    }
  };

  const handleSave = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    const next = [...config.notifications];
    if (form.index == null) next.push(form.value);
    else next[form.index] = form.value;
    await persist(next);
  };

  const handleDelete = async (index: number) => {
    const row = config.notifications[index];
    if (!window.confirm(`Delete connection "${row?.name || 'connection'}"?`)) return;
    await persist(config.notifications.filter((_, itemIndex) => itemIndex !== index));
  };

  const setFlag = (key: keyof NotificationConfig, value: boolean) => {
    if (!form) return;
    setForm({ ...form, value: { ...form.value, [key]: value } });
  };

  return (
    <div>
      <PageIntro title="Connect" caption="Saved connections are not sent." />
      <Section title="Connections">
        <div className="flex justify-end py-3">
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() => setForm({ index: null, value: blankNotification() })}
          >
            Add Connection
          </button>
        </div>
        {config.notifications.length === 0 ? (
          <p className="text-sm text-slate-400 pb-3">No connections yet. Saved connections are not sent.</p>
        ) : (
          <div className="space-y-2 pb-3">
            {config.notifications.map((row, index) => (
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
                  <button
                    type="button"
                    className={quietButtonClass}
                    onClick={() => setForm({ index, value: row })}
                  >
                    Edit
                  </button>
                  <button type="button" className={dangerButtonClass} onClick={() => void handleDelete(index)}>
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      {form ? (
        <Modal title={form.index == null ? 'Add Connection' : 'Edit Connection'} onClose={() => setForm(null)}>
          <form onSubmit={(event) => void handleSave(event)}>
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
                  { value: 'Webhook', label: 'Webhook' },
                  { value: 'Plugin', label: 'Plugin' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Enable">
              <Checkbox checked={form.value.enable} onChange={(value) => setFlag('enable', value)} />
            </FieldRow>
            {form.value.implementation === 'Webhook' ? (
              <FieldRow label="Webhook URL">
                <TextInput
                  value={form.value.webhookUrl}
                  onChange={(value) => setForm({ ...form, value: { ...form.value, webhookUrl: value } })}
                />
              </FieldRow>
            ) : null}
            {form.value.implementation === 'Plugin' ? (
              <FieldRow label="Plugin ID">
                <TextInput
                  value={form.value.pluginId}
                  onChange={(value) => setForm({ ...form, value: { ...form.value, pluginId: value } })}
                />
              </FieldRow>
            ) : null}
            {FLAGS.map((flag) => (
              <FieldRow key={flag.key} label={flag.label}>
                <Checkbox
                  checked={Boolean(form.value[flag.key])}
                  onChange={(value) => setFlag(flag.key, value)}
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
