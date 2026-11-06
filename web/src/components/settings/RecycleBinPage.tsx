import React from 'react';
import type { AppConfig, MediaManagementConfig } from '../../types';
import type { NavView } from '../../nav';
import {
  FieldRow,
  NumberInput,
  PageIntro,
  SaveBar,
  Section,
  TextInput,
  useDraftState,
} from './fields';
import { useConfirm } from '../modals/ConfirmModal';

interface RecycleBinPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onNavigate: (view: NavView) => void;
}

function normalizeRecyclePath(value: string | undefined): string {
  return (value ?? '').trim();
}

export const RecycleBinPage: React.FC<RecycleBinPageProps> = ({
  config,
  onSave,
  onDirtyChange,
  onNavigate,
}) => {
  const { confirm, dialog } = useConfirm();
  const { draft, setDraft, baseline, isDirty, saving, setSaving, commit } = useDraftState(
    config.mediaManagement,
    onDirtyChange
  );

  const patch = <K extends keyof MediaManagementConfig>(key: K, value: MediaManagementConfig[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleSave = async () => {
    const prevPath = normalizeRecyclePath(baseline.recycleBin);
    const nextPath = normalizeRecyclePath(draft.recycleBin);
    if (prevPath !== nextPath) {
      const fromLabel = prevPath || '{config}/RecycleBin';
      const toLabel = nextPath || '{config}/RecycleBin';
      const ok = await confirm({
        title: 'Change recycle bin path?',
        message: `Changing the recycle bin path permanently empties the current bin at "${fromLabel}" and switches to "${toLabel}". Items in the old location cannot be recovered. Continue?`,
        confirmLabel: 'Empty old bin and change path',
        danger: true,
      });
      if (!ok) return;
    }

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

  return (
    <div>
      {dialog}
      <PageIntro
        title="Recycle Bin"
        caption="Deleted library files are moved here instead of being erased immediately. Path and retention are stored in app settings (not environment variables)."
      />

      <Section title="Location">
        <FieldRow
          label="Recycle Bin Path"
          helpText="Absolute path inside the container (or host bind mount). Leave blank to use {config}/RecycleBin — typically /config/RecycleBin in Docker."
        >
          <TextInput
            value={draft.recycleBin}
            onChange={(value) => patch('recycleBin', value)}
            placeholder="/config/RecycleBin"
          />
        </FieldRow>
      </Section>

      <Section title="Cleanup">
        <FieldRow
          label="Cleanup Days"
          helpText="The nightly Recycle Bin Cleanup task permanently deletes items older than this many days (default 30)."
        >
          <NumberInput
            value={draft.recycleBinCleanupDays}
            onChange={(value) => patch('recycleBinCleanupDays', value)}
          />
        </FieldRow>
        <p className="text-xs text-slate-500 pt-1">
          To empty the bin immediately, run{' '}
          <button
            type="button"
            className="text-teal-400 hover:text-teal-300 underline underline-offset-2"
            onClick={() => onNavigate('system.tasks')}
          >
            System → Tasks → Recycle Bin Cleanup
          </button>
          .
        </p>
      </Section>

      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />
    </div>
  );
};
