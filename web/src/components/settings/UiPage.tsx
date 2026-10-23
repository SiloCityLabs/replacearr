import React from 'react';
import type { AppConfig, UiConfig } from '../../types';
import { formatAppDate } from '../../uiPrefs';
import {
  Checkbox,
  FieldRow,
  PageIntro,
  SaveBar,
  Section,
  Select,
  useDraftState,
} from './fields';

interface UiPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
}

export const UiPage: React.FC<UiPageProps> = ({ config, onSave, onDirtyChange }) => {
  const { draft, setDraft, isDirty, saving, setSaving, commit } = useDraftState(config.ui, onDirtyChange);

  const patch = <K extends keyof UiConfig>(key: K, value: UiConfig[K]) => {
    setDraft((current) => ({ ...current, [key]: value }));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const saved = await onSave({ ...config, ui: draft });
      commit(saved.ui);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const now = new Date().toISOString();

  return (
    <div>
      <PageIntro title="UI" caption="Calendar, dates, theme, and language." />
      <Section title="Calendar">
        <FieldRow label="First Day of Week">
          <Select
            value={draft.firstDayOfWeek}
            onChange={(value) => patch('firstDayOfWeek', Number(value))}
            options={[
              { value: 0, label: 'Sunday' },
              { value: 1, label: 'Monday' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Week Column Header">
          <Select
            value={draft.calendarWeekColumnHeader}
            onChange={(value) => patch('calendarWeekColumnHeader', value)}
            options={[
              { value: 'ddd M/D', label: 'ddd M/D' },
              { value: 'ddd MM/DD', label: 'ddd MM/DD' },
              { value: 'ddd D/M', label: 'ddd D/M' },
              { value: 'ddd DD/MM', label: 'ddd DD/MM' },
            ]}
          />
        </FieldRow>
      </Section>

      <Section title="Dates">
        <FieldRow label="Short Date Format">
          <Select
            value={draft.shortDateFormat}
            onChange={(value) => patch('shortDateFormat', value)}
            options={[
              { value: 'MMM D YYYY', label: 'MMM D YYYY' },
              { value: 'DD MMM YYYY', label: 'DD MMM YYYY' },
              { value: 'MM/DD/YYYY', label: 'MM/DD/YYYY' },
              { value: 'DD/MM/YYYY', label: 'DD/MM/YYYY' },
              { value: 'YYYY-MM-DD', label: 'YYYY-MM-DD' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Long Date Format">
          <Select
            value={draft.longDateFormat}
            onChange={(value) => patch('longDateFormat', value)}
            options={[
              { value: 'dddd, MMMM D YYYY', label: 'dddd, MMMM D YYYY' },
              { value: 'dddd, D MMMM YYYY', label: 'dddd, D MMMM YYYY' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Time Format">
          <Select
            value={draft.timeFormat}
            onChange={(value) => patch('timeFormat', value)}
            options={[
              { value: 'h:mm a', label: 'h:mm a' },
              { value: 'HH:mm', label: 'HH:mm' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Show Relative Dates">
          <Checkbox
            checked={draft.showRelativeDates}
            onChange={(value) => patch('showRelativeDates', value)}
          />
        </FieldRow>
        <FieldRow label="Preview">
          <div className="text-sm text-slate-200 space-y-1">
            <p>Short: {formatAppDate(now, draft, 'short')}</p>
            <p>Long: {formatAppDate(now, draft, 'long')}</p>
          </div>
        </FieldRow>
      </Section>

      <Section title="Appearance">
        <FieldRow label="Theme">
          <Select
            value={draft.theme}
            onChange={(value) => patch('theme', value)}
            options={[
              { value: 'auto', label: 'Auto' },
              { value: 'light', label: 'Light' },
              { value: 'dark', label: 'Dark' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Color Impaired Mode">
          <Checkbox
            checked={draft.enableColorImpairedMode}
            onChange={(value) => patch('enableColorImpairedMode', value)}
          />
        </FieldRow>
        <FieldRow label="Language">
          <Select
            value={draft.uiLanguage}
            onChange={(value) => patch('uiLanguage', value)}
            options={[{ value: 'en', label: 'English' }]}
          />
        </FieldRow>
      </Section>

      <SaveBar disabled={!isDirty} saving={saving} onSave={() => void handleSave()} />
    </div>
  );
};
