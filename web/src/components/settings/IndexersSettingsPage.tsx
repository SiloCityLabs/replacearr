import React, { useEffect, useState } from 'react';
import type { AppConfig, CardigannDefinition, DiscoveredPlugin, Indexer, IndexerProxy } from '../../types';
import { api } from '../../api';
import {
  blankIndexerForm,
  IndexerForm,
  IndexerModal,
  indexerToForm,
} from '../modals/IndexerModal';
import {
  blankIndexerProxyForm,
  IndexerProxyForm,
  IndexerProxyModal,
  indexerProxyToForm,
} from '../modals/IndexerProxyModal';
import {
  dangerButtonClass,
  errorText,
  FieldRow,
  NumberInput,
  PageIntro,
  primaryButtonClass,
  quietButtonClass,
  SaveBar,
  Section,
  useDraftState,
} from './fields';

interface IndexersSettingsPageProps {
  config: AppConfig;
  indexers: Indexer[];
  definitions: CardigannDefinition[];
  plugins?: DiscoveredPlugin[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const IndexersSettingsPage: React.FC<IndexersSettingsPageProps> = ({
  config,
  indexers,
  definitions,
  plugins = [],
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
  const [proxyForm, setProxyForm] = useState<IndexerProxyForm | null>(null);

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
          <button type="button" className={primaryButtonClass} onClick={() => setIndexerForm(blankIndexerForm())}>
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
            onClick={() => setProxyForm(blankIndexerProxyForm())}
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
                    onClick={() => setProxyForm(indexerProxyToForm(proxy))}
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
        <IndexerModal
          initial={indexerForm}
          definitions={definitions}
          plugins={plugins}
          onClose={() => setIndexerForm(null)}
          onSaved={onRefresh}
          onNotify={onNotify}
        />
      ) : null}

      {proxyForm ? (
        <IndexerProxyModal
          initial={proxyForm}
          onClose={() => setProxyForm(null)}
          onSaved={loadProxies}
          onNotify={onNotify}
        />
      ) : null}
    </div>
  );
};
