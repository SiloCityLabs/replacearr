import React, { useEffect, useState } from 'react';
import type { MediaFileInfo, MediaFileUpdate, QualityDefinition } from '../../types';
import { api } from '../../api';
import {
  errorText,
  FieldRow,
  Modal,
  primaryButtonClass,
  quietButtonClass,
  Select,
  TextInput,
} from '../settings/fields';

interface MediaFileEditModalProps {
  movieId: number;
  file: MediaFileInfo;
  onClose: () => void;
  onSaved: (file: MediaFileInfo) => void;
  onNotify?: (msg: string, isError?: boolean) => void;
}

const COMMON_LANGS = [
  { value: 'eng', label: 'English (eng)' },
  { value: 'spa', label: 'Spanish (spa)' },
  { value: 'fra', label: 'French (fra)' },
  { value: 'deu', label: 'German (deu)' },
  { value: 'ita', label: 'Italian (ita)' },
  { value: 'por', label: 'Portuguese (por)' },
  { value: 'jpn', label: 'Japanese (jpn)' },
  { value: 'kor', label: 'Korean (kor)' },
  { value: 'zho', label: 'Chinese (zho)' },
  { value: 'rus', label: 'Russian (rus)' },
  { value: 'hin', label: 'Hindi (hin)' },
  { value: 'ara', label: 'Arabic (ara)' },
];

export const MediaFileEditModal: React.FC<MediaFileEditModalProps> = ({
  movieId,
  file,
  onClose,
  onSaved,
  onNotify,
}) => {
  const [qualities, setQualities] = useState<QualityDefinition[]>([]);
  const [quality, setQuality] = useState(file.quality || 'Unknown');
  const [languages, setLanguages] = useState(file.languages || '');
  const [indexer, setIndexer] = useState(file.indexer || '');
  const [releaseGroup, setReleaseGroup] = useState(file.releaseGroup || '');
  const [sceneName, setSceneName] = useState(file.sceneName || '');
  const [videoCodec, setVideoCodec] = useState(file.videoCodec || '');
  const [audioCodec, setAudioCodec] = useState(file.audioCodec || '');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    void api
      .getQualities()
      .then((defs) => setQualities(defs.filter((q) => q.resolution > 0 || q.name === 'Unknown')))
      .catch(() => setQualities([]));
  }, []);

  const qualityOptions = (() => {
    const names = new Set(qualities.map((q) => q.name));
    if (quality && !names.has(quality)) {
      return [{ value: quality, label: quality }, ...qualities.map((q) => ({ value: q.name, label: q.name }))];
    }
    return qualities.map((q) => ({ value: q.name, label: q.name }));
  })();

  const save = async (event: React.FormEvent) => {
    event.preventDefault();
    setSaving(true);
    try {
      const matched = qualities.find((q) => q.name === quality);
      const payload: MediaFileUpdate = {
        quality,
        languages: languages.trim(),
        indexer: indexer.trim(),
        releaseGroup: releaseGroup.trim(),
        sceneName: sceneName.trim(),
        videoCodec: videoCodec.trim(),
        audioCodec: audioCodec.trim(),
        resolution: matched?.resolution || file.resolution || 0,
      };
      const updated = await api.updateMovieFile(movieId, file.id, payload);
      onNotify?.('File metadata saved');
      onSaved(updated);
      onClose();
    } catch (err) {
      onNotify?.(errorText(err, 'Failed to save file metadata'), true);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal title="Edit media file" onClose={onClose}>
      <form onSubmit={(event) => void save(event)}>
        <p className="text-xs text-slate-500 mb-3 font-mono break-all">{file.relativePath || file.path}</p>
        <FieldRow label="Quality" helpText="Override the auto-detected quality label.">
          {qualityOptions.length > 0 ? (
            <Select value={quality} onChange={setQuality} options={qualityOptions} />
          ) : (
            <TextInput value={quality} onChange={setQuality} />
          )}
        </FieldRow>
        <FieldRow label="Languages" helpText="Audio language tags (comma-separated), e.g. eng or eng, spa.">
          <TextInput value={languages} onChange={setLanguages} placeholder="eng" />
          <div className="mt-2 flex flex-wrap gap-1.5">
            {COMMON_LANGS.map((lang) => (
              <button
                key={lang.value}
                type="button"
                className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-[11px] text-slate-300 hover:border-sky-500/40 hover:text-sky-300"
                onClick={() => {
                  const parts = languages
                    .split(',')
                    .map((p) => p.trim())
                    .filter(Boolean);
                  if (parts.includes(lang.value)) return;
                  setLanguages([...parts, lang.value].join(', '));
                }}
              >
                {lang.value}
              </button>
            ))}
          </div>
        </FieldRow>
        <FieldRow label="Indexer" helpText="Indexer that provided this release, if known.">
          <TextInput value={indexer} onChange={setIndexer} placeholder="Optional" />
        </FieldRow>
        <FieldRow label="Release group">
          <TextInput value={releaseGroup} onChange={setReleaseGroup} />
        </FieldRow>
        <FieldRow label="Scene / release name">
          <TextInput value={sceneName} onChange={setSceneName} />
        </FieldRow>
        <FieldRow label="Video codec">
          <TextInput value={videoCodec} onChange={setVideoCodec} placeholder="x264" />
        </FieldRow>
        <FieldRow label="Audio codec">
          <TextInput value={audioCodec} onChange={setAudioCodec} placeholder="AAC" />
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
