import React, { useMemo, useState } from 'react';
import type { CardigannDefinition, DiscoveredPlugin, Indexer } from '../../types';
import { api } from '../../api';
import {
  CONTENT_TYPE_OPTIONS,
  emptyDefinitionFilters,
  filterDefinitions,
  languageLabel,
  PRIVACY_TYPE_OPTIONS,
  uniqueDefinitionLanguages,
} from '../../cardigannTaxonomy';
import {
  Checkbox,
  type ConnectionTestResult,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  parseNumberList,
  PasswordInput,
  primaryButtonClass,
  Select,
  testButtonClass,
  TestResultMessage,
  TextInput,
} from '../settings/fields';

export interface IndexerForm {
  id: number | null;
  name: string;
  implementation: string;
  protocol: string;
  privacy: string;
  enable: boolean;
  enableRss: boolean;
  enableAutomaticSearch: boolean;
  enableInteractiveSearch: boolean;
  useFlareSolverr: boolean;
  priority: number;
  baseUrl: string;
  apiKey: string;
  definitionFile: string;
  categories: string;
  pluginId: string;
}

function definitionWantsFlareSolverr(definition: CardigannDefinition | undefined): boolean {
  return Boolean(definition?.settings?.some((setting) => setting.type === 'info_flaresolverr'));
}

export function blankIndexerForm(): IndexerForm {
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
    useFlareSolverr: false,
    priority: 25,
    baseUrl: '',
    apiKey: '',
    definitionFile: '',
    categories: '2000, 5000',
    pluginId: '',
  };
}

export function indexerToForm(indexer: Indexer): IndexerForm {
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
    useFlareSolverr: Boolean(indexer.useFlareSolverr),
    priority: indexer.priority,
    baseUrl: indexer.baseUrl,
    apiKey: indexer.apiKey ?? '',
    definitionFile: indexer.definitionFile ?? '',
    categories: indexer.categories.join(', '),
    pluginId: indexer.pluginId ?? '',
  };
}

interface IndexerModalProps {
  initial: IndexerForm;
  definitions: CardigannDefinition[];
  plugins?: DiscoveredPlugin[];
  onClose: () => void;
  onSaved: () => Promise<void> | void;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const IndexerModal: React.FC<IndexerModalProps> = ({
  initial,
  definitions,
  plugins = [],
  onClose,
  onSaved,
  onNotify,
}) => {
  const [form, setForm] = useState<IndexerForm>(initial);
  const [busy, setBusy] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null);
  const [defFilters, setDefFilters] = useState(emptyDefinitionFilters);

  const definitionLanguages = useMemo(
    () => uniqueDefinitionLanguages(definitions),
    [definitions]
  );

  const filteredDefinitions = useMemo(() => {
    const matched = filterDefinitions(definitions, defFilters);
    if (!form.definitionFile) return matched;
    if (matched.some((definition) => (definition.fileName || definition.id) === form.definitionFile)) {
      return matched;
    }
    const selected = definitions.find(
      (definition) => (definition.fileName || definition.id) === form.definitionFile
    );
    return selected ? [selected, ...matched] : matched;
  }, [definitions, defFilters, form.definitionFile]);

  const applyDefinition = (value: string) => {
    const def = definitions.find((d) => (d.fileName || d.id) === value);
    if (!def) {
      setForm({ ...form, definitionFile: value });
      return;
    }
    setForm({
      ...form,
      definitionFile: value,
      name: form.name.trim() || def.name,
      implementation: 'Cardigann',
      protocol: 'torrent',
      privacy:
        def.type === 'private' ? 'private' : def.type === 'semi-private' ? 'semiPrivate' : 'public',
      baseUrl: form.baseUrl.trim() || def.links?.[0] || form.baseUrl,
      useFlareSolverr: form.useFlareSolverr || definitionWantsFlareSolverr(def),
    });
  };

