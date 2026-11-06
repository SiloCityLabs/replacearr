import React, { useState } from 'react';
import type { AppConfig, DownloadClient, DownloadClientOptions, RemotePathMapping } from '../../types';
import { api } from '../../api';
import {
  blankDownloadClientForm,
  DownloadClientForm,
  DownloadClientModal,
  downloadClientToForm,
} from '../modals/DownloadClientModal';
import {
  Checkbox,
  dangerButtonClass,
  errorText,
  FieldRow,
  PageIntro,
  primaryButtonClass,
  quietButtonClass,
  SaveBar,
  Section,
  TextInput,
  useDraftState,
} from './fields';
import { useConfirm } from '../modals/ConfirmModal';

interface DownloadClientsPageProps {
  config: AppConfig;
  clients: DownloadClient[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

interface HandlingDraft {
  downloadClientOptions: DownloadClientOptions;
  remotePathMappings: RemotePathMapping[];
}

export const DownloadClientsPage: React.FC<DownloadClientsPageProps> = ({
  config,
  clients,
  onSave,
  onDirtyChange,
  onRefresh,
  onNotify,
}) => {
  const { confirm, dialog } = useConfirm();
  const source: HandlingDraft = {
    downloadClientOptions: config.downloadClientOptions,
    remotePathMappings: config.remotePathMappings,
  };
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(source, onDirtyChange);
  const [form, setForm] = useState<DownloadClientForm | null>(null);

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

  const deleteClient = async (client: DownloadClient) => {
    const ok = await confirm({
      title: 'Delete download client',
      message: `Delete download client "${client.name}"?`,
      confirmLabel: 'Delete',
    });
    if (!ok) return;
    try {
      await api.deleteDownloadClient(client.id);
      onNotify(`Removed download client "${client.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete download client'), true);
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
            onClick={() => setForm(blankDownloadClientForm(clients))}
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
                  <p className="text-[11px] font-mono text-slate-500 truncate">{client.baseUrl}</p>
                </div>
                <div className="flex gap-2 shrink-0">
                  <button
                    type="button"
                    className={quietButtonClass}
                    onClick={() => setForm(downloadClientToForm(client))}
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
        <FieldRow
          label="Delete Torrent and Data When Complete"
          helpText="After a successful import, remove the torrent from the download client and delete its data."
        >
          <Checkbox
            checked={draft.downloadClientOptions.deleteTorrentAndDataWhenComplete}
            onChange={(value) =>
              setDraft((current) => ({
                ...current,
                downloadClientOptions: {
                  ...current.downloadClientOptions,
                  deleteTorrentAndDataWhenComplete: value,
                  deleteWhenSeedingComplete: value
                    ? current.downloadClientOptions.deleteWhenSeedingComplete
                    : false,
                },
              }))
            }
          />
        </FieldRow>
        <FieldRow
          label="Delete When Seeding Complete"
          helpText="Wait until the client reports seeding finished (stopped after ratio/idle goals), then delete torrent and data. Checking this also enables Delete Torrent and Data When Complete."
        >
          <Checkbox
            checked={draft.downloadClientOptions.deleteWhenSeedingComplete}
            onChange={(value) =>
              setDraft((current) => ({
                ...current,
                downloadClientOptions: {
                  ...current.downloadClientOptions,
                  deleteWhenSeedingComplete: value,
                  deleteTorrentAndDataWhenComplete: value
                    ? true
                    : current.downloadClientOptions.deleteTorrentAndDataWhenComplete,
                },
              }))
            }
          />
        </FieldRow>
        <FieldRow
          label="Download Client Working Folders"
          helpText="Pipe-separated folder name prefixes to skip while importing (default _UNPACK_|_FAILED_). Clients like SABnzbd create these temp folders while unpacking or after a failed job — ignoring them avoids importing incomplete downloads."
        >
          <TextInput
            value={draft.downloadClientOptions.downloadClientWorkingFolders}
            onChange={(value) => patchOptions('downloadClientWorkingFolders', value)}
          />
        </FieldRow>
      </Section>

      <Section
        title="Client Discovery"
        caption="Used by Find nearby and the setup wizard. LAN is also learned from SSDP/IGD when available."
      >
        <FieldRow
          label="Extra Subnets"
          helpText="Comma-separated CIDRs to probe in addition to auto-discovered LAN ranges (e.g. 10.5.0.0/21,192.168.1.0/24)."
        >
          <TextInput
            value={draft.downloadClientOptions.discoverSubnets ?? ''}
            placeholder="10.5.0.0/21"
            onChange={(value) => patchOptions('discoverSubnets', value)}
          />
        </FieldRow>
        <FieldRow
          label="Host /proc/net Path"
          helpText="Bind-mounted host routing table for Docker bridge mode (default /host/proc/net)."
        >
          <TextInput
            value={draft.downloadClientOptions.hostProcNet ?? ''}
            placeholder="/host/proc/net"
            onChange={(value) => patchOptions('hostProcNet', value)}
          />
        </FieldRow>
      </Section>
      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />

      {form ? (
        <DownloadClientModal
          initial={form}
          existingClients={clients}
          onClose={() => setForm(null)}
          onSaved={onRefresh}
          onNotify={onNotify}
        />
      ) : null}
      {dialog}
    </div>
  );
};
