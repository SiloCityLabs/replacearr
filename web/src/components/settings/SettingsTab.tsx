import React, { useCallback, useEffect, useState } from 'react';
import {
  Bell,
  Download,
  FolderCog,
  Gauge,
  ListPlus,
  Palette,
  Puzzle,
  Radio,
  Settings,
  SlidersHorizontal,
  Tags,
  Users,
  Clapperboard,
} from 'lucide-react';
import type {
  APIKey,
  AppConfig,
  CardigannDefinition,
  DiscoveredPlugin,
  DownloadClient,
  Indexer,
  QualityProfile,
  Role,
  RootFolder,
  User,
} from '../../types';
import { api } from '../../api';
import { PluginsTab } from '../PluginsTab';
import { UsersRbacTab } from '../UsersRbacTab';
import { GeneralPage } from './GeneralPage';
import { UiPage } from './UiPage';
import { MediaManagementPage } from './MediaManagementPage';
import { ProfilesPage } from './ProfilesPage';
import { QualityPage } from './QualityPage';
import { IndexersSettingsPage } from './IndexersSettingsPage';
import { DownloadClientsPage } from './DownloadClientsPage';
import { ImportListsPage } from './ImportListsPage';
import { ConnectPage } from './ConnectPage';
import { MetadataPage } from './MetadataPage';
import { TagsPage } from './TagsPage';
import { errorText } from './fields';

