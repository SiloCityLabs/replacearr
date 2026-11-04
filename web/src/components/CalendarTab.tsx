import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { CalendarDays, ChevronLeft, ChevronRight } from 'lucide-react';
import { api } from '../api';
import type { AppConfig, CalendarEvent } from '../types';
import type { NavView } from '../nav';

const STATUS_FILTERS = ['downloaded', 'missing', 'queued', 'unreleased'] as const;
const MEDIA_FILTERS = [
  { id: 'tv', label: 'TV' },
  { id: 'movies', label: 'Movies' },
  { id: 'music', label: 'Music' },
  { id: 'books', label: 'Books' },
] as const;

const STATUS_COLORS: Record<string, string> = {
  downloaded: 'bg-emerald-500/25 border-emerald-500/40 text-emerald-200',
  missing: 'bg-rose-500/25 border-rose-500/40 text-rose-200',
  queued: 'bg-sky-500/25 border-sky-500/40 text-sky-200',
  unreleased: 'bg-slate-500/25 border-slate-500/40 text-slate-300',
};

export interface CalendarTabProps {
  appUi?: AppConfig['ui'] | null;
  onNavigate: (view: NavView) => void;
  onNotify: (msg: string, isError?: boolean) => void;
}

function startOfMonth(d: Date): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1));
}

function endOfMonth(d: Date): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 0, 23, 59, 59));
}

function addMonths(d: Date, n: number): Date {
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + n, 1));
}

