import React, { useState } from 'react';
import type { AppConfig, MediaManagementConfig, RootFolder } from '../../types';
import { api, formatBytes } from '../../api';
import {
  Checkbox,
  errorText,
  FieldRow,
  NumberInput,
  PageIntro,
  primaryButtonClass,
  dangerButtonClass,
  SaveBar,
  Section,
  Select,
  StoredOnlyNote,
  TextInput,
  useDraftState,
} from './fields';

interface MediaManagementPageProps {
  config: AppConfig;
  rootFolders: RootFolder[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

export const MediaManagementPage: React.FC<MediaManagementPageProps> = ({
  config,
  rootFolders,
  onSave,
  onDirtyChange,
  onRefresh,
  onNotify,
}) => {
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(
    config.mediaManagement,
    onDirtyChange
  );
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [folderPath, setFolderPath] = useState('');
  const [folderType, setFolderType] = useState('all');
  const [folderBusy, setFolderBusy] = useState(false);

  const patch = <K extends keyof MediaManagementConfig>(key: K, value: MediaManagementConfig[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const saved = await onSave({ ...config, mediaManagement: draft });
      commit(saved.mediaManagement);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const handleAddFolder = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!folderPath.trim()) return;
    setFolderBusy(true);
    try {
      const created = await api.addRootFolder({ path: folderPath.trim(), mediaType: folderType });
      setFolderPath('');
      onNotify(`Added root folder "${created.path}"`);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to add root folder'), true);
    } finally {
      setFolderBusy(false);
    }
  };

  const handleDeleteFolder = async (folder: RootFolder) => {
    if (!window.confirm(`Delete root folder "${folder.path}"?`)) return;
    try {
      await api.deleteRootFolder(folder.id);
      onNotify(`Removed root folder "${folder.path}"`);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete root folder'), true);
    }
  };

  return (
    <div>
      <PageIntro title="Media Management" caption="Naming, folders, importing, and root folders." />
      <StoredOnlyNote />

      <Section title="Episode Naming">
        <FieldRow label="Rename Episodes">
          <Checkbox checked={draft.renameEpisodes} onChange={(value) => patch('renameEpisodes', value)} />
        </FieldRow>
        <FieldRow label="Replace Illegal Characters">
          <Checkbox
            checked={draft.replaceIllegalCharacters}
            onChange={(value) => patch('replaceIllegalCharacters', value)}
          />
        </FieldRow>
        <FieldRow label="Colon Replacement">
          <Select
            value={draft.colonReplacementFormat}
            onChange={(value) => patch('colonReplacementFormat', value)}
            options={[
              { value: 'delete', label: 'Delete' },
              { value: 'dash', label: 'Dash' },
              { value: 'space', label: 'Space' },
              { value: 'spaceDash', label: 'Space Dash' },
              { value: 'spaceDashSpace', label: 'Space Dash Space' },
              { value: 'smart', label: 'Smart' },
              { value: 'custom', label: 'Custom' },
            ]}
          />
        </FieldRow>
        {draft.colonReplacementFormat === 'custom' ? (
          <FieldRow label="Custom Colon Replacement">
            <TextInput
              value={draft.customColonReplacementFormat}
              onChange={(value) => patch('customColonReplacementFormat', value)}
            />
          </FieldRow>
        ) : null}
        <FieldRow label="Multi Episode Style">
          <Select
            value={draft.multiEpisodeStyle}
            onChange={(value) => patch('multiEpisodeStyle', value)}
            options={[
              { value: 'extend', label: 'Extend' },
              { value: 'duplicate', label: 'Duplicate' },
              { value: 'repeat', label: 'Repeat' },
              { value: 'scene', label: 'Scene' },
              { value: 'range', label: 'Range' },
              { value: 'prefixedRange', label: 'Prefixed Range' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Standard Episode Format">
          <TextInput
            value={draft.standardEpisodeFormat}
            onChange={(value) => patch('standardEpisodeFormat', value)}
          />
        </FieldRow>
        <FieldRow label="Daily Episode Format">
          <TextInput value={draft.dailyEpisodeFormat} onChange={(value) => patch('dailyEpisodeFormat', value)} />
        </FieldRow>
        <FieldRow label="Anime Episode Format">
          <TextInput value={draft.animeEpisodeFormat} onChange={(value) => patch('animeEpisodeFormat', value)} />
        </FieldRow>
        <FieldRow label="Episode Title Required">
          <Select
            value={draft.episodeTitleRequired}
            onChange={(value) => patch('episodeTitleRequired', value)}
            options={[
              { value: 'always', label: 'Always' },
              { value: 'bulkSeasonReleases', label: 'Bulk Season Releases' },
              { value: 'never', label: 'Never' },
            ]}
          />
        </FieldRow>
      </Section>

      <Section title="Movie Naming">
        <FieldRow label="Rename Movies">
          <Checkbox checked={draft.renameMovies} onChange={(value) => patch('renameMovies', value)} />
        </FieldRow>
        <FieldRow label="Movie Folder Format">
          <TextInput value={draft.movieFolderFormat} onChange={(value) => patch('movieFolderFormat', value)} />
        </FieldRow>
        <FieldRow label="Movie File Format">
          <TextInput value={draft.movieFileFormat} onChange={(value) => patch('movieFileFormat', value)} />
        </FieldRow>
        <FieldRow label="Auto Rename Folders">
          <Checkbox checked={draft.autoRenameFolders} onChange={(value) => patch('autoRenameFolders', value)} />
        </FieldRow>
      </Section>

      {showAdvanced ? (
        <>
          <Section title="Folders">
            <FieldRow label="Series Folder Format">
              <TextInput
                value={draft.seriesFolderFormat}
                onChange={(value) => patch('seriesFolderFormat', value)}
              />
            </FieldRow>
            <FieldRow label="Season Folder Format">
              <TextInput
                value={draft.seasonFolderFormat}
                onChange={(value) => patch('seasonFolderFormat', value)}
              />
            </FieldRow>
            <FieldRow label="Specials Folder Format">
              <TextInput
                value={draft.specialsFolderFormat}
                onChange={(value) => patch('specialsFolderFormat', value)}
              />
            </FieldRow>
            <FieldRow label="Create Empty Series Folders">
              <Checkbox
                checked={draft.createEmptySeriesFolders}
                onChange={(value) => patch('createEmptySeriesFolders', value)}
              />
            </FieldRow>
            <FieldRow label="Create Empty Movie Folders">
              <Checkbox
                checked={draft.createEmptyMovieFolders}
                onChange={(value) => patch('createEmptyMovieFolders', value)}
              />
            </FieldRow>
            <FieldRow label="Delete Empty Folders">
              <Checkbox
                checked={draft.deleteEmptyFolders}
                onChange={(value) => patch('deleteEmptyFolders', value)}
              />
            </FieldRow>
          </Section>

          <Section title="Importing">
            <FieldRow label="Skip Free Space Check">
              <Checkbox
                checked={draft.skipFreeSpaceCheckWhenImporting}
                onChange={(value) => patch('skipFreeSpaceCheckWhenImporting', value)}
              />
            </FieldRow>
            <FieldRow label="Minimum Free Space">
              <NumberInput
                value={draft.minimumFreeSpaceWhenImporting}
                onChange={(value) => patch('minimumFreeSpaceWhenImporting', value)}
              />
            </FieldRow>
            <FieldRow label="Copy Using Hardlinks">
              <Checkbox
                checked={draft.copyUsingHardlinks}
                onChange={(value) => patch('copyUsingHardlinks', value)}
              />
            </FieldRow>
            <FieldRow label="Use Script Import">
              <Checkbox checked={draft.useScriptImport} onChange={(value) => patch('useScriptImport', value)} />
            </FieldRow>
            <FieldRow label="Script Import Path">
              <TextInput value={draft.scriptImportPath} onChange={(value) => patch('scriptImportPath', value)} />
            </FieldRow>
            <FieldRow label="Import Extra Files">
              <Checkbox
                checked={draft.importExtraFiles}
                onChange={(value) => patch('importExtraFiles', value)}
              />
            </FieldRow>
            <FieldRow label="Extra File Extensions">
              <TextInput
                value={draft.extraFileExtensions}
                onChange={(value) => patch('extraFileExtensions', value)}
              />
            </FieldRow>
          </Section>
        </>
      ) : null}

      <Section title="File Management">
        <FieldRow label="Unmonitor Deleted Episodes">
          <Checkbox
            checked={draft.unmonitorDeletedEpisodes}
            onChange={(value) => patch('unmonitorDeletedEpisodes', value)}
          />
        </FieldRow>
        <FieldRow label="Unmonitor Deleted Movies">
          <Checkbox
            checked={draft.unmonitorDeletedMovies}
            onChange={(value) => patch('unmonitorDeletedMovies', value)}
          />
        </FieldRow>
        <FieldRow label="Propers and Repacks">
          <Select
            value={draft.downloadPropersAndRepacks}
            onChange={(value) => patch('downloadPropersAndRepacks', value)}
            options={[
              { value: 'preferAndUpgrade', label: 'Prefer and Upgrade' },
              { value: 'doNotUpgrade', label: 'Do Not Upgrade' },
              { value: 'doNotPrefer', label: 'Do Not Prefer' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Analyze Video Files">
          <Checkbox checked={draft.analyzeVideoFiles} onChange={(value) => patch('analyzeVideoFiles', value)} />
        </FieldRow>
        <FieldRow label="Rescan After Refresh">
          <Select
            value={draft.rescanAfterRefresh}
            onChange={(value) => patch('rescanAfterRefresh', value)}
            options={[
              { value: 'always', label: 'Always' },
              { value: 'afterManual', label: 'After Manual' },
              { value: 'never', label: 'Never' },
            ]}
          />
        </FieldRow>
        <FieldRow label="File Date">
          <Select
            value={draft.fileDate}
            onChange={(value) => patch('fileDate', value)}
            options={[
              { value: 'none', label: 'None' },
              { value: 'localAirDate', label: 'Local Air Date' },
              { value: 'utcAirDate', label: 'UTC Air Date' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Recycle Bin">
          <TextInput value={draft.recycleBin} onChange={(value) => patch('recycleBin', value)} />
        </FieldRow>
        <FieldRow label="Recycle Bin Cleanup Days">
          <NumberInput
            value={draft.recycleBinCleanupDays}
            onChange={(value) => patch('recycleBinCleanupDays', value)}
          />
        </FieldRow>
      </Section>

      {showAdvanced ? (
        <Section title="Permissions">
          <FieldRow label="Set Permissions">
            <Checkbox
              checked={draft.setPermissionsLinux}
              onChange={(value) => patch('setPermissionsLinux', value)}
            />
          </FieldRow>
          <FieldRow label="chmod Folder">
            <TextInput value={draft.chmodFolder} onChange={(value) => patch('chmodFolder', value)} />
          </FieldRow>
          <FieldRow label="chown Group">
            <TextInput value={draft.chownGroup} onChange={(value) => patch('chownGroup', value)} />
          </FieldRow>
        </Section>
      ) : null}

      <SaveBar
        disabled={!isDirty}
        saving={saving}
        onSave={() => void handleSave()}
        showAdvanced={showAdvanced}
        onToggleAdvanced={() => setShowAdvanced((current) => !current)}
      />

      <section className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 mt-4 space-y-4">
        <div>
          <h3 className="text-sm font-semibold text-white">Root Folders</h3>
          <p className="text-xs text-slate-500 mt-1">Library paths for series and movies.</p>
        </div>
        {rootFolders.length === 0 ? (
          <p className="text-xs text-slate-400">No root folders yet.</p>
        ) : (
          <div className="space-y-2">
            {rootFolders.map((folder) => (
              <div
                key={folder.id}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
              >
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <code className="text-xs font-mono text-slate-200 break-all">{folder.path}</code>
                    <span className="text-[10px] font-mono uppercase px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300">
                      {folder.mediaType}
                    </span>
                    <span
                      className={`text-[10px] font-mono uppercase px-1.5 py-0.5 rounded ${
                        folder.accessible
                          ? 'bg-emerald-500/10 text-emerald-300'
                          : 'bg-rose-500/10 text-rose-300'
                      }`}
                    >
                      {folder.accessible ? 'accessible' : 'not accessible'}
                    </span>
                  </div>
                  <p className="text-[11px] text-slate-500 mt-1">
                    {formatBytes(folder.freeSpace)} free / {formatBytes(folder.totalSpace)} total
                  </p>
                </div>
                <button type="button" className={dangerButtonClass} onClick={() => void handleDeleteFolder(folder)}>
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
        <form onSubmit={(event) => void handleAddFolder(event)} className="grid grid-cols-1 sm:grid-cols-[1fr_140px_auto] gap-2">
          <TextInput value={folderPath} onChange={setFolderPath} placeholder="/data/media" />
          <Select
            value={folderType}
            onChange={setFolderType}
            options={[
              { value: 'tv', label: 'TV' },
              { value: 'movie', label: 'Movie' },
              { value: 'all', label: 'All' },
            ]}
          />
          <button type="submit" disabled={folderBusy} className={primaryButtonClass}>
            Add
          </button>
        </form>
      </section>
    </div>
  );
};
