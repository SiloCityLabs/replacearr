import React, { useCallback, useEffect, useId, useState } from 'react';
import { X } from 'lucide-react';
import { dangerButtonClass, quietButtonClass } from '../settings/fields';

export type ConfirmCheckbox = {
  id: string;
  label: string;
  description?: string;
  defaultChecked?: boolean;
};

export type ConfirmOptions = {
  title?: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  /** Defaults to true (rose danger styling). */
  danger?: boolean;
  checkboxes?: ConfirmCheckbox[];
};

/** Confirmed checkbox map, or false when cancelled. */
export type ConfirmResult = false | Record<string, boolean>;

type ConfirmModalProps = ConfirmOptions & {
  onConfirm: (values: Record<string, boolean>) => void;
  onCancel: () => void;
};

function initialCheckboxValues(checkboxes: ConfirmCheckbox[] | undefined): Record<string, boolean> {
  const values: Record<string, boolean> = {};
  for (const box of checkboxes ?? []) {
    values[box.id] = box.defaultChecked ?? false;
  }
  return values;
}

export const ConfirmModal: React.FC<ConfirmModalProps> = ({
  title = 'Confirm',
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = true,
  checkboxes,
  onConfirm,
  onCancel,
}) => {
  const titleId = useId();
  const messageId = useId();
  const [values, setValues] = useState(() => initialCheckboxValues(checkboxes));

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        // Capture + stop so parent dialogs (e.g. Music artist panel) do not also close.
        event.stopImmediatePropagation();
        onCancel();
      }
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  }, [onCancel]);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4 bg-slate-950/70"
      role="presentation"
      onClick={onCancel}
    >
      <div
        role="alertdialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={messageId}
        className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-xl p-5 shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-3 mb-3">
          <h3 id={titleId} className="text-sm font-semibold text-white">
            {title}
          </h3>
          <button
            type="button"
            onClick={onCancel}
            className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800"
            aria-label="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <p id={messageId} className="text-sm text-slate-300 leading-relaxed">
          {message}
        </p>
        {checkboxes && checkboxes.length > 0 ? (
          <div className="mt-4 space-y-3">
            {checkboxes.map((box) => {
              const descId = `${box.id}-desc`;
              return (
                <label
                  key={box.id}
                  className="flex items-start gap-3 p-3 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer"
                >
                  <input
                    type="checkbox"
                    className="mt-0.5 w-4 h-4 rounded border-slate-700 bg-slate-900 text-rose-500 focus:ring-rose-500/40"
                    checked={values[box.id] ?? false}
                    aria-describedby={box.description ? descId : undefined}
                    onChange={(event) =>
                      setValues((current) => ({
                        ...current,
                        [box.id]: event.target.checked,
                      }))
                    }
                  />
                  <span className="min-w-0">
                    <span className="block text-sm font-medium text-slate-100">{box.label}</span>
                    {box.description ? (
                      <span id={descId} className="block text-xs text-slate-500 mt-0.5 leading-relaxed">
                        {box.description}
                      </span>
                    ) : null}
                  </span>
                </label>
              );
            })}
          </div>
        ) : null}
        <div className="mt-5 flex justify-end gap-2">
          <button type="button" className={quietButtonClass} onClick={onCancel}>
            {cancelLabel}
          </button>
          <button
            type="button"
            className={
              danger
                ? dangerButtonClass
                : 'inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg'
            }
            onClick={() => onConfirm(values)}
            autoFocus
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
};

type PendingConfirm = {
  options: ConfirmOptions;
  resolve: (value: ConfirmResult) => void;
};

/** Promise-based confirm dialog; render `dialog` once in the component tree. */
export function useConfirm() {
  const [pending, setPending] = useState<PendingConfirm | null>(null);

  const confirm = useCallback((options: ConfirmOptions | string) => {
    const normalized: ConfirmOptions =
      typeof options === 'string' ? { message: options } : options;
    return new Promise<ConfirmResult>((resolve) => {
      setPending({ options: normalized, resolve });
    });
  }, []);

  const settle = useCallback((value: ConfirmResult) => {
    setPending((current) => {
      current?.resolve(value);
      return null;
    });
  }, []);

  const dialog = pending ? (
    <ConfirmModal
      {...pending.options}
      onConfirm={(values) => settle(values)}
      onCancel={() => settle(false)}
    />
  ) : null;

  return { confirm, dialog };
}