export const CalendarTab: React.FC<CalendarTabProps> = ({ appUi, onNavigate, onNotify }) => {
  const firstDay = appUi?.firstDayOfWeek ?? 0;
  const [cursor, setCursor] = useState(() => startOfMonth(new Date()));
  const [statuses, setStatuses] = useState<Set<string>>(new Set(STATUS_FILTERS));
  const [mediaTypes, setMediaTypes] = useState<Set<string>>(new Set(MEDIA_FILTERS.map((m) => m.id)));
  const [events, setEvents] = useState<CalendarEvent[]>([]);
  const [loading, setLoading] = useState(false);

  const range = useMemo(() => {
    const start = startOfMonth(cursor);
    const end = endOfMonth(cursor);
    return { start: start.toISOString(), end: end.toISOString() };
  }, [cursor]);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.getCalendar({
        start: range.start,
        end: range.end,
        mediaTypes: [...mediaTypes].join(','),
        statuses: [...statuses].join(','),
      });
      setEvents(data);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load calendar', true);
    } finally {
      setLoading(false);
    }
  }, [range, mediaTypes, statuses, onNotify]);

  useEffect(() => {
    void load();
  }, [load]);

  const days = useMemo(() => {
    const start = startOfMonth(cursor);
    const end = endOfMonth(cursor);
    const startWeekday = (start.getUTCDay() - firstDay + 7) % 7;
    const cells: Array<{ date: Date | null; key: string }> = [];
    for (let i = 0; i < startWeekday; i++) {
      cells.push({ date: null, key: `pad-${i}` });
    }
    for (let day = 1; day <= end.getUTCDate(); day++) {
      const d = new Date(Date.UTC(start.getUTCFullYear(), start.getUTCMonth(), day));
      cells.push({ date: d, key: d.toISOString() });
    }
    while (cells.length % 7 !== 0) {
      cells.push({ date: null, key: `trail-${cells.length}` });
    }
    return cells;
  }, [cursor, firstDay]);

  const eventsByDay = useMemo(() => {
    const map = new Map<string, CalendarEvent[]>();
    for (const ev of events) {
      const key = ev.date.slice(0, 10);
      const list = map.get(key) ?? [];
      list.push(ev);
      map.set(key, list);
    }
    return map;
  }, [events]);

  const weekdayLabels = useMemo(() => {
    const names = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
    const out: string[] = [];
    for (let i = 0; i < 7; i++) out.push(names[(firstDay + i) % 7]);
    return out;
  }, [firstDay]);

  const toggleSet = (set: Set<string>, id: string, setter: (s: Set<string>) => void) => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    setter(next);
  };

  const monthLabel = cursor.toLocaleString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' });

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-white inline-flex items-center gap-2">
            <CalendarDays className="w-5 h-5 text-cyan-300" />
            Calendar
          </h2>
          <p className="text-xs text-slate-400 mt-1">Downloaded, missing, queued, and unreleased across libraries.</p>
        </div>
        <div className="inline-flex items-center gap-2">
          <button
            type="button"
            onClick={() => setCursor((c) => addMonths(c, -1))}
            className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 hover:bg-slate-800"
          >
            <ChevronLeft className="w-4 h-4" />
          </button>
          <span className="text-sm font-semibold text-white min-w-[9rem] text-center">{monthLabel}</span>
          <button
            type="button"
            onClick={() => setCursor((c) => addMonths(c, 1))}
            className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 hover:bg-slate-800"
          >
            <ChevronRight className="w-4 h-4" />
          </button>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {STATUS_FILTERS.map((s) => (
          <button
            key={s}
            type="button"
            onClick={() => toggleSet(statuses, s, setStatuses)}
            className={`px-2.5 py-1 rounded-lg text-[11px] font-medium border capitalize ${
              statuses.has(s) ? STATUS_COLORS[s] : 'bg-slate-950 border-slate-800 text-slate-500'
            }`}
          >
            {s}
          </button>
        ))}
        <span className="w-px bg-slate-800 mx-1" />
        {MEDIA_FILTERS.map((m) => (
          <button
            key={m.id}
            type="button"
            onClick={() => toggleSet(mediaTypes, m.id, setMediaTypes)}
            className={`px-2.5 py-1 rounded-lg text-[11px] font-medium border ${
              mediaTypes.has(m.id)
                ? 'bg-indigo-500/20 border-indigo-500/40 text-indigo-200'
                : 'bg-slate-950 border-slate-800 text-slate-500'
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <div className="grid grid-cols-7 border-b border-slate-800">
          {weekdayLabels.map((d) => (
            <div key={d} className="px-2 py-2 text-[10px] uppercase tracking-wide text-slate-500 text-center">
              {d}
            </div>
          ))}
        </div>
        {loading ? (
          <p className="p-6 text-sm text-slate-400">Loading…</p>
        ) : (
          <div className="grid grid-cols-7">
            {days.map((cell) => {
              if (!cell.date) {
                return <div key={cell.key} className="min-h-[5.5rem] border-t border-r border-slate-800/60 bg-slate-950/40" />;
              }
              const key = cell.date.toISOString().slice(0, 10);
              const dayEvents = eventsByDay.get(key) ?? [];
              return (
                <div key={cell.key} className="min-h-[5.5rem] border-t border-r border-slate-800/60 p-1.5">
                  <div className="text-[10px] text-slate-500 mb-1">{cell.date.getUTCDate()}</div>
                  <div className="space-y-0.5">
                    {dayEvents.slice(0, 3).map((ev) => (
                      <button
                        key={ev.id}
                        type="button"
                        title={ev.title}
                        onClick={() => {
                          if (ev.mediaType === 'tv') onNavigate('series.library');
                          else if (ev.mediaType === 'movies' || ev.mediaType === 'movie') onNavigate('movies.library');
                          else if (ev.mediaType === 'music') onNavigate('music.library');
                          else onNavigate('books.library');
                        }}
                        className={`w-full text-left px-1.5 py-0.5 rounded border text-[10px] truncate ${
                          STATUS_COLORS[ev.status] ?? STATUS_COLORS.unreleased
                        }`}
                      >
                        {ev.title}
                      </button>
                    ))}
                    {dayEvents.length > 3 ? (
                      <div className="text-[10px] text-slate-500 px-1">+{dayEvents.length - 3} more</div>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
