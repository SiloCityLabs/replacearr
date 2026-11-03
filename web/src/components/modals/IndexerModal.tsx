import React, { useState } from 'react';
import type { CardigannDefinition, DiscoveredPlugin, Indexer } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  parseNumberList,
  PasswordInput,
  primaryButtonClass,
  Select,
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
  priority: number;
  baseUrl: string;
  apiKey: string;
  definitionFile: string;
  categories: string;
  pluginId: string;
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

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const payload: Partial<Indexer> = {
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
      priority: form.priority,
      baseUrl: form.baseUrl,
      apiKey: form.apiKey,
      definitionFile: form.definitionFile,
      categories: parseNumberList(form.categories),
      pluginId: form.pluginId,
    };
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
          <Select
            value={form.definitionFile}
            onChange={(value) => setForm({ ...form, definitionFile: value })}
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
        <div className="pt-3 flex justify-end">
          <button type="submit" disabled={busy} className={primaryButtonClass}>
            Save
          </button>
        </div>
      </form>
    </Modal>
  );
};
