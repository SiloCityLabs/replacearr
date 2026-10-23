import React, { useEffect, useRef, useState } from 'react';
import { Eye, EyeOff, X } from 'lucide-react';

export const inputClass =
  'w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white';

const saveButtonClass =
  'inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 disabled:opacity-40 disabled:hover:bg-indigo-600 text-white text-xs font-semibold rounded-lg';

export function FieldRow({
  label,
  helpText,
  children,
}: {
  label: string;
  helpText?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-[240px_1fr] gap-2 md:gap-6 py-3 border-b border-slate-800/80">
      <div>
        <div className="text-sm font-medium text-slate-200">{label}</div>
        {helpText ? <p className="text-xs text-slate-500 mt-1">{helpText}</p> : null}
      </div>
      <div className="min-w-0">{children}</div>
    </div>
  );
}

export function Section({
  title,
  caption,
  children,
}: {
  title: string;
  caption?: string;
  children: React.ReactNode;
}) {
  return (
    <section className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 mb-4">
      <h3 className="text-sm font-semibold text-white">{title}</h3>
      {caption ? <p className="text-xs text-slate-500 mt-1">{caption}</p> : null}
      <div className="mt-1">{children}</div>
    </section>
  );
}

export function SaveBar({
  disabled,
  saving,
  onSave,
  showAdvanced,
  onToggleAdvanced,
}: {
  disabled: boolean;
  saving: boolean;
  onSave: () => void;
  showAdvanced?: boolean;
  onToggleAdvanced?: () => void;
}) {
  return (
    <div className="sticky bottom-0 z-20 mt-4 flex items-center justify-between gap-3 py-3 bg-slate-950/90 backdrop-blur border-t border-slate-800">
      <div>
        {onToggleAdvanced ? (
          <button
            type="button"
            onClick={onToggleAdvanced}
            className="text-xs font-medium text-slate-300 hover:text-white"
          >
            {showAdvanced ? 'Hide advanced' : 'Show advanced'}
          </button>
        ) : null}
      </div>
      <button
        type="button"
        disabled={disabled || saving}
        onClick={onSave}
        className={saveButtonClass}
      >
        {saving ? 'Saving...' : 'Save'}
      </button>
    </div>
  );
}

export function TextInput({
  value,
  onChange,
  disabled,
  readOnly,
  placeholder,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  readOnly?: boolean;
  placeholder?: string;
}) {
  return (
    <input
      type="text"
      className={inputClass}
      value={value}
      disabled={disabled}
      readOnly={readOnly}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}

export function NumberInput({
  value,
  onChange,
  disabled,
}: {
  value: number;
  onChange: (value: number) => void;
  disabled?: boolean;
}) {
  return (
    <input
      type="number"
      className={inputClass}
      value={Number.isFinite(value) ? value : 0}
      disabled={disabled}
      onChange={(e) => {
        const next = Number(e.target.value);
        onChange(Number.isFinite(next) ? next : 0);
      }}
    />
  );
}

export function Checkbox({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  disabled?: boolean;
}) {
  return (
    <label className="inline-flex items-center gap-2 text-sm text-slate-200">
      <input
        type="checkbox"
        className={`${inputClass} !w-4 !h-4 !p-0`}
        checked={checked}
        disabled={disabled}
        onChange={(e) => onChange(e.target.checked)}
      />
      {label}
    </label>
  );
}

export function Select({
  value,
  onChange,
  options,
  disabled,
}: {
  value: string | number;
  onChange: (value: string) => void;
  options: Array<{ value: string | number; label: string }>;
  disabled?: boolean;
}) {
  return (
    <select
      className={inputClass}
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
    >
      {options.map((option) => (
        <option key={String(option.value)} value={option.value}>
          {option.label}
        </option>
      ))}
    </select>
  );
}

export function PasswordInput({
  value,
  onChange,
  readOnly,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  disabled?: boolean;
}) {
  const [visible, setVisible] = useState(false);
  return (
    <div className="flex items-center gap-2">
      <input
        type={visible ? 'text' : 'password'}
        className={inputClass}
        value={value}
        readOnly={readOnly}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
      />
      <button
        type="button"
        onClick={() => setVisible((current) => !current)}
        className="shrink-0 inline-flex items-center gap-1 px-2.5 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-300"
        aria-label={visible ? 'Hide password' : 'Show password'}
      >
        {visible ? <EyeOff className="w-3.5 h-3.5" /> : <Eye className="w-3.5 h-3.5" />}
        {visible ? 'Hide' : 'Show'}
      </button>
    </div>
  );
}

export function StoredOnlyNote() {
  return (
    <p className="text-xs text-amber-200/90 bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2 mb-4">
      Saved in Replacearr. Import, renaming, notifications, and list sync do not run from these values yet.
    </p>
  );
}

export function Modal({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center p-4 bg-slate-950/70 overflow-y-auto">
      <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-xl p-5 my-8">
        <div className="flex items-center justify-between gap-3 mb-4">
          <h3 className="text-sm font-semibold text-white">{title}</h3>
          <button
            type="button"
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function PageIntro({ title, caption }: { title: string; caption?: string }) {
  return (
    <div className="mb-4">
      <h2 className="text-lg font-bold text-white">{title}</h2>
      {caption ? <p className="text-xs text-slate-400 mt-1">{caption}</p> : null}
    </div>
  );
}

export function useDraftState<T>(source: T, onDirtyChange: (dirty: boolean) => void) {
  const [draft, setDraft] = useState(source);
  const [baseline, setBaseline] = useState(source);
  const [saving, setSaving] = useState(false);
  const sourceKey = JSON.stringify(source);
  const seenSource = useRef(sourceKey);
  const isDirty = JSON.stringify(draft) !== JSON.stringify(baseline);

  useEffect(() => {
    onDirtyChange(isDirty);
  }, [isDirty, onDirtyChange]);

  useEffect(() => () => onDirtyChange(false), [onDirtyChange]);

  useEffect(() => {
    if (sourceKey === seenSource.current) return;
    if (JSON.stringify(draft) !== JSON.stringify(baseline)) return;
    seenSource.current = sourceKey;
    const parsed: unknown = JSON.parse(sourceKey);
    setDraft(parsed as T);
    setBaseline(parsed as T);
  }, [sourceKey, draft, baseline]);

  const commit = (next: T) => {
    seenSource.current = JSON.stringify(next);
    setDraft(next);
    setBaseline(next);
  };

  return { draft, setDraft, setBaseline, isDirty, saving, setSaving, commit };
}

export function errorText(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

export function parseNumberList(value: string): number[] {
  return value
    .split(',')
    .map((part) => Number(part.trim()))
    .filter((n) => Number.isFinite(n));
}

export function parseStringList(value: string): string[] {
  return value
    .split(',')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
}

export const primaryButtonClass =
  'inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg';

export const quietButtonClass =
  'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-300 hover:text-white';

export const dangerButtonClass =
  'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/30 text-xs font-medium';
