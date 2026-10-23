import type { AppConfig } from './types';

const UI_STORAGE_KEY = 'replacearr_ui';

const MONTHS_SHORT = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const MONTHS_FULL = [
  'January',
  'February',
  'March',
  'April',
  'May',
  'June',
  'July',
  'August',
  'September',
  'October',
  'November',
  'December',
];
const DAYS_FULL = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
const DAYS_SHORT = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

function resolvedTheme(theme: string): 'light' | 'dark' {
  if (theme === 'auto') {
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  }
  if (theme === 'light' || theme === 'dark') return theme;
  return 'dark';
}

export function applyUiPreferences(ui: AppConfig['ui']): void {
  try {
    localStorage.setItem(UI_STORAGE_KEY, JSON.stringify(ui));
  } catch {
    // Ignore storage errors.
  }
  document.documentElement.dataset.theme = resolvedTheme(ui.theme);
  document.documentElement.dataset.colorImpaired = ui.enableColorImpairedMode ? 'true' : 'false';
}

export function loadCachedUiPreferences(): void {
  try {
    const raw = localStorage.getItem(UI_STORAGE_KEY);
    if (!raw) return;
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object' || !('theme' in parsed)) return;
    const theme = (parsed as { theme: unknown }).theme;
    if (typeof theme !== 'string') return;
    applyUiPreferences(parsed as AppConfig['ui']);
  } catch {
    // Ignore invalid cache.
  }
}

function applyTokens(pattern: string, date: Date): string {
  const hours = date.getHours();
  const hour12 = hours % 12 || 12;
  return pattern.replace(/dddd|ddd|YYYY|MMMM|MMM|MM|DD|HH|mm|M|D|h|a/g, (token) => {
    switch (token) {
      case 'dddd':
        return DAYS_FULL[date.getDay()];
      case 'ddd':
        return DAYS_SHORT[date.getDay()];
      case 'YYYY':
        return String(date.getFullYear());
      case 'MMMM':
        return MONTHS_FULL[date.getMonth()];
      case 'MMM':
        return MONTHS_SHORT[date.getMonth()];
      case 'MM':
        return String(date.getMonth() + 1).padStart(2, '0');
      case 'DD':
        return String(date.getDate()).padStart(2, '0');
      case 'HH':
        return String(hours).padStart(2, '0');
      case 'mm':
        return String(date.getMinutes()).padStart(2, '0');
      case 'M':
        return String(date.getMonth() + 1);
      case 'D':
        return String(date.getDate());
      case 'h':
        return String(hour12);
      case 'a':
        return hours < 12 ? 'am' : 'pm';
      default:
        return token;
    }
  });
}

function calendarStart(date: Date): number {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
}

export function formatAppDate(
  iso: string,
  ui: AppConfig['ui'] | null | undefined,
  kind: 'short' | 'long' = 'short'
): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;

  const showRelative = ui?.showRelativeDates !== false;
  const timePattern = ui?.timeFormat || 'HH:mm';
  const datePattern =
    kind === 'long' ? ui?.longDateFormat || 'dddd, MMMM D YYYY' : ui?.shortDateFormat || 'MMM D YYYY';
  const time = applyTokens(timePattern, date);

  if (showRelative) {
    const dayDiff = Math.round((calendarStart(new Date()) - calendarStart(date)) / 86400000);
    if (dayDiff === 0) return `Today ${time}`;
    if (dayDiff === 1) return `Yesterday ${time}`;
  }

  return `${applyTokens(datePattern, date)} ${time}`;
}
