import React, { useState } from 'react';
import type { DownloadClient } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  PasswordInput,
  primaryButtonClass,
  quietButtonClass,
  Select,
  TextInput,
} from '../settings/fields';

export interface DownloadClientForm {
  id: number | null;
  name: string;
  implementation: string;
  protocol: string;
  enable: boolean;
  priority: number;
  baseUrl: string;
  username: string;
  password: string;
  category: string;
  removeCompletedDownloads: boolean;
  removeFailedDownloads: boolean;
}

export function blankDownloadClientForm(): DownloadClientForm {
  return {
    id: null,
    name: '',
    implementation: 'qBittorrent',
    protocol: 'torrent',
    enable: true,
    priority: 1,
    baseUrl: 'http://localhost:8080',
    username: '',
    password: '',
    category: 'replacearr',
    removeCompletedDownloads: true,
    removeFailedDownloads: true,
  };
}

export function downloadClientToForm(client: DownloadClient): DownloadClientForm {
  return {
    id: client.id,
    name: client.name,
    implementation: client.implementation,
    protocol: protocolForImplementation(client.implementation, client.protocol),
    enable: client.enable,
    priority: client.priority,
    baseUrl: client.baseUrl,
    username: client.username,
    password: client.password ?? '',
    category: client.category,
    removeCompletedDownloads: client.removeCompletedDownloads,
    removeFailedDownloads: client.removeFailedDownloads,
  };
}

/** Protocol is fixed by client type; only Plugin can choose. */
function protocolForImplementation(implementation: string, current = 'torrent'): string {
  switch (implementation) {
    case 'SABnzbd':
    case 'NZBGet':
      return 'usenet';
    case 'qBittorrent':
    case 'Transmission':
    case 'Deluge':
      return 'torrent';
    case 'Plugin':
      return current === 'usenet' ? 'usenet' : 'torrent';
    default:
      return 'torrent';
  }
}

interface DownloadClientModalProps {
  initial: DownloadClientForm;
  onClose: () => void;
  onSaved: () => Promise<void> | void;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const DownloadClientModal: React.FC<DownloadClientModalProps> = ({
  initial,
  onClose,
  onSaved,
  onNotify,
}) => {
  const [form, setForm] = useState<DownloadClientForm>(initial);
  const [testResult, setTestResult] = useState('');
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const payload: Partial<DownloadClient> = {
      name: form.name.trim() || form.implementation,
      implementation: form.implementation,
      implementationName: form.implementation,
      configContract: `${form.implementation}Settings`,
      protocol: protocolForImplementation(form.implementation, form.protocol),
      enable: form.enable,
      priority: form.priority,
      baseUrl: form.baseUrl,
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
      await onSaved();
      onClose();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save download client'), true);
    } finally {
      setBusy(false);
    }
  };

  const test = async () => {
    setTestResult('');
    setTesting(true);
    try {
      const result = await api.testDownloadClient({ baseUrl: form.baseUrl });
      setTestResult(`${result.ok ? 'OK' : 'Failed'} — ${result.message} (${result.latencyMs} ms)`);
      if (!result.ok) onNotify(result.message, true);
    } catch (err) {
      const message = errorText(err, 'Download client test failed');
      setTestResult(message);
      onNotify(message, true);
    } finally {
      setTesting(false);
    }
  };

  return (
    <Modal
      title={form.id == null ? 'Add Download Client' : 'Edit Download Client'}
      onClose={onClose}
    >
      <form onSubmit={(event) => void save(event)}>
        <FieldRow label="Name">
          <TextInput value={form.name} onChange={(value) => setForm({ ...form, name: value })} />
        </FieldRow>
        <FieldRow
          label="Implementation"
          helpText={`Protocol: ${protocolForImplementation(form.implementation, form.protocol)}`}
        >
          <Select
            value={form.implementation}
            onChange={(value) =>
              setForm({
                ...form,
                implementation: value,
                protocol: protocolForImplementation(value, form.protocol),
              })
            }
            options={[
              { value: 'qBittorrent', label: 'qBittorrent' },
              { value: 'Transmission', label: 'Transmission' },
              { value: 'Deluge', label: 'Deluge' },
              { value: 'SABnzbd', label: 'SABnzbd' },
              { value: 'NZBGet', label: 'NZBGet' },
              { value: 'Plugin', label: 'Plugin' },
            ]}
          />
        </FieldRow>
        {form.implementation === 'Plugin' ? (
          <FieldRow label="Protocol" helpText="Required for plugin clients only.">
            <Select
              value={form.protocol}
              onChange={(value) => setForm({ ...form, protocol: value })}
              options={[
                { value: 'torrent', label: 'Torrent' },
                { value: 'usenet', label: 'Usenet' },
              ]}
            />
          </FieldRow>
        ) : null}
        <FieldRow label="Enable">
          <Checkbox checked={form.enable} onChange={(value) => setForm({ ...form, enable: value })} />
        </FieldRow>
        <FieldRow label="Priority">
          <NumberInput value={form.priority} onChange={(value) => setForm({ ...form, priority: value })} />
        </FieldRow>
        <FieldRow
          label="Base URL"
          helpText="Full URL including http/https, port, and any reverse-proxy path."
        >
          <TextInput
            value={form.baseUrl}
            onChange={(value) => setForm({ ...form, baseUrl: value })}
            placeholder="http://localhost:8080"
          />
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
          <button
            type="button"
            disabled={testing}
            className={quietButtonClass}
            onClick={() => void test()}
          >
            {testing ? 'Testing…' : 'Test'}
          </button>
          <button type="submit" disabled={busy} className={primaryButtonClass}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  );
};
