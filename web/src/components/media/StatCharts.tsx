import React from 'react';

export interface ChartSlice {
  name: string;
  count: number;
}

const CHART_COLORS = [
  '#818cf8',
  '#34d399',
  '#fbbf24',
  '#f87171',
  '#a78bfa',
  '#38bdf8',
  '#fb923c',
  '#e879f9',
  '#2dd4bf',
  '#94a3b8',
];

function colorAt(i: number): string {
  return CHART_COLORS[i % CHART_COLORS.length];
}

function titleCase(name: string): string {
  if (!name) return name;
  return name.replace(/(^|[\s_-])([a-z])/g, (_, p1: string, p2: string) => `${p1}${p2.toUpperCase()}`);
}

export interface DonutChartProps {
  title: string;
  data: ChartSlice[];
  emptyLabel?: string;
}

export const DonutChart: React.FC<DonutChartProps> = ({ title, data, emptyLabel = 'No data' }) => {
  const slices = data.filter((d) => d.count > 0);
  const total = slices.reduce((sum, d) => sum + d.count, 0);
  const size = 160;
  const stroke = 28;
  const radius = (size - stroke) / 2;
  const circumference = 2 * Math.PI * radius;

  let offset = 0;
  const arcs = slices.map((slice, i) => {
    const len = total > 0 ? (slice.count / total) * circumference : 0;
    const arc = {
      ...slice,
      color: colorAt(i),
      dasharray: `${len} ${circumference - len}`,
      dashoffset: -offset,
    };
    offset += len;
    return arc;
  });

  return (
    <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 h-full">
      <h3 className="text-sm font-semibold text-white mb-3">{title}</h3>
      {total === 0 ? (
        <p className="text-xs text-slate-500 py-10 text-center">{emptyLabel}</p>
      ) : (
        <div className="flex flex-col sm:flex-row items-center gap-4">
          <div className="relative shrink-0" style={{ width: size, height: size }}>
            <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="-rotate-90">
              <circle
                cx={size / 2}
                cy={size / 2}
                r={radius}
                fill="none"
                stroke="#1e293b"
                strokeWidth={stroke}
              />
              {arcs.map((arc) => (
                <circle
                  key={arc.name}
                  cx={size / 2}
                  cy={size / 2}
                  r={radius}
                  fill="none"
                  stroke={arc.color}
                  strokeWidth={stroke}
                  strokeDasharray={arc.dasharray}
                  strokeDashoffset={arc.dashoffset}
                  strokeLinecap="butt"
                />
              ))}
            </svg>
            <div className="absolute inset-0 flex flex-col items-center justify-center pointer-events-none">
              <div className="text-xl font-bold text-white tabular-nums">{total}</div>
              <div className="text-[10px] uppercase tracking-wide text-slate-500">total</div>
            </div>
          </div>
          <ul className="flex-1 space-y-1.5 w-full min-w-0">
            {arcs.map((arc) => (
              <li key={arc.name} className="flex items-center gap-2 text-xs">
                <span className="w-2.5 h-2.5 rounded-sm shrink-0" style={{ background: arc.color }} />
                <span className="text-slate-300 truncate flex-1">{titleCase(arc.name)}</span>
                <span className="text-slate-400 font-mono tabular-nums">
                  {arc.count}
                  <span className="text-slate-600 ml-1">
                    ({Math.round((arc.count / total) * 100)}%)
                  </span>
                </span>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};

export interface BarChartProps {
  title: string;
  data: ChartSlice[];
  emptyLabel?: string;
  valueFormatter?: (n: number) => string;
}

export const BarChart: React.FC<BarChartProps> = ({
  title,
  data,
  emptyLabel = 'No data',
  valueFormatter = (n) => String(n),
}) => {
  const rows = data.filter((d) => d.count > 0);
  const max = rows.reduce((m, d) => Math.max(m, d.count), 0);

  return (
    <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-4 h-full">
      <h3 className="text-sm font-semibold text-white mb-3">{title}</h3>
      {rows.length === 0 ? (
        <p className="text-xs text-slate-500 py-10 text-center">{emptyLabel}</p>
      ) : (
        <div className="space-y-2.5">
          {rows.map((row, i) => {
            const pct = max > 0 ? (row.count / max) * 100 : 0;
            return (
              <div key={row.name} className="space-y-1">
                <div className="flex items-center justify-between gap-2 text-xs">
                  <span className="text-slate-300 truncate">{titleCase(row.name)}</span>
                  <span className="text-slate-400 font-mono tabular-nums shrink-0">
                    {valueFormatter(row.count)}
                  </span>
                </div>
                <div className="h-2 rounded-full bg-slate-800 overflow-hidden">
                  <div
                    className="h-full rounded-full transition-[width] duration-500"
                    style={{ width: `${pct}%`, background: colorAt(i) }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
