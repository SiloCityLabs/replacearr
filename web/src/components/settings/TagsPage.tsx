import React, { useEffect, useState } from 'react';
import type { Tag, TagDetails } from '../../types';
import { api } from '../../api';
import {
  dangerButtonClass,
  errorText,
  PageIntro,
  primaryButtonClass,
  TextInput,
} from './fields';

const TAG_PATTERN = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

const COUNT_FIELDS: Array<{ key: keyof Omit<TagDetails, 'id' | 'label'>; label: string }> = [
  { key: 'seriesIds', label: 'Series' },
  { key: 'movieIds', label: 'Movies' },
  { key: 'indexerIds', label: 'Indexers' },
  { key: 'downloadClientIds', label: 'Download Clients' },
  { key: 'delayProfileIds', label: 'Delay Profiles' },
  { key: 'releaseProfileIds', label: 'Release Profiles' },
  { key: 'notificationIds', label: 'Notifications' },
  { key: 'importListIds', label: 'Import Lists' },
];

interface TagsPageProps {
  onNotify: (msg: string, isError?: boolean) => void;
}

export const TagsPage: React.FC<TagsPageProps> = ({ onNotify }) => {
  const [tags, setTags] = useState<Tag[]>([]);
  const [label, setLabel] = useState('');
  const [error, setError] = useState('');
  const [details, setDetails] = useState<TagDetails | null>(null);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    const rows = await api.getTags();
    setTags(rows);
  };

  useEffect(() => {
    let active = true;
    api
      .getTags()
      .then((rows) => {
        if (active) setTags(rows);
      })
      .catch((err) => {
        if (active) onNotify(errorText(err, 'Failed to load tags'), true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [onNotify]);

  const handleAdd = async (event: React.FormEvent) => {
    event.preventDefault();
    const next = label.trim();
    if (!TAG_PATTERN.test(next)) {
      setError('Use lowercase letters, numbers, and single hyphens.');
      return;
    }
    setError('');
    try {
      const created = await api.createTag(next);
      setLabel('');
      onNotify(`Added tag "${created.label}"`);
      await load();
    } catch (err) {
      onNotify(errorText(err, 'Failed to add tag'), true);
    }
  };

  const handleDelete = async (tag: Tag) => {
    if (!window.confirm(`Delete tag "${tag.label}"?`)) return;
    try {
      await api.deleteTag(tag.id);
      if (details?.id === tag.id) setDetails(null);
      onNotify(`Deleted tag "${tag.label}"`);
      await load();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete tag'), true);
    }
  };

  const handleOpen = async (tag: Tag) => {
    try {
      const row = await api.getTagDetails(tag.id);
      setDetails(row);
    } catch (err) {
      onNotify(errorText(err, 'Failed to load tag details'), true);
    }
  };

  const unused =
    details != null && COUNT_FIELDS.every((field) => details[field.key].length === 0);

  return (
    <div>
      <PageIntro title="Tags" caption="Labels for series, movies, indexers, and profiles." />
      <form onSubmit={(event) => void handleAdd(event)} className="flex flex-col sm:flex-row gap-2 mb-4">
        <div className="flex-1">
          <TextInput value={label} onChange={setLabel} placeholder="anime" />
          {error ? <p className="text-xs text-rose-300 mt-1">{error}</p> : null}
        </div>
        <button type="submit" className={primaryButtonClass}>
          Add Tag
        </button>
      </form>
      {loading ? (
        <p className="text-sm text-slate-400">Loading tags...</p>
      ) : tags.length === 0 ? (
        <p className="text-sm text-slate-400">No tags yet.</p>
      ) : (
        <div className="space-y-2">
          {tags.map((tag) => (
            <div
              key={tag.id}
              className="p-3 rounded-xl bg-slate-900/90 border border-slate-800 flex items-center justify-between gap-3"
            >
              <button type="button" onClick={() => void handleOpen(tag)} className="text-sm text-white text-left">
                {tag.label}
              </button>
              <button type="button" className={dangerButtonClass} onClick={() => void handleDelete(tag)}>
                Delete
              </button>
            </div>
          ))}
        </div>
      )}
      {details ? (
        <div className="mt-4 bg-slate-900/90 border border-slate-800 rounded-xl p-5">
          <h3 className="text-sm font-semibold text-white">{details.label}</h3>
          {unused ? (
            <p className="text-xs text-slate-400 mt-2">Not used yet.</p>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 mt-3">
              {COUNT_FIELDS.map((field) => (
                <div key={field.key} className="rounded-lg bg-slate-950 border border-slate-800 px-3 py-2">
                  <p className="text-[11px] text-slate-500">{field.label}</p>
                  <p className="text-sm text-white">{details[field.key].length}</p>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
};
