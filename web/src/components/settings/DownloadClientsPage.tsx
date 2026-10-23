import React, { useState } from 'react';
import type { AppConfig, DownloadClient, DownloadClientOptions, RemotePathMapping } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  dangerButtonClass,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  PageIntro,
  PasswordInput,
  primaryButtonClass,
  quietButtonClass,
  SaveBar,
  Section,
  Select,
  TextInput,
  useDraftState,
} from './fields';

interface DownloadClientsPageProps {
  config: AppConfig;
  clients: DownloadClient[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

interface ClientForm {
  id: number | null;
  name: string;
  implementation: string;
  protocol: string;
  enable: boolean;
  priority: number;
  host: string;
  port: number;
  useSsl: boolean;
  username: string;
  password: string;
  category: string;
  removeCompletedDownloads: boolean;
  removeFailedDownloads: boolean;
}

interface HandlingDraft {
  downloadClientOptions: DownloadClientOptions;
  remotePathMappings: RemotePathMapping[];
}

function blankClient(): ClientForm {
  return {
    id: null,
    name: '',
    implementation: 'qBittorrent',
    protocol: 'torrent',
    enable: true,
    priority: 1,
    host: 'localhost',
    port: 8080,
    useSsl: false,
    username: '',
    password: '',
    category: '',
    removeCompletedDownloads: true,
    removeFailedDownloads: true,
  };
}

function clientToForm(client: DownloadClient): ClientForm {
  return {
    id: client.id,
    name: client.name,
    implementation: client.implementation,
    protocol: client.protocol,
    enable: client.enable,
    priority: client.priority,
    host: client.host,
    port: client.port,
    useSsl: client.useSsl,
    username: client.username,
    password: client.password ?? '',
    category: client.category,
    removeCompletedDownloads: client.removeCompletedDownloads,
    removeFailedDownloads: client.removeFailedDownloads,
  };
}

export const DownloadClientsPage: React.FC<DownloadClientsPageProps> = ({
  config,
  clients,
  onSave,
  onDirtyChange,
  onRefresh,
  onNotify,
}) => {
  const source: HandlingDraft = {
    downloadClientOptions: config.downloadClientOptions,
    remotePathMappings: config.remotePathMappings,
  };
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(source, onDirtyChange);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [form, setForm] = useState<ClientForm | null>(null);
  const [testResult, setTestResult] = useState('');
  const [busy, setBusy] = useState(false);

  const patchOptions = <K extends keyof DownloadClientOptions>(key: K, value: DownloadClientOptions[K]) => {
    setDraft((current) => ({
      ...current,
      downloadClientOptions: { ...current.downloadClientOptions, [key]: value },
    }));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const saved = await onSave({
        ...config,
        downloadClientOptions: draft.downloadClientOptions,
        remotePathMappings: draft.remotePathMappings,
      });
      commit({
        downloadClientOptions: saved.downloadClientOptions,
        remotePathMappings: saved.remotePathMappings,
      });
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const saveClient = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!form) return;
    const payload: Partial<DownloadClient> = {
      name: form.name,
      implementation: form.implementation,
      implementationName: form.implementation,
      configContract: `${form.implementation}Settings`,
      protocol: form.protocol,
      enable: form.enable,
      priority: form.priority,
      host: form.host,
      port: form.port,
      useSsl: form.useSsl,
      username: form.username,
      password: form.password,
      category: form.category,
      removeCompletedDownloads: form.removeCompletedDownloads,
      removeFailedDownloads: form.removeFailedDownloads,
    };
    setBusy(true);
    try {
      if (form.id == null) {
        const created = await api.addDownloadClient(payload);
        onNotify(`Added download client "${created.name}"`);
      } else {
        const updated = await api.updateDownloadClient(form.id, payload);
        onNotify(`Updated download client "${updated.name}"`);
      }
      setForm(null);
      setTestResult('');
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save download client'), true);
    } finally {
      setBusy(false);
    }
  };

  const deleteClient = async (client: DownloadClient) => {
    if (!window.confirm(`Delete download client "${client.name}"?`)) return;
    try {
      await api.deleteDownloadClient(client.id);
      onNotify(`Removed download client "${client.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete download client'), true);
    }
  };

  const testClient = async () => {
    if (!form) return;
    setTestResult('');
    try {
      const result = await api.testDownloadClient({
        host: form.host,
        port: form.port,
        useSsl: form.useSsl,
      });
      setTestResult(`${result.ok ? 'OK' : 'Failed'} — ${result.message} (${result.latencyMs} ms)`);
    } catch (err) {
      const message = errorText(err, 'Download client test failed');
      setTestResult(message);
      onNotify(message, true);
    }
  };

  return (
    <div>
      <PageIntro
        title="Download Clients"
        caption="Client records are live. Completed download handling and remote path mappings are stored only."
      />

      <Section title="Download Clients">
        <div className="flex justify-end py-3">
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() => {
              setTestResult('');
              setForm(blankClient());
            }}
          >
            Add Download Client
          </button>
        </div>
        {clients.length === 0 ? (
          <p className="text-xs text-slate-400 pb-3">No download clients yet.</p>
        ) : (
          <div className="space-y-2 pb-3">
            {clients.map((client) => (
              <div
                key={client.id}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-3"
              >
                <div className="min-w-0">
                  <p className="text-sm text-white truncate">{client.name}</p>
                  <p className="text-xs text-slate-400 truncate">
                    {client.implementation} · {client.protocol} · {client.enable ? 'Enabled' : 'Disabled'} · priority{' '}
                    {client.priority}
                  </p>
                  <p className="text-[11px] font-mono text-slate-500 truncate">
                    {client.useSsl ? 'https' : 'http'}://{client.host}:{client.port}
                  </p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <button
                    type="button"
                    className={quietButtonClass}
                    onClick={() => {
                      setTestResult('');
                      setForm(clientToForm(client));
                    }}
                  >
                    Edit
                  </button>
                  <button type="button" className={dangerButtonClass} onClick={() => void deleteClient(client)}>
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      <Section title="Remote Path Mappings" caption="Stored only.">
        <div className="space-y-3 py-3">
          {draft.remotePathMappings.map((mapping, index) => (
            <div key={`${mapping.id}-${index}`} className="grid grid-cols-1 md:grid-cols-[1fr_1fr_1fr_auto] gap-2">
              <TextInput
                value={mapping.host}
                placeholder="Host"
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    remotePathMappings: current.remotePathMappings.map((row, rowIndex) =>
                      rowIndex === index ? { ...row, host: value } : row
                    ),
                  }))
                }
              />
              <TextInput
                value={mapping.remotePath}
                placeholder="Remote path"
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    remotePathMappings: current.remotePathMappings.map((row, rowIndex) =>
                      rowIndex === index ? { ...row, remotePath: value } : row
                    ),
                  }))
                }
              />
              <TextInput
                value={mapping.localPath}
                placeholder="Local path"
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    remotePathMappings: current.remotePathMappings.map((row, rowIndex) =>
                      rowIndex === index ? { ...row, localPath: value } : row
                    ),
                  }))
                }
              />
              <button
                type="button"
                className={dangerButtonClass}
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    remotePathMappings: current.remotePathMappings.filter((_, rowIndex) => rowIndex !== index),
                  }))
                }
              >
                Remove
              </button>
            </div>
          ))}
          <button
            type="button"
            className={quietButtonClass}
            onClick={() =>
              setDraft((current) => ({
                ...current,
                remotePathMappings: [
                  ...current.remotePathMappings,
                  { id: 0, host: '', remotePath: '', localPath: '' },
                ],
              }))
            }
          >
            Add Remote Path
          </button>
        </div>
      </Section>

      <Section title="Completed Download Handling" caption="Stored only. Client records are live.">
        <FieldRow label="Enable Completed Download Handling">
          <Checkbox
            checked={draft.downloadClientOptions.enableCompletedDownloadHandling}
            onChange={(value) => patchOptions('enableCompletedDownloadHandling', value)}
          />
        </FieldRow>
        <FieldRow label="Auto Redownload Failed">
          <Checkbox
            checked={draft.downloadClientOptions.autoRedownloadFailed}
            onChange={(value) => patchOptions('autoRedownloadFailed', value)}
          />
        </FieldRow>
        <FieldRow label="Auto Redownload Failed from Interactive Search">
          <Checkbox
            checked={draft.downloadClientOptions.autoRedownloadFailedFromInteractiveSearch}
            onChange={(value) => patchOptions('autoRedownloadFailedFromInteractiveSearch', value)}
          />
        </FieldRow>
        {showAdvanced ? (
          <FieldRow label="Download Client Working Folders">
            <TextInput
              value={draft.downloadClientOptions.downloadClientWorkingFolders}
              onChange={(value) => patchOptions('downloadClientWorkingFolders', value)}
            />
          </FieldRow>
        ) : null}
      </Section>
      <SaveBar
        disabled={!isDirty}
        saving={saving}
        onSave={() => void handleSave()}
        showAdvanced={showAdvanced}
        onToggleAdvanced={() => setShowAdvanced((current) => !current)}
      />

      {form ? (
        <Modal title={form.id == null ? 'Add Download Client' : 'Edit Download Client'} onClose={() => setForm(null)}>
          <form onSubmit={(event) => void saveClient(event)}>
            <FieldRow label="Name">
              <TextInput value={form.name} onChange={(value) => setForm({ ...form, name: value })} />
            </FieldRow>
            <FieldRow label="Implementation">
              <Select
                value={form.implementation}
                onChange={(value) => setForm({ ...form, implementation: value })}
                options={[
                  { value: 'qBittorrent', label: 'qBittorrent' },
                  { value: 'Transmission', label: 'Transmission' },
                  { value: 'SABnzbd', label: 'SABnzbd' },
                  { value: 'NZBGet', label: 'NZBGet' },
                  { value: 'Plugin', label: 'Plugin' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Protocol">
              <Select
                value={form.protocol}
                onChange={(value) => setForm({ ...form, protocol: value })}
                options={[
                  { value: 'torrent', label: 'Torrent' },
                  { value: 'usenet', label: 'Usenet' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Enable">
              <Checkbox checked={form.enable} onChange={(value) => setForm({ ...form, enable: value })} />
            </FieldRow>
            <FieldRow label="Priority">
              <NumberInput value={form.priority} onChange={(value) => setForm({ ...form, priority: value })} />
            </FieldRow>
            <FieldRow label="Host">
              <TextInput value={form.host} onChange={(value) => setForm({ ...form, host: value })} />
            </FieldRow>
            <FieldRow label="Port">
              <NumberInput value={form.port} onChange={(value) => setForm({ ...form, port: value })} />
            </FieldRow>
            <FieldRow label="Use SSL">
              <Checkbox checked={form.useSsl} onChange={(value) => setForm({ ...form, useSsl: value })} />
            </FieldRow>
            <FieldRow label="Username">
              <TextInput value={form.username} onChange={(value) => setForm({ ...form, username: value })} />
            </FieldRow>
            <FieldRow label="Password">
              <PasswordInput value={form.password} onChange={(value) => setForm({ ...form, password: value })} />
            </FieldRow>
            <FieldRow label="Category">
              <TextInput value={form.category} onChange={(value) => setForm({ ...form, category: value })} />
            </FieldRow>
            <FieldRow label="Remove Completed Downloads">
              <Checkbox
                checked={form.removeCompletedDownloads}
                onChange={(value) => setForm({ ...form, removeCompletedDownloads: value })}
              />
            </FieldRow>
            <FieldRow label="Remove Failed Downloads">
              <Checkbox
                checked={form.removeFailedDownloads}
                onChange={(value) => setForm({ ...form, removeFailedDownloads: value })}
              />
            </FieldRow>
            {testResult ? <p className="text-xs text-slate-300 mt-3">{testResult}</p> : null}
            <div className="pt-3 flex justify-end gap-2">
              <button type="button" className={quietButtonClass} onClick={() => void testClient()}>
                Test
              </button>
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
