import React, { useState } from 'react';
import type { Movie, QualityProfile, Tag } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  errorText,
  FieldRow,
  Modal,
  primaryButtonClass,
  quietButtonClass,
  Select,
  TextInput,
} from '../settings/fields';

interface MovieEditModalProps {
  movie: Movie;
  profiles: QualityProfile[];
  tags: Tag[];
  onClose: () => void;
  onSaved: (movie: Movie) => void;
  onNotify?: (msg: string, isError?: boolean) => void;
}

export const MovieEditModal: React.FC<MovieEditModalProps> = ({
  movie,
  profiles,
  tags,
  onClose,
  onSaved,
  onNotify,
}) => {
  const [monitored, setMonitored] = useState(movie.monitored);
  const [qualityProfileId, setQualityProfileId] = useState(
    movie.qualityProfileId || profiles[0]?.id || 1,
  );
  const [path, setPath] = useState(movie.path || '');
  const [selectedTags, setSelectedTags] = useState<number[]>(movie.tags || []);
  const [minAvailability, setMinAvailability] = useState(movie.minimumAvailability || 'released');
  const [saving, setSaving] = useState(false);

  const toggleTag = (id: number) => {
    setSelectedTags((prev) => (prev.includes(id) ? prev.filter((t) => t !== id) : [...prev, id]));
  };

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    try {
      const updated = await api.updateMovie(movie.id, {
        ...movie,
        monitored,
        qualityProfileId,
        path,
        tags: selectedTags,
        minimumAvailability: minAvailability,
      });
      onNotify?.('Movie settings saved');
      onSaved(updated);
      onClose();
    } catch (err) {
      onNotify?.(errorText(err, 'Failed to save movie'), true);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal title={`Edit ${movie.title}`} onClose={onClose}>
      <form onSubmit={(event) => void save(event)}>
        <FieldRow label="Monitored">
          <Checkbox checked={monitored} onChange={setMonitored} />
        </FieldRow>
        <FieldRow label="Quality profile">
          <Select
            value={String(qualityProfileId)}
            onChange={(value) => setQualityProfileId(Number(value))}
            options={profiles.map((p) => ({ value: String(p.id), label: p.name }))}
          />
        </FieldRow>
        <FieldRow label="Path" helpText="Folder path for this movie on disk.">
          <TextInput value={path} onChange={setPath} />
        </FieldRow>
        <FieldRow label="Minimum availability">
          <Select
            value={minAvailability}
            onChange={setMinAvailability}
            options={[
              { value: 'announced', label: 'Announced' },
              { value: 'inCinemas', label: 'In Cinemas' },
              { value: 'released', label: 'Released' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Tags">
          {tags.length === 0 ? (
            <p className="text-sm text-slate-500">No tags defined</p>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              {tags.map((t) => {
                const on = selectedTags.includes(t.id);
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => toggleTag(t.id)}
                    className={`px-2 py-1 rounded-full border text-xs ${
                      on
                        ? 'bg-sky-500/20 text-sky-300 border-sky-500/40'
                        : 'bg-slate-900 text-slate-400 border-slate-700'
                    }`}
                  >
                    {t.label}
                  </button>
                );
              })}
            </div>
          )}
        </FieldRow>
        <div className="pt-3 flex justify-end gap-2">
          <button type="button" className={quietButtonClass} onClick={onClose} disabled={saving}>
            Cancel
          </button>
          <button type="submit" disabled={saving} className={primaryButtonClass}>
            {saving ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  );
};