export interface SettingsTabProps {
  config: AppConfig | null;
  indexers: Indexer[];
  definitions: CardigannDefinition[];
  plugins: DiscoveredPlugin[];
  clients: DownloadClient[];
  profiles: QualityProfile[];
  rootFolders: RootFolder[];
  users: User[];
  roles: Role[];
  apiKeys: APIKey[];
  permissions: string[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
  onConfigChange: (config: AppConfig) => void;
}

const CATEGORIES = [
  {
    id: 'media',
    title: 'Media Management',
    description: 'Episode and movie naming, imports, and root folders.',
    icon: FolderCog,
  },
  {
    id: 'profiles',
    title: 'Profiles',
    description: 'Quality, delay, and release profiles.',
    icon: SlidersHorizontal,
  },
  {
    id: 'quality',
    title: 'Quality',
    description: 'Quality definitions and size limits.',
    icon: Gauge,
  },
  {
    id: 'indexers',
    title: 'Indexers',
    description: 'Indexers, proxies, and indexer options.',
    icon: Radio,
  },
  {
    id: 'downloadClients',
    title: 'Download Clients',
    description: 'Clients, remote path mappings, and completed download handling.',
    icon: Download,
  },
  {
    id: 'importLists',
    title: 'Import Lists',
    description: 'Lists and how removals are synced.',
    icon: ListPlus,
  },
  {
    id: 'connect',
    title: 'Connect',
    description: 'Notifications and webhooks.',
    icon: Bell,
  },
  {
    id: 'metadata',
    title: 'Metadata',
    description: 'Language and metadata consumers.',
    icon: Clapperboard,
  },
  {
    id: 'tags',
    title: 'Tags',
    description: 'Labels and where they are used.',
    icon: Tags,
  },
  {
    id: 'general',
    title: 'General',
    description: 'Host, security, proxy, logging, and backup.',
    icon: Settings,
  },
  {
    id: 'ui',
    title: 'UI',
    description: 'Dates, theme, and language.',
    icon: Palette,
  },
  {
    id: 'users',
    title: 'Users',
    description: 'Accounts, roles, and API keys.',
    icon: Users,
  },
  {
    id: 'plugins',
    title: 'Plugins',
    description: 'Installed Go plugins.',
    icon: Puzzle,
  },
] as const;

type SettingsView = 'home' | (typeof CATEGORIES)[number]['id'];

export const SettingsTab: React.FC<SettingsTabProps> = ({
  config,
  indexers,
  definitions,
  plugins,
  clients,
  profiles,
  rootFolders,
  users,
  roles,
  apiKeys,
  permissions,
  onRefresh,
  onNotify,
  onConfigChange,
}) => {
  const [view, setView] = useState<SettingsView>('home');
  const [dirty, setDirty] = useState(false);
  const [localConfig, setLocalConfig] = useState<AppConfig | null>(config);
  const onDirtyChange = useCallback((next: boolean) => setDirty(next), []);

  useEffect(() => {
    if (config && !dirty) setLocalConfig(config);
  }, [config, dirty]);

  const save = useCallback(
    async (next: AppConfig) => {
      try {
        const saved = await api.saveConfig(next);
        setLocalConfig(saved);
        onConfigChange(saved);
        onNotify('Saved settings');
        return saved;
      } catch (err) {
        onNotify(errorText(err, 'Failed to save settings'), true);
        throw err;
      }
    },
    [onConfigChange, onNotify]
  );

  const handleApiKey = (apiKey: string) => {
    setLocalConfig((prev) => (prev ? { ...prev, security: { ...prev.security, apiKey } } : prev));
    if (!config) return;
    onConfigChange({ ...config, security: { ...config.security, apiKey } });
  };

  const openView = (next: SettingsView) => {
    if (next === view) return;
    if (dirty && !window.confirm('Discard unsaved changes?')) return;
    setDirty(false);
    setView(next);
  };

  if (!localConfig) {
    return <p className="text-sm text-slate-400">Loading settings...</p>;
  }

  const current = CATEGORIES.find((category) => category.id === view);

  return (
    <div className="flex flex-col md:flex-row gap-4 md:gap-6">
      <nav className="flex md:flex-col gap-1 overflow-x-auto md:overflow-visible md:w-56 shrink-0">
        <button
          type="button"
          onClick={() => openView('home')}
          className={`shrink-0 md:w-full text-left px-3 py-2 rounded-lg text-xs font-medium border whitespace-nowrap ${
            view === 'home'
              ? 'bg-indigo-600/15 text-white border-indigo-500/30'
              : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60 border-transparent'
          }`}
        >
          All Settings
        </button>
        {CATEGORIES.map((category) => {
          const Icon = category.icon;
          const active = view === category.id;
          return (
            <button
              key={category.id}
              type="button"
              onClick={() => openView(category.id)}
              className={`shrink-0 md:w-full inline-flex items-center gap-2 text-left px-3 py-2 rounded-lg text-xs font-medium border whitespace-nowrap ${
                active
                  ? 'bg-indigo-600/15 text-white border-indigo-500/30'
                  : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60 border-transparent'
              }`}
            >
              <Icon className={`w-3.5 h-3.5 ${active ? 'text-indigo-300' : 'text-slate-500'}`} />
              {category.title}
            </button>
          );
        })}
      </nav>

      <div className="flex-1 min-w-0">
        {view === 'home' ? (
          <div>
            <h2 className="text-lg font-bold text-white mb-4">Settings</h2>
            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
              {CATEGORIES.map((category) => {
                const Icon = category.icon;
                return (
                  <button
                    key={category.id}
                    type="button"
                    onClick={() => openView(category.id)}
                    className="text-left bg-slate-900/90 border border-slate-800 hover:border-indigo-500/40 rounded-xl p-5 space-y-2 transition"
                  >
                    <Icon className="w-5 h-5 text-indigo-300" />
                    <div className="text-sm font-semibold text-white">{category.title}</div>
                    <p className="text-xs text-slate-400">{category.description}</p>
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}

        {view !== 'home' && current ? (
          <div key={view}>
            {view === 'media' ? (
              <MediaManagementPage
                config={localConfig}
                rootFolders={rootFolders}
                onSave={save}
                onDirtyChange={onDirtyChange}
                onRefresh={onRefresh}
                onNotify={onNotify}
              />
            ) : null}
            {view === 'profiles' ? (
              <ProfilesPage
                config={localConfig}
                profiles={profiles}
                indexers={indexers}
                onSave={save}
                onRefresh={onRefresh}
                onNotify={onNotify}
              />
            ) : null}
            {view === 'quality' ? (
              <QualityPage config={localConfig} onSave={save} onDirtyChange={onDirtyChange} />
            ) : null}
            {view === 'indexers' ? (
              <IndexersSettingsPage
                config={localConfig}
                indexers={indexers}
                definitions={definitions}
                onSave={save}
                onDirtyChange={onDirtyChange}
                onRefresh={onRefresh}
                onNotify={onNotify}
              />
            ) : null}
            {view === 'downloadClients' ? (
              <DownloadClientsPage
                config={localConfig}
                clients={clients}
                onSave={save}
                onDirtyChange={onDirtyChange}
                onRefresh={onRefresh}
                onNotify={onNotify}
              />
            ) : null}
            {view === 'importLists' ? (
              <ImportListsPage
                config={localConfig}
                profiles={profiles}
                rootFolders={rootFolders}
                onSave={save}
                onDirtyChange={onDirtyChange}
              />
            ) : null}
            {view === 'connect' ? <ConnectPage config={localConfig} onSave={save} onNotify={onNotify} /> : null}
            {view === 'metadata' ? (
              <MetadataPage config={localConfig} onSave={save} onDirtyChange={onDirtyChange} />
            ) : null}
            {view === 'tags' ? <TagsPage onNotify={onNotify} /> : null}
            {view === 'general' ? (
              <GeneralPage
                config={localConfig}
                onSave={save}
                onDirtyChange={onDirtyChange}
                onNotify={onNotify}
                onApiKey={handleApiKey}
              />
            ) : null}
            {view === 'ui' ? <UiPage config={localConfig} onSave={save} onDirtyChange={onDirtyChange} /> : null}
            {view === 'users' ? (
              <UsersRbacTab
                users={users}
                roles={roles}
                apiKeys={apiKeys}
                permissions={permissions}
                onRefresh={onRefresh}
                onNotify={onNotify}
              />
            ) : null}
            {view === 'plugins' ? (
              <PluginsTab plugins={plugins} onRefresh={onRefresh} onNotify={onNotify} />
            ) : null}
          </div>
        ) : null}
      </div>
    </div>
  );
};
