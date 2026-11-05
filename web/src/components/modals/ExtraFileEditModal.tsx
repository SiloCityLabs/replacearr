import React, { useState } from 'react';
import type { MovieExtraFile } from '../../types';
import { api } from '../../api';
import {
  errorText,
  FieldRow,
  Modal,
  primaryButtonClass,
  quietButtonClass,
  TextInput,
} from '../settings/fields';

interface ExtraFileEditModalProps {
  movieId: number;
  file: MovieExtraFile;
  onClose: () => void;
  onSaved: () => void;
  onNotify?: (msg: string, isError?: boolean) => void;
}

export const ExtraFileEditModal: React.FC<ExtraFileEditModalProps> = ({
  movieId,
  file,
  onClose,
  onSaved,
  onNotify,
}) => {
  const [name, setName] = useState(file.relativePath);
  const [saving, setSaving] = useState(false);

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) {
      onNotify?.('File name is required', true);
      return;
    }
    setSaving(true);
    try {
      await api.renameMovieExtraFile(movieId, file.relativePath, trimmed);
      onNotify?.('File renamed');
      onSaved();
      onClose();
    } catch (err) {
      onNotify?.(errorText(err, 'Failed to rename file'), true);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal title="Edit file" onClose={onClose}>
      <form onSubmit={(event) => void save(event)}>
        <FieldRow label="Type">
          <p className="text-sm text-slate-300 capitalize">{file.type || 'other'}</p>
        </FieldRow>
        <FieldRow label="Extension">
          <p className="text-sm font-mono text-slate-300">{file.extension || '—'}</p>
        </FieldRow>
        <FieldRow label="File name" helpText="Rename the sidecar inside the movie folder (e.g. Movie.en.srt).">
          <TextInput value={name} onChange={setName} />
        </FieldRow>
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className={quietButtonClass} onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className={primaryButtonClass} disabled={saving}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  );
};
