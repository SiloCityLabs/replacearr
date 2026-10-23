import React, { useEffect, useState } from 'react';
import type {
  AppConfig,
  DelayProfile,
  Indexer,
  QualityDefinition,
  QualityItem,
  QualityProfile,
  ReleaseProfile,
} from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  dangerButtonClass,
  errorText,
  FieldRow,
  Modal,
  NumberInput,
  PageIntro,
  parseStringList,
  primaryButtonClass,
  quietButtonClass,
  Section,
  Select,
  TextInput,
} from './fields';

interface ProfilesPageProps {
  config: AppConfig;
  profiles: QualityProfile[];
  indexers: Indexer[];
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

interface ProfileForm {
  id: number | null;
  name: string;
  upgradeAllowed: boolean;
  cutoff: number;
  items: QualityItem[];
  minFormatScore: number;
  cutoffFormatScore: number;
}

function qualitiesToItems(qualities: QualityDefinition[], existing?: QualityItem[]): QualityItem[] {
  const allowed = new Map((existing ?? []).map((item) => [item.id, item.allowed]));
  return qualities.map((quality) => ({
    id: quality.id,
    name: quality.name,
    source: quality.source,
    resolution: quality.resolution,
    allowed: allowed.get(quality.id) ?? false,
  }));
}

export const ProfilesPage: React.FC<ProfilesPageProps> = ({
  config,
  profiles,
  indexers,
  onSave,
  onRefresh,
  onNotify,
}) => {
  const [qualities, setQualities] = useState<QualityDefinition[]>(config.qualityDefinitions);
  const [profileForm, setProfileForm] = useState<ProfileForm | null>(null);
  const [delayForm, setDelayForm] = useState<{ index: number | null; value: DelayProfile } | null>(null);
  const [releaseForm, setReleaseForm] = useState<{
    index: number | null;
    value: ReleaseProfile;
    requiredText: string;
    ignoredText: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let active = true;
    api
      .getQualities()
      .then((rows) => {
        if (!active) return;
        setQualities(rows.length > 0 ? rows : config.qualityDefinitions);
      })
      .catch(() => {
        if (active) setQualities(config.qualityDefinitions);
      });
    return () => {
      active = false;
    };
  }, [config.qualityDefinitions]);

  const openCreateProfile = () => {
    const items = qualitiesToItems(qualities);
    setProfileForm({
      id: null,
      name: '',
      upgradeAllowed: true,
      cutoff: items[0]?.id ?? 0,
      items,
      minFormatScore: 0,
      cutoffFormatScore: 0,
    });
  };

  const openEditProfile = (profile: QualityProfile) => {
    const source = qualities.length > 0 ? qualities : config.qualityDefinitions;
    setProfileForm({
      id: profile.id,
      name: profile.name,
      upgradeAllowed: profile.upgradeAllowed,
      cutoff: profile.cutoff,
      items: qualitiesToItems(source, profile.items),
      minFormatScore: profile.minFormatScore,
      cutoffFormatScore: profile.cutoffFormatScore,
    });
  };

