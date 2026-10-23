import React, { useEffect, useState } from 'react';
import type { AppConfig, CardigannDefinition, Indexer, IndexerProxy } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  dangerButtonClass,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  PageIntro,
  parseNumberList,
  PasswordInput,
  primaryButtonClass,
  quietButtonClass,
  SaveBar,
  Section,
  Select,
  TextInput,
  useDraftState,
} from './fields';

interface IndexersSettingsPageProps {
  config: AppConfig;
  indexers: Indexer[];
  definitions: CardigannDefinition[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

interface IndexerForm {
  id: number | null;
  name: string;
  implementation: string;
  protocol: string;
  privacy: string;
  enable: boolean;
  enableRss: boolean;
  enableAutomaticSearch: boolean;
  enableInteractiveSearch: boolean;
  priority: number;
  baseUrl: string;
  apiKey: string;
  definitionFile: string;
  categories: string;
  pluginId: string;
}

interface ProxyForm {
  id: number | null;
  name: string;
  implementation: string;
  host: string;
  requestTimeout: number;
  sessionTtlMinutes: number;
  disableMedia: boolean;
  enable: boolean;
}

function blankIndexer(): IndexerForm {
  return {
    id: null,
    name: '',
    implementation: 'Torznab',
    protocol: 'torrent',
    privacy: 'public',
    enable: true,
    enableRss: true,
    enableAutomaticSearch: true,
    enableInteractiveSearch: true,
    priority: 25,
    baseUrl: '',
    apiKey: '',
    definitionFile: '',
    categories: '',
    pluginId: '',
  };
}

function indexerToForm(indexer: Indexer): IndexerForm {
  return {
    id: indexer.id,
    name: indexer.name,
    implementation: indexer.implementation,
    protocol: indexer.protocol,
    privacy: indexer.privacy,
    enable: indexer.enable,
    enableRss: indexer.enableRss,
    enableAutomaticSearch: indexer.enableAutomaticSearch,
    enableInteractiveSearch: indexer.enableInteractiveSearch,
    priority: indexer.priority,
    baseUrl: indexer.baseUrl,
    apiKey: indexer.apiKey ?? '',
    definitionFile: indexer.definitionFile ?? '',
    categories: indexer.categories.join(', '),
    pluginId: indexer.pluginId ?? '',
  };
}

function blankProxy(): ProxyForm {
  return {
    id: null,
    name: '',
    implementation: 'FlareSolverr',
    host: 'http://localhost:8191/',
    requestTimeout: 60,
    sessionTtlMinutes: 15,
    disableMedia: false,
    enable: true,
  };
}

export const IndexersSettingsPage: React.FC<IndexersSettingsPageProps> = ({
  config,
  indexers,
  definitions,
  onSave,
  onDirtyChange,
  onRefresh,
  onNotify,
}) => {
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(
    config.indexerOptions,
    onDirtyChange
  );
  const [proxies, setProxies] = useState<IndexerProxy[]>([]);
  const [clearanceCount, setClearanceCount] = useState(0);
  const [indexerForm, setIndexerForm] = useState<IndexerForm | null>(null);
  const [proxyForm, setProxyForm] = useState<ProxyForm | null>(null);
  const [proxyTest, setProxyTest] = useState('');
  const [busy, setBusy] = useState(false);

  const loadProxies = async () => {
    const [rows, clearances] = await Promise.all([api.getIndexerProxies(), api.getClearances()]);
    setProxies(rows);
    setClearanceCount(clearances.length);
  };

  useEffect(() => {
    let active = true;
    Promise.all([api.getIndexerProxies(), api.getClearances()])
      .then(([rows, clearances]) => {
        if (!active) return;
        setProxies(rows);
        setClearanceCount(clearances.length);
      })
      .catch((err) => {
        if (active) onNotify(errorText(err, 'Failed to load indexer proxies'), true);
      });
    return () => {
      active = false;
    };
  }, [onNotify]);

  const saveOptions = async () => {
    setSaving(true);
    try {
      const saved = await onSave({ ...config, indexerOptions: draft });
      commit(saved.indexerOptions);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const saveIndexer = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!indexerForm) return;
    const payload: Partial<Indexer> = {
      name: indexerForm.name,
      implementation: indexerForm.implementation,
      implementationName: indexerForm.implementation,
      configContract: `${indexerForm.implementation}Settings`,
      protocol: indexerForm.protocol,
      privacy: indexerForm.privacy,
      enable: indexerForm.enable,
      enableRss: indexerForm.enableRss,
      enableAutomaticSearch: indexerForm.enableAutomaticSearch,
      enableInteractiveSearch: indexerForm.enableInteractiveSearch,
      priority: indexerForm.priority,
      baseUrl: indexerForm.baseUrl,
      apiKey: indexerForm.apiKey,
      definitionFile: indexerForm.definitionFile,
      categories: parseNumberList(indexerForm.categories),
      pluginId: indexerForm.pluginId,
    };
    setBusy(true);
    try {
      if (indexerForm.id == null) {
        const created = await api.addIndexer(payload);
        onNotify(`Added indexer "${created.name}"`);
      } else {
        const updated = await api.updateIndexer(indexerForm.id, payload);
        onNotify(`Updated indexer "${updated.name}"`);
      }
      setIndexerForm(null);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save indexer'), true);
    } finally {
      setBusy(false);
    }
  };

  const deleteIndexer = async (indexer: Indexer) => {
    if (!window.confirm(`Delete indexer "${indexer.name}"?`)) return;
    try {
      await api.deleteIndexer(indexer.id);
      onNotify(`Removed indexer "${indexer.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete indexer'), true);
    }
  };

  const saveProxy = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!proxyForm) return;
    const payload: Partial<IndexerProxy> = {
      name: proxyForm.name,
      implementation: proxyForm.implementation,
      implementationName: proxyForm.implementation,
      configContract: `${proxyForm.implementation}Settings`,
      host: proxyForm.host,
      requestTimeout: proxyForm.requestTimeout,
      sessionTtlMinutes: proxyForm.sessionTtlMinutes,
      disableMedia: proxyForm.disableMedia,
      enable: proxyForm.enable,
    };
    setBusy(true);
    try {
      if (proxyForm.id == null) {
        const created = await api.addIndexerProxy(payload);
        onNotify(`Added indexer proxy "${created.name}"`);
      } else {
        const updated = await api.updateIndexerProxy(proxyForm.id, payload);
        onNotify(`Updated indexer proxy "${updated.name}"`);
      }
      setProxyForm(null);
      setProxyTest('');
      await loadProxies();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save indexer proxy'), true);
    } finally {
      setBusy(false);
    }
  };

  const deleteProxy = async (proxy: IndexerProxy) => {
    if (!window.confirm(`Delete indexer proxy "${proxy.name}"?`)) return;
    try {
      await api.deleteIndexerProxy(proxy.id);
      onNotify(`Removed indexer proxy "${proxy.name}"`);
      await loadProxies();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete indexer proxy'), true);
    }
  };

  const testProxy = async () => {
    if (!proxyForm) return;
    setProxyTest('');
    try {
      const health = await api.testIndexerProxy(proxyForm.host);
      setProxyTest(health.message || (health.healthy ? 'OK' : 'Unreachable'));
    } catch (err) {
      const message = errorText(err, 'Proxy test failed');
      setProxyTest(message);
      onNotify(message, true);
    }
  };

  const clearClearances = async () => {
    try {
      await api.clearClearances();
      onNotify('Cleared FlareSolverr clearances');
      await loadProxies();
    } catch (err) {
      onNotify(errorText(err, 'Failed to clear clearances'), true);
    }
  };

  const groups = [
    { label: 'Torrent', rows: indexers.filter((indexer) => indexer.protocol === 'torrent') },
    { label: 'Usenet', rows: indexers.filter((indexer) => indexer.protocol === 'usenet') },
    {
      label: 'Other',
      rows: indexers.filter((indexer) => indexer.protocol !== 'torrent' && indexer.protocol !== 'usenet'),
    },
  ];

  return (
    <div>
      <PageIntro title="Indexers" caption="Indexer rows save from the editor. Options below are stored only." />

      <Section title="Indexers">
        <div className="flex justify-end py-3">
          <button type="button" className={primaryButtonClass} onClick={() => setIndexerForm(blankIndexer())}>
            Add Indexer
          </button>
        </div>
        {groups.map((group) =>
          group.rows.length === 0 ? null : (
            <div key={group.label} className="mb-4">
              <h4 className="text-xs font-semibold uppercase tracking-wide text-slate-500 mb-2">{group.label}</h4>
              <div className="space-y-2">
                {group.rows.map((indexer) => (
                  <div
                    key={indexer.id}
                    className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
                  >
                    <div className="min-w-0">
                      <p className="text-sm text-white truncate">{indexer.name}</p>
                      <p className="text-xs text-slate-400 truncate">
                        {indexer.implementation} · {indexer.enable ? 'Enabled' : 'Disabled'} · priority {indexer.priority}
                      </p>
                      <p className="text-[11px] font-mono text-slate-500 truncate">{indexer.baseUrl}</p>
                    </div>
                    <div className="flex gap-2 shrink-0">
                      <button type="button" className={quietButtonClass} onClick={() => setIndexerForm(indexerToForm(indexer))}>
                        Edit
                      </button>
                      <button type="button" className={dangerButtonClass} onClick={() => void deleteIndexer(indexer)}>
                        Delete
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )
        )}
      </Section>

      <Section title="Proxies">
        <div className="flex flex-wrap justify-end gap-2 py-3">
          <button type="button" className={quietButtonClass} onClick={() => void clearClearances()}>
            Clear FlareSolverr clearances ({clearanceCount})
          </button>
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() => {
              setProxyTest('');
              setProxyForm(blankProxy());
            }}
          >
            Add Proxy
          </button>
        </div>
        {proxies.length === 0 ? (
          <p className="text-xs text-slate-400 pb-3">No indexer proxies yet.</p>
        ) : (
          <div className="space-y-2 pb-3">
            {proxies.map((proxy) => (
              <div
                key={proxy.id}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-3"
              >
                <div className="min-w-0">
                  <p className="text-sm text-white truncate">{proxy.name}</p>
                  <p className="text-xs text-slate-400 truncate">
                    {proxy.implementation} · {proxy.enable ? 'Enabled' : 'Disabled'} · {proxy.host}
                  </p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <button
                    type="button"
                    className={quietButtonClass}
                    onClick={() => {
                      setProxyTest('');
                      setProxyForm({
                        id: proxy.id,
                        name: proxy.name,
                        implementation: proxy.implementation,
                        host: proxy.host,
                        requestTimeout: proxy.requestTimeout,
                        sessionTtlMinutes: proxy.sessionTtlMinutes,
                        disableMedia: proxy.disableMedia,
                        enable: proxy.enable,
                      });
                    }}
                  >
                    Edit
                  </button>
                  <button type="button" className={dangerButtonClass} onClick={() => void deleteProxy(proxy)}>
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      <Section title="Options" caption="Stored only. This save does not update indexer rows.">
        <FieldRow label="Minimum Age" helpText="Stored only.">
          <NumberInput value={draft.minimumAge} onChange={(value) => setDraft({ ...draft, minimumAge: value })} />
        </FieldRow>
        <FieldRow label="Retention" helpText="Stored only.">
          <NumberInput value={draft.retention} onChange={(value) => setDraft({ ...draft, retention: value })} />
        </FieldRow>
        <FieldRow label="Maximum Size" helpText="Stored only.">
          <NumberInput value={draft.maximumSize} onChange={(value) => setDraft({ ...draft, maximumSize: value })} />
        </FieldRow>
        <FieldRow label="RSS Sync Interval" helpText="Stored only.">
          <NumberInput
            value={draft.rssSyncInterval}
            onChange={(value) => setDraft({ ...draft, rssSyncInterval: value })}
          />
        </FieldRow>
      </Section>
      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void saveOptions()} />

      {indexerForm ? (
        <Modal title={indexerForm.id == null ? 'Add Indexer' : 'Edit Indexer'} onClose={() => setIndexerForm(null)}>
          <form onSubmit={(event) => void saveIndexer(event)}>
            <FieldRow label="Name">
              <TextInput value={indexerForm.name} onChange={(value) => setIndexerForm({ ...indexerForm, name: value })} />
            </FieldRow>
            <FieldRow label="Implementation">
              <Select
                value={indexerForm.implementation}
                onChange={(value) => setIndexerForm({ ...indexerForm, implementation: value })}
                options={[
                  { value: 'Torznab', label: 'Torznab' },
                  { value: 'Newznab', label: 'Newznab' },
                  { value: 'Cardigann', label: 'Cardigann' },
                  { value: 'Plugin', label: 'Plugin' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Protocol">
              <Select
                value={indexerForm.protocol}
                onChange={(value) => setIndexerForm({ ...indexerForm, protocol: value })}
                options={[
                  { value: 'torrent', label: 'Torrent' },
                  { value: 'usenet', label: 'Usenet' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Privacy">
              <Select
                value={indexerForm.privacy}
                onChange={(value) => setIndexerForm({ ...indexerForm, privacy: value })}
                options={[
                  { value: 'public', label: 'Public' },
                  { value: 'semiPrivate', label: 'Semi Private' },
                  { value: 'private', label: 'Private' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Enable">
              <Checkbox checked={indexerForm.enable} onChange={(value) => setIndexerForm({ ...indexerForm, enable: value })} />
            </FieldRow>
            <FieldRow label="Enable RSS">
              <Checkbox
                checked={indexerForm.enableRss}
                onChange={(value) => setIndexerForm({ ...indexerForm, enableRss: value })}
              />
            </FieldRow>
            <FieldRow label="Enable Automatic Search">
              <Checkbox
                checked={indexerForm.enableAutomaticSearch}
                onChange={(value) => setIndexerForm({ ...indexerForm, enableAutomaticSearch: value })}
              />
            </FieldRow>
            <FieldRow label="Enable Interactive Search">
              <Checkbox
                checked={indexerForm.enableInteractiveSearch}
                onChange={(value) => setIndexerForm({ ...indexerForm, enableInteractiveSearch: value })}
              />
            </FieldRow>
            <FieldRow label="Priority">
              <NumberInput
                value={indexerForm.priority}
                onChange={(value) => setIndexerForm({ ...indexerForm, priority: value })}
              />
            </FieldRow>
            <FieldRow label="Base URL">
              <TextInput
                value={indexerForm.baseUrl}
                onChange={(value) => setIndexerForm({ ...indexerForm, baseUrl: value })}
              />
            </FieldRow>
            <FieldRow label="API Key">
              <PasswordInput
                value={indexerForm.apiKey}
                onChange={(value) => setIndexerForm({ ...indexerForm, apiKey: value })}
              />
            </FieldRow>
            <FieldRow label="Definition File">
              <Select
                value={indexerForm.definitionFile}
                onChange={(value) => setIndexerForm({ ...indexerForm, definitionFile: value })}
                options={[
                  { value: '', label: 'None' },
                  ...definitions.map((definition) => ({
                    value: definition.fileName || definition.id,
                    label: definition.name,
                  })),
                ]}
              />
            </FieldRow>
            <FieldRow label="Categories">
              <TextInput
                value={indexerForm.categories}
                onChange={(value) => setIndexerForm({ ...indexerForm, categories: value })}
                placeholder="2000, 5000"
              />
            </FieldRow>
            <FieldRow label="Plugin ID">
              <TextInput
                value={indexerForm.pluginId}
                onChange={(value) => setIndexerForm({ ...indexerForm, pluginId: value })}
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

      {proxyForm ? (
        <Modal title={proxyForm.id == null ? 'Add Proxy' : 'Edit Proxy'} onClose={() => setProxyForm(null)}>
          <form onSubmit={(event) => void saveProxy(event)}>
            <FieldRow label="Name">
              <TextInput value={proxyForm.name} onChange={(value) => setProxyForm({ ...proxyForm, name: value })} />
            </FieldRow>
            <FieldRow label="Implementation">
              <Select
                value={proxyForm.implementation}
                onChange={(value) => setProxyForm({ ...proxyForm, implementation: value })}
                options={[
                  { value: 'FlareSolverr', label: 'FlareSolverr' },
                  { value: 'Http', label: 'Http' },
                  { value: 'Socks4', label: 'Socks4' },
                  { value: 'Socks5', label: 'Socks5' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Host">
              <TextInput value={proxyForm.host} onChange={(value) => setProxyForm({ ...proxyForm, host: value })} />
            </FieldRow>
            <FieldRow label="Request Timeout">
              <NumberInput
                value={proxyForm.requestTimeout}
                onChange={(value) => setProxyForm({ ...proxyForm, requestTimeout: value })}
              />
            </FieldRow>
            <FieldRow label="Session TTL Minutes">
              <NumberInput
                value={proxyForm.sessionTtlMinutes}
                onChange={(value) => setProxyForm({ ...proxyForm, sessionTtlMinutes: value })}
              />
            </FieldRow>
            <FieldRow label="Disable Media">
              <Checkbox
                checked={proxyForm.disableMedia}
                onChange={(value) => setProxyForm({ ...proxyForm, disableMedia: value })}
              />
            </FieldRow>
            <FieldRow label="Enable">
              <Checkbox checked={proxyForm.enable} onChange={(value) => setProxyForm({ ...proxyForm, enable: value })} />
            </FieldRow>
            {proxyTest ? <p className="text-xs text-slate-300 mt-3">{proxyTest}</p> : null}
            <div className="pt-3 flex justify-end gap-2">
              <button type="button" className={quietButtonClass} onClick={() => void testProxy()}>
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
