import React from 'react';
import type { OverviewResponse } from '../../types';
import { FieldRow, PageIntro, Section } from './fields';

interface SystemInfoPageProps {
  overview: OverviewResponse | null;
}

function compatEntries(
  value: OverviewResponse['system']['compatEndpoints'] | undefined
): Array<[string, string]> {
  if (!value) return [];
  if (Array.isArray(value)) {
    return value.map((endpoint) => [endpoint, endpoint]);
  }
  return Object.entries(value);
}

export const SystemInfoPage: React.FC<SystemInfoPageProps> = ({ overview }) => {
  const system = overview?.system;
  const endpoints = compatEntries(system?.compatEndpoints);

  return (
    <div>
      <PageIntro title="System" caption="Runtime status for this Replacearr process." />

      <Section title="Status" caption="Live process and database details.">
        <FieldRow label="Status">
          <span className="text-sm font-mono text-emerald-400">ONLINE</span>
        </FieldRow>
        <FieldRow label="Version" helpText="Application build version.">
          <span className="text-sm font-mono text-slate-200">
            {system?.version || '1.0.0'}
          </span>
        </FieldRow>
        <FieldRow label="Build" helpText="Compiled without CGO using modernc.org/sqlite.">
          <span className="text-sm font-mono text-slate-200">Zero-CGO</span>
        </FieldRow>
        <FieldRow label="Database" helpText="Journal mode used by the embedded SQLite store.">
          <span className="text-sm font-mono text-slate-200">
            {system?.sqliteWal === false ? 'SQLite' : 'SQLite WAL'}
          </span>
        </FieldRow>
        <FieldRow label="Uptime">
          <span className="text-sm font-mono text-slate-200">
            {system?.uptime || '—'}
          </span>
        </FieldRow>
        {system?.flareSolverrUrl ? (
          <FieldRow label="FlareSolverr" helpText="Active proxy host used for challenged indexers.">
            <span className="text-sm font-mono text-slate-200 break-all">
              {system.flareSolverrUrl}
            </span>
          </FieldRow>
        ) : null}
      </Section>

      {endpoints.length > 0 ? (
        <Section title="Compat endpoints" caption="Wire-compatible *arr API prefixes on this instance.">
          {endpoints.map(([name, path]) => (
            <FieldRow key={name} label={name}>
              <span className="text-sm font-mono text-slate-200 break-all">{path}</span>
            </FieldRow>
          ))}
        </Section>
      ) : null}
    </div>
  );
};
