import React, { useState } from 'react';
import type { IndexerProxy } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  primaryButtonClass,
  quietButtonClass,
  Select,
  TextInput,
} from '../settings/fields';

export interface IndexerProxyForm {
  id: number | null;
  name: string;
  implementation: string;
  host: string;
  requestTimeout: number;
  sessionTtlMinutes: number;
  disableMedia: boolean;
  enable: boolean;
}

export function blankIndexerProxyForm(): IndexerProxyForm {
  return {
    id: null,
    name: 'FlareSolverr',
    implementation: 'FlareSolverr',
    host: 'http://localhost:8191/',
    requestTimeout: 60,
    sessionTtlMinutes: 15,
    disableMedia: false,
    enable: true,
  };
}

export function indexerProxyToForm(proxy: IndexerProxy): IndexerProxyForm {
  return {
    id: proxy.id,
    name: proxy.name,
    implementation: proxy.implementation,
    host: proxy.host,
    requestTimeout: proxy.requestTimeout,
    sessionTtlMinutes: proxy.sessionTtlMinutes,
    disableMedia: proxy.disableMedia,
    enable: proxy.enable,
  };
}

interface IndexerProxyModalProps {
  initial: IndexerProxyForm;
  onClose: () => void;
  onSaved: () => Promise<void> | void;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const IndexerProxyModal: React.FC<IndexerProxyModalProps> = ({
  initial,
  onClose,
  onSaved,
  onNotify,
}) => {
  const [form, setForm] = useState<IndexerProxyForm>(initial);
  const [testResult, setTestResult] = useState('');
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const payload: Partial<IndexerProxy> = {
      name: form.name.trim() || form.implementation,
      implementation: form.implementation,
      implementationName: form.implementation,
      configContract: `${form.implementation}Settings`,
      host: form.host,
      requestTimeout: form.requestTimeout,
      sessionTtlMinutes: form.sessionTtlMinutes,
      disableMedia: form.disableMedia,
      enable: form.enable,
    };
    setBusy(true);
    try {
      if (form.id == null) {
        const created = await api.addIndexerProxy(payload);
        onNotify(`Added indexer proxy "${created.name}"`);
      } else {
        const updated = await api.updateIndexerProxy(form.id, payload);
        onNotify(`Updated indexer proxy "${updated.name}"`);
      }
      await onSaved();
      onClose();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save indexer proxy'), true);
    } finally {
      setBusy(false);
    }
  };

  const test = async () => {
    setTestResult('');
    setTesting(true);
    try {
      const health = await api.testIndexerProxy(form.host);
      setTestResult(health.message || (health.healthy ? 'OK' : 'Unreachable'));
      if (!health.healthy) onNotify(health.message || 'Proxy unreachable', true);
    } catch (err) {
      const message = errorText(err, 'Proxy test failed');
      setTestResult(message);
      onNotify(message, true);
    } finally {
      setTesting(false);
    }
  };

  return (
    <Modal title={form.id == null ? 'Add Proxy' : 'Edit Proxy'} onClose={onClose}>
      <form onSubmit={(event) => void save(event)}>
        <FieldRow label="Name">
          <TextInput value={form.name} onChange={(value) => setForm({ ...form, name: value })} />
        </FieldRow>
        <FieldRow label="Implementation">
          <Select
            value={form.implementation}
            onChange={(value) => setForm({ ...form, implementation: value })}
            options={[
              { value: 'FlareSolverr', label: 'FlareSolverr' },
              { value: 'Http', label: 'Http' },
              { value: 'Socks4', label: 'Socks4' },
              { value: 'Socks5', label: 'Socks5' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Host" helpText="Full URL, e.g. http://localhost:8191/">
          <TextInput value={form.host} onChange={(value) => setForm({ ...form, host: value })} />
        </FieldRow>
        <FieldRow label="Request Timeout">
          <NumberInput
            value={form.requestTimeout}
            onChange={(value) => setForm({ ...form, requestTimeout: value })}
          />
        </FieldRow>
        <FieldRow label="Session TTL Minutes">
          <NumberInput
            value={form.sessionTtlMinutes}
            onChange={(value) => setForm({ ...form, sessionTtlMinutes: value })}
          />
        </FieldRow>
        <FieldRow label="Disable Media">
          <Checkbox
            checked={form.disableMedia}
            onChange={(value) => setForm({ ...form, disableMedia: value })}
          />
        </FieldRow>
        <FieldRow label="Enable">
          <Checkbox checked={form.enable} onChange={(value) => setForm({ ...form, enable: value })} />
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