  const formPayload = (): Partial<Indexer> => ({
    ...(form.id != null ? { id: form.id } : {}),
    name: form.name.trim(),
    implementation: form.implementation,
    implementationName: form.implementation,
    configContract: `${form.implementation}Settings`,
    protocol: form.protocol,
    privacy: form.privacy,
    enable: form.enable,
    enableRss: form.enableRss,
    enableAutomaticSearch: form.enableAutomaticSearch,
    enableInteractiveSearch: form.enableInteractiveSearch,
    useFlareSolverr: form.useFlareSolverr,
    priority: form.priority,
    baseUrl: form.baseUrl,
    apiKey: form.apiKey,
    definitionFile: form.definitionFile,
    categories: parseNumberList(form.categories),
    pluginId: form.pluginId,
  });

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const payload = formPayload();
    setBusy(true);
    try {
      if (form.id == null) {
        const created = await api.addIndexer(payload);
        onNotify(`Added indexer "${created.name}"`);
      } else {
        const updated = await api.updateIndexer(form.id, payload);
        onNotify(`Updated indexer "${updated.name}"`);
      }
      await onSaved();
      onClose();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save indexer'), true);
    } finally {
      setBusy(false);
    }
  };

  const test = async () => {
    setTestResult(null);
    setTesting(true);
    try {
      const result = await api.testIndexer(formPayload());
      setTestResult({
        ok: result.ok,
        message: `${result.ok ? 'OK' : 'Failed'} — ${result.message} (${result.latencyMs} ms)`,
      });
      if (!result.ok) onNotify(result.message, true);
      else onNotify(result.message);
    } catch (err) {
      const message = errorText(err, 'Indexer test failed');
      setTestResult({ ok: false, message });
      onNotify(message, true);
    } finally {
      setTesting(false);
    }
  };

  return (
    <Modal title={form.id == null ? 'Add Indexer' : 'Edit Indexer'} onClose={onClose}>
      <form onSubmit={(event) => void save(event)}>
        <FieldRow label="Name">
          <TextInput value={form.name} onChange={(value) => setForm({ ...form, name: value })} />
        </FieldRow>
        <FieldRow label="Implementation">
          <Select
            value={form.implementation}
            onChange={(value) =>
              setForm({
                ...form,
                implementation: value,
                protocol: value === 'Newznab' ? 'usenet' : value === 'Torznab' ? 'torrent' : form.protocol,
              })
            }
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
            value={form.protocol}
            onChange={(value) => setForm({ ...form, protocol: value })}
            options={[
              { value: 'torrent', label: 'Torrent' },
              { value: 'usenet', label: 'Usenet' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Privacy">
          <Select
            value={form.privacy}
            onChange={(value) => setForm({ ...form, privacy: value })}
            options={[
              { value: 'public', label: 'Public' },
              { value: 'semiPrivate', label: 'Semi Private' },
              { value: 'private', label: 'Private' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Enable">
          <Checkbox checked={form.enable} onChange={(value) => setForm({ ...form, enable: value })} />
        </FieldRow>
        <FieldRow label="Enable RSS">
          <Checkbox checked={form.enableRss} onChange={(value) => setForm({ ...form, enableRss: value })} />
        </FieldRow>
        <FieldRow label="Enable Automatic Search">
          <Checkbox
            checked={form.enableAutomaticSearch}
            onChange={(value) => setForm({ ...form, enableAutomaticSearch: value })}
          />
        </FieldRow>
        <FieldRow label="Enable Interactive Search">
          <Checkbox
            checked={form.enableInteractiveSearch}
            onChange={(value) => setForm({ ...form, enableInteractiveSearch: value })}
          />
        </FieldRow>
        <FieldRow
          label="Use FlareSolverr"
          helpText="Route this indexer's HTTP through an enabled FlareSolverr proxy under Indexers → Proxies."
        >
          <Checkbox
            checked={form.useFlareSolverr}
            onChange={(value) => setForm({ ...form, useFlareSolverr: value })}
          />
        </FieldRow>
        <FieldRow label="Priority">
          <NumberInput value={form.priority} onChange={(value) => setForm({ ...form, priority: value })} />
        </FieldRow>
        <FieldRow label="Base URL">
          <TextInput value={form.baseUrl} onChange={(value) => setForm({ ...form, baseUrl: value })} />
        </FieldRow>
        <FieldRow label="API Key">
          <PasswordInput value={form.apiKey} onChange={(value) => setForm({ ...form, apiKey: value })} />
        </FieldRow>
        <FieldRow label="Definition File">
          <div className="space-y-2">
            <TextInput
              value={defFilters.query}
              onChange={(value) => setDefFilters({ ...defFilters, query: value })}
              placeholder="Filter definitions by name…"
            />
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <Select
                value={defFilters.type}
                onChange={(value) => setDefFilters({ ...defFilters, type: value })}
                options={[
                  { value: '', label: 'All privacy' },
                  ...PRIVACY_TYPE_OPTIONS.map((opt) => ({
                    value: opt.value,
                    label: opt.label,
                  })),
                ]}
              />
              <Select
                value={defFilters.language}
                onChange={(value) => setDefFilters({ ...defFilters, language: value })}
                options={[
                  { value: '', label: 'All languages' },
                  ...definitionLanguages.map((lang) => ({
                    value: lang,
                    label: languageLabel(lang),
                  })),
                ]}
              />
              <Select
                value={defFilters.contentType}
                onChange={(value) => setDefFilters({ ...defFilters, contentType: value })}
                options={[
                  { value: '', label: 'All content' },
                  ...CONTENT_TYPE_OPTIONS.map((cat) => ({ value: cat, label: cat })),
                ]}
              />
            </div>
            <Select
              value={form.definitionFile}
              onChange={applyDefinition}
              options={[
                { value: '', label: `None (${filteredDefinitions.length} shown)` },
                ...filteredDefinitions.map((definition) => ({
                  value: definition.fileName || definition.id,
                  label: `${definition.name} · ${definition.language || 'en-US'} · ${definition.type || 'public'}`,
                })),
              ]}
            />
          </div>
        </FieldRow>
        <FieldRow label="Categories">
          <TextInput
            value={form.categories}
            onChange={(value) => setForm({ ...form, categories: value })}
            placeholder="2000, 5000"
          />
        </FieldRow>
        <FieldRow label="Plugin ID">
          {plugins.length > 0 ? (
            <Select
              value={form.pluginId}
              onChange={(value) => setForm({ ...form, pluginId: value })}
              options={[
                { value: '', label: 'None' },
                ...plugins.map((plugin) => ({
                  value: plugin.manifest.id,
                  label: `${plugin.manifest.name} (${plugin.manifest.id})`,
                })),
              ]}
            />
          ) : (
            <TextInput value={form.pluginId} onChange={(value) => setForm({ ...form, pluginId: value })} />
          )}
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
