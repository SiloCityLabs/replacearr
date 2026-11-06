import React, { useState } from 'react';
import type { DiscoveredDownloadClient, DownloadClient } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  type ConnectionTestResult,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  PasswordInput,
  primaryButtonClass,
  Select,
  testButtonClass,
  TestResultMessage,
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

/** Next unused default: max existing priority + 1, or 1 when none exist. */
export function nextDownloadClientPriority(
  existing: Array<{ priority: number }> = [],
): number {
  let max = 0;
  for (const client of existing) {
    if (client.priority > max) max = client.priority;
  }
  return max + 1;
}

export function blankDownloadClientForm(
  existing: Array<{ priority: number }> = [],
): DownloadClientForm {
  return {
    id: null,
    name: '',
    implementation: 'qBittorrent',
    protocol: 'torrent',
    enable: true,
    priority: nextDownloadClientPriority(existing),
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

export function discoveredToForm(
  hit: DiscoveredDownloadClient,
  existing: Array<{ priority: number }> = [],
): DownloadClientForm {
  return {
    ...blankDownloadClientForm(existing),
    name: hit.nameHint || hit.implementation,
    implementation: hit.implementation,
    protocol: protocolForImplementation(hit.implementation, hit.protocol),
    baseUrl: hit.baseUrl,
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
  /** Used to warn when priority collides with another configured client. */
  existingClients?: Array<{ id: number; name: string; priority: number }>;
  onClose: () => void;
  onSaved: () => Promise<void> | void;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const DownloadClientModal: React.FC<DownloadClientModalProps> = ({
  initial,
  existingClients = [],
  onClose,
  onSaved,
  onNotify,
}) => {
  const [form, setForm] = useState<DownloadClientForm>(initial);
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);

  const priorityConflict = existingClients.find(
    (client) => client.priority === form.priority && client.id !== form.id,
  );

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
      username:
        form.implementation === 'SABnzbd' || form.implementation === 'Deluge' ? '' : form.username,
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
    setTestResult(null);
    setTesting(true);
    try {
      const result = await api.testDownloadClient({
        id: form.id,
        implementation: form.implementation,
        baseUrl: form.baseUrl,
        username: form.username,
        password: form.password,
      });
      setTestResult({
        ok: result.ok,
        message: `${result.ok ? 'OK' : 'Failed'} — ${result.message} (${result.latencyMs} ms)`,
      });
      if (!result.ok) onNotify(result.message, true);
    } catch (err) {
      const message = errorText(err, 'Download client test failed');
      setTestResult({ ok: false, message });
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
                username: value === 'SABnzbd' || value === 'Deluge' ? '' : form.username,
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
        <FieldRow
          label="Priority"
          helpText="Lower numbers are preferred. New clients default to the next free priority."
        >
          <NumberInput value={form.priority} onChange={(value) => setForm({ ...form, priority: value })} />
          {priorityConflict ? (
            <p className="text-xs text-amber-200/90 bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2 mt-2">
              Priority {form.priority} is already used by "{priorityConflict.name}". Prefer a unique value so
              clients are ordered predictably.
            </p>
          ) : null}
        </FieldRow>
        <FieldRow
          label="Base URL"
          helpText="Full URL including http/https, port, and any reverse-proxy path."
        >
          <TextInput
            value={form.baseUrl}
            onChange={(value) => {
              setTestResult(null);
              setForm({ ...form, baseUrl: value });
            }}
            placeholder="http://localhost:8080"
          />
        </FieldRow>
        {form.implementation === 'SABnzbd' || form.implementation === 'Deluge' ? null : (
          <FieldRow label="Username">
            <TextInput value={form.username} onChange={(value) => setForm({ ...form, username: value })} />
          </FieldRow>
        )}
        <FieldRow
          label={form.implementation === 'SABnzbd' ? 'API Key' : 'Password'}
          helpText={
            form.implementation === 'SABnzbd'
              ? 'SABnzbd Config → General → API Key'
              : form.implementation === 'Deluge'
                ? 'Deluge WebUI password'
                : undefined
          }
        >
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
        {testResult ? <TestResultMessage result={testResult} /> : null}
        <div className="pt-3 flex justify-end gap-2">
          <button
            type="button"
            disabled={testing}
            className={testButtonClass(testResult?.ok)}
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