  const saveProfile = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!profileForm) return;
    setBusy(true);
    const payload = {
      name: profileForm.name,
      upgradeAllowed: profileForm.upgradeAllowed,
      cutoff: profileForm.cutoff,
      items: profileForm.items,
      minFormatScore: profileForm.minFormatScore,
      cutoffFormatScore: profileForm.cutoffFormatScore,
    };
    try {
      if (profileForm.id == null) {
        const created = await api.createProfile(payload);
        onNotify(`Created quality profile "${created.name}"`);
      } else {
        const updated = await api.updateProfile(profileForm.id, payload);
        onNotify(`Updated quality profile "${updated.name}"`);
      }
      setProfileForm(null);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to save quality profile'), true);
    } finally {
      setBusy(false);
    }
  };

  const deleteProfile = async (profile: QualityProfile) => {
    if (!window.confirm(`Delete quality profile "${profile.name}"?`)) return;
    try {
      await api.deleteProfile(profile.id);
      onNotify(`Deleted quality profile "${profile.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(errorText(err, 'Failed to delete quality profile'), true);
    }
  };

  const saveDelay = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!delayForm) return;
    const next = [...config.delayProfiles];
    if (delayForm.index == null) next.push(delayForm.value);
    else next[delayForm.index] = delayForm.value;
    setBusy(true);
    try {
      await onSave({ ...config, delayProfiles: next });
      setDelayForm(null);
    } catch {
      // Parent already reported the error.
    } finally {
      setBusy(false);
    }
  };

  const deleteDelay = async (index: number) => {
    if (!window.confirm('Delete this delay profile?')) return;
    try {
      await onSave({
        ...config,
        delayProfiles: config.delayProfiles.filter((_, itemIndex) => itemIndex !== index),
      });
    } catch {
      // Parent already reported the error.
    }
  };

  const saveRelease = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!releaseForm) return;
    const value: ReleaseProfile = {
      ...releaseForm.value,
      required: parseStringList(releaseForm.requiredText),
      ignored: parseStringList(releaseForm.ignoredText),
    };
    const next = [...config.releaseProfiles];
    if (releaseForm.index == null) next.push(value);
    else next[releaseForm.index] = value;
    setBusy(true);
    try {
      await onSave({ ...config, releaseProfiles: next });
      setReleaseForm(null);
    } catch {
      // Parent already reported the error.
    } finally {
      setBusy(false);
    }
  };

  const deleteRelease = async (index: number) => {
    if (!window.confirm('Delete this release profile?')) return;
    try {
      await onSave({
        ...config,
        releaseProfiles: config.releaseProfiles.filter((_, itemIndex) => itemIndex !== index),
      });
    } catch {
      // Parent already reported the error.
    }
  };

  const allowedItems = profileForm?.items.filter((item) => item.allowed) ?? [];
  const cutoffOptions =
    profileForm == null
      ? []
      : allowedItems.some((item) => item.id === profileForm.cutoff) || profileForm.cutoff === 0
        ? allowedItems
        : [
            ...allowedItems,
            profileForm.items.find((item) => item.id === profileForm.cutoff) ?? {
              id: profileForm.cutoff,
              name: `Quality ${profileForm.cutoff}`,
              source: '',
              resolution: 0,
              allowed: true,
            },
          ];

  return (
    <div>
      <PageIntro title="Profiles" caption="Quality profiles are saved immediately. Delay and release profiles are stored only." />

      <Section title="Quality Profiles">
        <div className="flex justify-end py-3">
          <button type="button" className={primaryButtonClass} onClick={openCreateProfile}>
            Add Quality Profile
          </button>
        </div>
        {profiles.length === 0 ? (
          <p className="text-xs text-slate-400 pb-3">No quality profiles yet.</p>
        ) : (
          <div className="space-y-2 pb-3">
            {profiles.map((profile) => (
              <div
                key={profile.id}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3"
              >
                <div>
                  <p className="text-sm font-medium text-white">{profile.name}</p>
                  <p className="text-xs text-slate-400 mt-1">
                    Cutoff {profile.cutoff} · Upgrades {profile.upgradeAllowed ? 'allowed' : 'off'}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button type="button" className={quietButtonClass} onClick={() => openEditProfile(profile)}>
                    Edit
                  </button>
                  <button type="button" className={dangerButtonClass} onClick={() => void deleteProfile(profile)}>
                    Delete
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </Section>

      <Section title="Delay Profiles" caption="Saved in Replacearr. Nothing delays a grab from these profiles yet.">
        <div className="flex justify-end py-3">
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() =>
              setDelayForm({
                index: null,
                value: {
                  id: 0,
                  enableUsenet: true,
                  enableTorrent: true,
                  preferredProtocol: 'preferUsenet',
                  usenetDelay: 0,
                  torrentDelay: 0,
                  bypassIfHighestQuality: false,
                  bypassIfAboveCustomFormatScore: false,
                  minimumCustomFormatScore: 0,
                  order: config.delayProfiles.length + 1,
                  tags: [],
                },
              })
            }
          >
            Add Delay Profile
          </button>
        </div>
        <div className="space-y-2 pb-3">
          {config.delayProfiles.map((profile, index) => (
            <div
              key={`${profile.id}-${index}`}
              className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-3"
            >
              <p className="text-sm text-slate-200">{profile.preferredProtocol}</p>
              <div className="flex gap-2">
                <button
                  type="button"
                  className={quietButtonClass}
                  onClick={() => setDelayForm({ index, value: profile })}
                >
                  Edit
                </button>
                <button type="button" className={dangerButtonClass} onClick={() => void deleteDelay(index)}>
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      </Section>

      <Section title="Release Profiles" caption="Saved in Replacearr. Required and ignored terms are not applied to searches yet.">
        <div className="flex justify-end py-3">
          <button
            type="button"
            className={primaryButtonClass}
            onClick={() =>
              setReleaseForm({
                index: null,
                requiredText: '',
                ignoredText: '',
                value: {
                  id: 0,
                  enabled: true,
                  name: '',
                  required: [],
                  ignored: [],
                  indexerId: 0,
                  tags: [],
                },
              })
            }
          >
            Add Release Profile
          </button>
        </div>
        <div className="space-y-2 pb-3">
          {config.releaseProfiles.map((profile, index) => (
            <div
              key={`${profile.id}-${index}`}
              className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-3"
            >
              <div>
                <p className="text-sm text-white">{profile.name || 'Untitled'}</p>
                <p className="text-xs text-slate-400">{profile.enabled ? 'Enabled' : 'Disabled'}</p>
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  className={quietButtonClass}
                  onClick={() =>
                    setReleaseForm({
                      index,
                      value: profile,
                      requiredText: profile.required.join(', '),
                      ignoredText: profile.ignored.join(', '),
                    })
                  }
                >
                  Edit
                </button>
                <button type="button" className={dangerButtonClass} onClick={() => void deleteRelease(index)}>
                  Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      </Section>

      {profileForm ? (
        <Modal title={profileForm.id == null ? 'Add Quality Profile' : 'Edit Quality Profile'} onClose={() => setProfileForm(null)}>
          <form onSubmit={(event) => void saveProfile(event)} className="space-y-1">
            <FieldRow label="Name">
              <TextInput
                value={profileForm.name}
                onChange={(value) => setProfileForm({ ...profileForm, name: value })}
              />
            </FieldRow>
            <FieldRow label="Upgrades Allowed">
              <Checkbox
                checked={profileForm.upgradeAllowed}
                onChange={(value) => setProfileForm({ ...profileForm, upgradeAllowed: value })}
              />
            </FieldRow>
            <FieldRow label="Cutoff">
              <Select
                value={profileForm.cutoff}
                onChange={(value) => setProfileForm({ ...profileForm, cutoff: Number(value) })}
                options={cutoffOptions.map((item) => ({
                  value: item.id,
                  label: item.name,
                }))}
              />
            </FieldRow>
            <FieldRow label="Qualities">
              <div className="space-y-2">
                {profileForm.items.map((item) => {
                  const definition = qualities.find((quality) => quality.id === item.id);
                  const title = definition?.title || item.name;
                  return (
                  <label key={item.id} className="flex items-center justify-between gap-3 text-sm text-slate-200">
                    <span>
                      {title === item.name ? title : `${title} (${item.name})`} · {item.resolution}
                    </span>
                    <input
                      type="checkbox"
                      className="w-4 h-4"
                      checked={item.allowed}
                      onChange={(event) =>
                        setProfileForm({
                          ...profileForm,
                          items: profileForm.items.map((row) =>
                            row.id === item.id ? { ...row, allowed: event.target.checked } : row
                          ),
                        })
                      }
                    />
                  </label>
                  );
                })}
              </div>
            </FieldRow>
            <FieldRow label="Minimum Format Score">
              <NumberInput
                value={profileForm.minFormatScore}
                onChange={(value) => setProfileForm({ ...profileForm, minFormatScore: value })}
              />
            </FieldRow>
            <FieldRow label="Cutoff Format Score">
              <NumberInput
                value={profileForm.cutoffFormatScore}
                onChange={(value) => setProfileForm({ ...profileForm, cutoffFormatScore: value })}
              />
            </FieldRow>
            <div className="pt-3 flex justify-end">
              <button type="submit" disabled={busy} className={primaryButtonClass}>
                Save
              </button>
            </div>
          </form>
        </Modal>
      ) : null}

      {delayForm ? (
        <Modal title={delayForm.index == null ? 'Add Delay Profile' : 'Edit Delay Profile'} onClose={() => setDelayForm(null)}>
          <form onSubmit={(event) => void saveDelay(event)}>
            <FieldRow label="Preferred Protocol">
              <Select
                value={delayForm.value.preferredProtocol}
                onChange={(value) =>
                  setDelayForm({ ...delayForm, value: { ...delayForm.value, preferredProtocol: value } })
                }
                options={[
                  { value: 'preferUsenet', label: 'Prefer Usenet' },
                  { value: 'preferTorrent', label: 'Prefer Torrent' },
                  { value: 'onlyUsenet', label: 'Only Usenet' },
                  { value: 'onlyTorrent', label: 'Only Torrent' },
                ]}
              />
            </FieldRow>
            <FieldRow label="Enable Usenet">
              <Checkbox
                checked={delayForm.value.enableUsenet}
                onChange={(value) =>
                  setDelayForm({ ...delayForm, value: { ...delayForm.value, enableUsenet: value } })
                }
              />
            </FieldRow>
            <FieldRow label="Enable Torrent">
              <Checkbox
                checked={delayForm.value.enableTorrent}
                onChange={(value) =>
                  setDelayForm({ ...delayForm, value: { ...delayForm.value, enableTorrent: value } })
                }
              />
            </FieldRow>
            <FieldRow label="Usenet Delay">
              <NumberInput
                value={delayForm.value.usenetDelay}
                onChange={(value) =>
                  setDelayForm({ ...delayForm, value: { ...delayForm.value, usenetDelay: value } })
                }
              />
            </FieldRow>
            <FieldRow label="Torrent Delay">
              <NumberInput
                value={delayForm.value.torrentDelay}
                onChange={(value) =>
                  setDelayForm({ ...delayForm, value: { ...delayForm.value, torrentDelay: value } })
                }
              />
            </FieldRow>
            <FieldRow label="Bypass If Highest Quality">
              <Checkbox
                checked={delayForm.value.bypassIfHighestQuality}
                onChange={(value) =>
                  setDelayForm({
                    ...delayForm,
                    value: { ...delayForm.value, bypassIfHighestQuality: value },
                  })
                }
              />
            </FieldRow>
            <FieldRow label="Bypass If Above Custom Format Score">
              <Checkbox
                checked={delayForm.value.bypassIfAboveCustomFormatScore}
                onChange={(value) =>
                  setDelayForm({
                    ...delayForm,
                    value: { ...delayForm.value, bypassIfAboveCustomFormatScore: value },
                  })
                }
              />
            </FieldRow>
            <FieldRow label="Minimum Custom Format Score">
              <NumberInput
                value={delayForm.value.minimumCustomFormatScore}
                onChange={(value) =>
                  setDelayForm({
                    ...delayForm,
                    value: { ...delayForm.value, minimumCustomFormatScore: value },
                  })
                }
              />
            </FieldRow>
            <div className="pt-3 flex justify-end">
              <button type="submit" disabled={busy} className={primaryButtonClass}>
                Save
              </button>
            </div>
          </form>
        </Modal>
      ) : null}

      {releaseForm ? (
        <Modal
          title={releaseForm.index == null ? 'Add Release Profile' : 'Edit Release Profile'}
          onClose={() => setReleaseForm(null)}
        >
          <form onSubmit={(event) => void saveRelease(event)}>
            <FieldRow label="Name">
              <TextInput
                value={releaseForm.value.name}
                onChange={(value) =>
                  setReleaseForm({ ...releaseForm, value: { ...releaseForm.value, name: value } })
                }
              />
            </FieldRow>
            <FieldRow label="Enabled">
              <Checkbox
                checked={releaseForm.value.enabled}
                onChange={(value) =>
                  setReleaseForm({ ...releaseForm, value: { ...releaseForm.value, enabled: value } })
                }
              />
            </FieldRow>
            <FieldRow label="Required">
              <TextInput
                value={releaseForm.requiredText}
                onChange={(value) => setReleaseForm({ ...releaseForm, requiredText: value })}
                placeholder="comma-separated"
              />
            </FieldRow>
            <FieldRow label="Ignored">
              <TextInput
                value={releaseForm.ignoredText}
                onChange={(value) => setReleaseForm({ ...releaseForm, ignoredText: value })}
                placeholder="comma-separated"
              />
            </FieldRow>
            <FieldRow label="Indexer">
              <Select
                value={releaseForm.value.indexerId}
                onChange={(value) =>
                  setReleaseForm({
                    ...releaseForm,
                    value: { ...releaseForm.value, indexerId: Number(value) },
                  })
                }
                options={[
                  { value: 0, label: 'Any' },
                  ...indexers.map((indexer) => ({ value: indexer.id, label: indexer.name })),
                ]}
              />
            </FieldRow>
            <div className="pt-3 flex justify-end">
              <button type="submit" disabled={busy} className={primaryButtonClass}>
                Save
              </button>
            </div>
          </form>
        </Modal>
      ) : null}
    </div>
  );
};
