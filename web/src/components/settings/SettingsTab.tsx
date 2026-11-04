import React, { useCallback, useEffect, useState } from 'react';
import type {
  AppConfig,
  OverviewResponse,
  QualityProfile,
  RootFolder,
  Indexer,
} from '../../types';
import { api } from '../../api';
import { GeneralPage } from './GeneralPage';
import { UiPage } from './UiPage';
import { MediaManagementPage } from './MediaManagementPage';
import { ProfilesPage } from './ProfilesPage';
import { QualityPage } from './QualityPage';
import { ImportListsPage } from './ImportListsPage';
import { MetadataPage } from './MetadataPage';
import { SubtitlesPage } from './SubtitlesPage';
import { TagsPage } from './TagsPage';
import { errorText } from './fields';

export type SettingsPageId =
  | 'media'
  | 'profiles'
  | 'quality'
  | 'importLists'
  | 'metadata'
  | 'subtitles'
  | 'tags'
  | 'general'
  | 'ui';

export interface SettingsTabProps {
  page: SettingsPageId;
  config: AppConfig | null;
  overview: OverviewResponse | null;
  indexers: Indexer[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
  onConfigChange: (config: AppConfig) => void;
  onDirtyChange?: (dirty: boolean) => void;
}

export const SettingsTab: React.FC<SettingsTabProps> = ({
  page,
  config,
  indexers,
  profiles,
  rootFolders,
  onRefresh,
  onNotify,
  onConfigChange,
  onDirtyChange,
}) => {
  const [dirty, setDirty] = useState(false);
  const [localConfig, setLocalConfig] = useState<AppConfig | null>(config);

  const handleDirty = useCallback(
    (next: boolean) => {
      setDirty(next);
      onDirtyChange?.(next);
    },
    [onDirtyChange]
  );

  useEffect(() => {
    if (config && !dirty) setLocalConfig(config);
  }, [config, dirty]);

  useEffect(() => {
    setDirty(false);
    onDirtyChange?.(false);
    if (config) setLocalConfig(config);
    // Reset when navigating between settings pages
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [page]);

  const save = useCallback(
    async (next: AppConfig) => {
      try {
        const saved = await api.saveConfig(next);
        setLocalConfig(saved);
        onConfigChange(saved);
        setDirty(false);
        onDirtyChange?.(false);
        onNotify('Saved settings');
        return saved;
      } catch (err) {
        onNotify(errorText(err, 'Failed to save settings'), true);
        throw err;
      }
    },
    [onConfigChange, onNotify, onDirtyChange]
  );

  const handleApiKey = (apiKey: string) => {
    setLocalConfig((prev) => (prev ? { ...prev, security: { ...prev.security, apiKey } } : prev));
    if (!config) return;
    onConfigChange({ ...config, security: { ...config.security, apiKey } });
  };

  if (!localConfig) {
    return <p className="text-sm text-slate-400">Loading settings...</p>;
  }

  switch (page) {
    case 'media':
      return (
        <MediaManagementPage
          config={localConfig}
          rootFolders={rootFolders}
          onSave={save}
          onDirtyChange={handleDirty}
          onRefresh={onRefresh}
          onNotify={onNotify}
        />
      );
    case 'profiles':
      return (
        <ProfilesPage
          config={localConfig}
          profiles={profiles}
          indexers={indexers}
          onSave={save}
          onRefresh={onRefresh}
          onNotify={onNotify}
        />
      );
    case 'quality':
      return <QualityPage config={localConfig} onSave={save} onDirtyChange={handleDirty} />;
    case 'importLists':
      return (
        <ImportListsPage
          config={localConfig}
          profiles={profiles}
          rootFolders={rootFolders}
          onSave={save}
          onDirtyChange={handleDirty}
        />
      );
    case 'metadata':
      return <MetadataPage config={localConfig} onSave={save} onDirtyChange={handleDirty} />;
    case 'subtitles':
      return <SubtitlesPage config={localConfig} onSave={save} onDirtyChange={handleDirty} />;
    case 'tags':
      return <TagsPage onNotify={onNotify} />;
    case 'general':
      return (
        <GeneralPage
          config={localConfig}
          onSave={save}
          onDirtyChange={handleDirty}
          onNotify={onNotify}
          onApiKey={handleApiKey}
        />
      );
    case 'ui':
      return <UiPage config={localConfig} onSave={save} onDirtyChange={handleDirty} />;
    default:
      return null;
  }
};
