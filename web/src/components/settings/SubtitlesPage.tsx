import React from 'react';
import type { AppConfig, SubtitlesConfig } from '../../types';
import {
  Checkbox,
  FieldRow,
  PageIntro,
  PasswordInput,
  Section,
  SaveBar,
  TextInput,
  useDraftState,
} from './fields';

interface SubtitlesPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
}

const blankSubtitles = (): SubtitlesConfig => ({
  enabled: true,
  autoDownload: true,
  languages: ['en'],
  preferHearingImpaired: false,
  includeForced: true,
  openSubtitles: { apiKey: '', username: '', password: '' },
});

export const SubtitlesPage: React.FC<SubtitlesPageProps> = ({
  config,
  onSave,
  onDirtyChange,
}) => {
  const initial = config.subtitles ?? blankSubtitles();
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(
    {
      ...blankSubtitles(),
      ...initial,
      openSubtitles: {
        ...blankSubtitles().openSubtitles,
        ...(initial.openSubtitles || {}),
      },
      languages: initial.languages?.length ? initial.languages : ['en'],
    },
    onDirtyChange
  );

  const handleSave = async () => {
    setSaving(true);
    try {
      const languages = draft.languages
        .map((l) => l.trim().toLowerCase())
        .filter(Boolean);
      const payload: SubtitlesConfig = {
        ...draft,
        languages: languages.length ? languages : ['en'],
      };
      const saved = await onSave({ ...config, subtitles: payload });
      commit({
        ...blankSubtitles(),
        ...(saved.subtitles || payload),
        openSubtitles: {
          ...blankSubtitles().openSubtitles,
          ...(saved.subtitles?.openSubtitles || payload.openSubtitles),
        },
      });
    } catch {
      // Parent already reported.
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-4">
      <PageIntro
        title="Subtitles"
        caption="Native OpenSubtitles integration for Movies and Series. Sidecars are written next to video files — no Bazarr required."
      />

      <Section title="Behavior">
        <FieldRow label="Enable">
          <Checkbox
            label="Enable subtitles"
            checked={draft.enabled}
            onChange={(enabled) => setDraft((d) => ({ ...d, enabled }))}
          />
        </FieldRow>
        <FieldRow label="Auto download">
          <Checkbox
            label="Download after import"
            checked={draft.autoDownload}
            onChange={(autoDownload) => setDraft((d) => ({ ...d, autoDownload }))}
          />
        </FieldRow>
        <FieldRow label="Hearing impaired">
          <Checkbox
            label="Prefer hearing-impaired tracks"
            checked={draft.preferHearingImpaired}
            onChange={(preferHearingImpaired) =>
              setDraft((d) => ({ ...d, preferHearingImpaired }))
            }
          />
        </FieldRow>
        <FieldRow label="Forced">
          <Checkbox
            label="Include forced / foreign-parts-only"
            checked={draft.includeForced}
            onChange={(includeForced) => setDraft((d) => ({ ...d, includeForced }))}
          />
        </FieldRow>
        <FieldRow label="Languages" helpText="Comma-separated ISO codes (en, es, fr)">
          <TextInput
            value={draft.languages.join(', ')}
            onChange={(value) =>
              setDraft((d) => ({
                ...d,
                languages: value.split(/[,\s]+/).filter(Boolean),
              }))
            }
            placeholder="en, es"
          />
        </FieldRow>
      </Section>

      <Section title="OpenSubtitles.com">
        <FieldRow
          label="API key"
          helpText="Create a consumer key at opensubtitles.com/consumers"
        >
          <TextInput
            value={draft.openSubtitles.apiKey}
            onChange={(apiKey) =>
              setDraft((d) => ({
                ...d,
                openSubtitles: { ...d.openSubtitles, apiKey },
              }))
            }
            placeholder="Api-Key"
          />
        </FieldRow>
        <FieldRow label="Username" helpText="Optional — raises daily download quota">
          <TextInput
            value={draft.openSubtitles.username}
            onChange={(username) =>
              setDraft((d) => ({
                ...d,
                openSubtitles: { ...d.openSubtitles, username },
              }))
            }
          />
        </FieldRow>
        <FieldRow label="Password">
          <PasswordInput
            value={draft.openSubtitles.password}
            onChange={(password) =>
              setDraft((d) => ({
                ...d,
                openSubtitles: { ...d.openSubtitles, password },
              }))
            }
          />
        </FieldRow>
      </Section>

      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />
    </div>
  );
};
