import React, { useState } from 'react';
import type { AppConfig, HostConfig, LoggingConfig, ProxyConfig, SecurityConfig } from '../../types';
import { api } from '../../api';
import {
  Checkbox,
  errorText,
  FieldRow,
  NumberInput,
  PageIntro,
  PasswordInput,
  SaveBar,
  Section,
  Select,
  TextInput,
  useDraftState,
} from './fields';

const LOCKED_HELP = 'Set by an environment variable and applied at startup.';

interface GeneralPageProps {
  config: AppConfig;
  onSave: (next: AppConfig) => Promise<AppConfig>;
  onDirtyChange: (dirty: boolean) => void;
  onNotify: (msg: string, isError?: boolean) => void;
  onApiKey: (apiKey: string) => void;
}

export const GeneralPage: React.FC<GeneralPageProps> = ({
  config,
  onSave,
  onDirtyChange,
  onNotify,
  onApiKey,
}) => {
  const { draft, setDraft, setBaseline, isDirty, saving, setSaving, commit } = useDraftState(
    config,
    onDirtyChange
  );
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [regenerating, setRegenerating] = useState(false);

  const patchHost = <K extends keyof HostConfig>(key: K, value: HostConfig[K]) => {
    setDraft((current) => ({ ...current, host: { ...current.host, [key]: value } }));
  };
  const patchSecurity = <K extends keyof SecurityConfig>(key: K, value: SecurityConfig[K]) => {
    setDraft((current) => ({ ...current, security: { ...current.security, [key]: value } }));
  };
  const patchProxy = <K extends keyof ProxyConfig>(key: K, value: ProxyConfig[K]) => {
    setDraft((current) => ({ ...current, proxy: { ...current.proxy, [key]: value } }));
  };
  const patchLogging = <K extends keyof LoggingConfig>(key: K, value: LoggingConfig[K]) => {
    setDraft((current) => ({ ...current, logging: { ...current.logging, [key]: value } }));
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const next: AppConfig = {
        ...draft,
        updates: { ...draft.updates, mechanism: 'docker' },
      };
      const saved = await onSave(next);
      commit(saved);
    } catch {
      // Parent already reported the error.
    } finally {
      setSaving(false);
    }
  };

  const handleRegenerate = async () => {
    if (!window.confirm('Regenerate the API key? Clients using the current key will need the new value.')) {
      return;
    }
    setRegenerating(true);
    try {
      const res = await api.regenerateApiKey();
      setDraft((current) => ({
        ...current,
        security: { ...current.security, apiKey: res.apiKey },
      }));
      setBaseline((current) => ({
        ...current,
        security: { ...current.security, apiKey: res.apiKey },
      }));
      onApiKey(res.apiKey);
      onNotify('API key regenerated. Copy it now.');
    } catch (err) {
      onNotify(errorText(err, 'Failed to regenerate API key'), true);
    } finally {
      setRegenerating(false);
    }
  };

  return (
    <div>
      <PageIntro title="General" caption="Host, security, proxy, logging, updates, and backup." />

      {draft.host.pendingRestart ? (
        <p className="text-xs text-amber-200/90 bg-amber-500/10 border border-amber-500/20 rounded-lg px-3 py-2 mb-4">
          Bind address, port, or URL base changes apply the next time Replacearr starts.
        </p>
      ) : null}

      <Section title="Host">
        <FieldRow
          label="Bind Address"
          helpText={draft.host.bindAddressEnvLocked ? LOCKED_HELP : undefined}
        >
          <TextInput
            value={draft.host.bindAddress}
            disabled={draft.host.bindAddressEnvLocked}
            onChange={(value) => patchHost('bindAddress', value)}
          />
        </FieldRow>
        <FieldRow label="Port" helpText={draft.host.portEnvLocked ? LOCKED_HELP : undefined}>
          <NumberInput
            value={draft.host.port}
            disabled={draft.host.portEnvLocked}
            onChange={(value) => patchHost('port', value)}
          />
        </FieldRow>
        <FieldRow label="URL Base" helpText={draft.host.urlBaseEnvLocked ? LOCKED_HELP : undefined}>
          <TextInput
            value={draft.host.urlBase}
            disabled={draft.host.urlBaseEnvLocked}
            onChange={(value) => patchHost('urlBase', value)}
          />
        </FieldRow>
        <FieldRow label="Instance Name">
          <TextInput
            value={draft.host.instanceName}
            onChange={(value) => patchHost('instanceName', value)}
          />
        </FieldRow>
        <FieldRow label="Application URL">
          <TextInput
            value={draft.host.applicationUrl}
            onChange={(value) => patchHost('applicationUrl', value)}
          />
        </FieldRow>
      </Section>

      <Section title="Security">
        <FieldRow
          label="Authentication Method"
          helpText="Sign-in is the Users page. There is no shared single password."
        >
          <Select
            value="forms"
            disabled
            onChange={() => undefined}
            options={[{ value: 'forms', label: 'Forms' }]}
          />
        </FieldRow>
        <FieldRow label="Authentication Required">
          <Select
            value={draft.security.authenticationRequired}
            onChange={(value) => patchSecurity('authenticationRequired', value)}
            options={[
              { value: 'enabled', label: 'Enabled' },
              { value: 'disabledForLocalAddresses', label: 'Disabled for Local Addresses' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Certificate Validation">
          <Select
            value={draft.security.certificateValidation}
            onChange={(value) => patchSecurity('certificateValidation', value)}
            options={[
              { value: 'enabled', label: 'Enabled' },
              { value: 'disabledForLocalAddresses', label: 'Disabled for Local Addresses' },
              { value: 'disabled', label: 'Disabled' },
            ]}
          />
        </FieldRow>
        <FieldRow label="API Key">
          <div className="space-y-2">
            <PasswordInput value={draft.security.apiKey} readOnly onChange={() => undefined} />
            <button
              type="button"
              onClick={() => void handleRegenerate()}
              disabled={regenerating}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-300 disabled:opacity-40"
            >
              {regenerating ? 'Regenerating...' : 'Regenerate'}
            </button>
          </div>
        </FieldRow>
      </Section>

      {showAdvanced ? (
        <Section
          title="SSL"
          caption="Replacearr does not terminate TLS in-process. Use a reverse proxy in front of the bind address."
        >
          <FieldRow label="Enable SSL">
            <Checkbox checked={draft.host.enableSsl} onChange={(value) => patchHost('enableSsl', value)} />
          </FieldRow>
          <FieldRow label="SSL Port">
            <NumberInput value={draft.host.sslPort} onChange={(value) => patchHost('sslPort', value)} />
          </FieldRow>
          <FieldRow label="SSL Certificate Path">
            <TextInput value={draft.host.sslCertPath} onChange={(value) => patchHost('sslCertPath', value)} />
          </FieldRow>
          <FieldRow label="SSL Key Path">
            <TextInput value={draft.host.sslKeyPath} onChange={(value) => patchHost('sslKeyPath', value)} />
          </FieldRow>
          <FieldRow label="SSL Certificate Password">
            <PasswordInput
              value={draft.host.sslCertPassword}
              onChange={(value) => patchHost('sslCertPassword', value)}
            />
          </FieldRow>
        </Section>
      ) : null}

      {showAdvanced ? (
        <Section title="Proxy">
          <FieldRow label="Use Proxy">
            <Checkbox checked={draft.proxy.enabled} onChange={(value) => patchProxy('enabled', value)} />
          </FieldRow>
          <FieldRow label="Proxy Type">
            <Select
              value={draft.proxy.type}
              onChange={(value) => patchProxy('type', value)}
              options={[
                { value: 'http', label: 'HTTP' },
                { value: 'socks4', label: 'SOCKS4' },
                { value: 'socks5', label: 'SOCKS5' },
              ]}
            />
          </FieldRow>
          <FieldRow label="Hostname">
            <TextInput value={draft.proxy.hostname} onChange={(value) => patchProxy('hostname', value)} />
          </FieldRow>
          <FieldRow label="Port">
            <NumberInput value={draft.proxy.port} onChange={(value) => patchProxy('port', value)} />
          </FieldRow>
          <FieldRow label="Username">
            <TextInput value={draft.proxy.username} onChange={(value) => patchProxy('username', value)} />
          </FieldRow>
          <FieldRow label="Password">
            <PasswordInput value={draft.proxy.password} onChange={(value) => patchProxy('password', value)} />
          </FieldRow>
          <FieldRow label="Bypass Filter">
            <TextInput value={draft.proxy.bypassFilter} onChange={(value) => patchProxy('bypassFilter', value)} />
          </FieldRow>
          <FieldRow label="Bypass Local Addresses">
            <Checkbox
              checked={draft.proxy.bypassLocalAddresses}
              onChange={(value) => patchProxy('bypassLocalAddresses', value)}
            />
          </FieldRow>
        </Section>
      ) : null}

      <Section title="Logging">
        <FieldRow label="Log Level">
          <Select
            value={draft.logging.logLevel}
            onChange={(value) => patchLogging('logLevel', value)}
            options={[
              { value: 'info', label: 'Info' },
              { value: 'debug', label: 'Debug' },
              { value: 'trace', label: 'Trace' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Console Log Level">
          <Select
            value={draft.logging.consoleLogLevel}
            onChange={(value) => patchLogging('consoleLogLevel', value)}
            options={[
              { value: 'info', label: 'Info' },
              { value: 'debug', label: 'Debug' },
              { value: 'trace', label: 'Trace' },
            ]}
          />
        </FieldRow>
        <FieldRow label="Log Size Limit">
          <NumberInput
            value={draft.logging.logSizeLimit}
            onChange={(value) => patchLogging('logSizeLimit', value)}
          />
        </FieldRow>
        <FieldRow label="History Cleanup Days">
          <NumberInput
            value={draft.logging.historyCleanupDays}
            onChange={(value) => patchLogging('historyCleanupDays', value)}
          />
        </FieldRow>
        <FieldRow label="Log Indexer Response">
          <Checkbox
            checked={draft.logging.logIndexerResponse}
            onChange={(value) => patchLogging('logIndexerResponse', value)}
          />
        </FieldRow>
      </Section>

      {showAdvanced ? (
        <>
          <Section title="Analytics" caption="Replacearr does not send telemetry. This value is stored only.">
            <FieldRow label="Send Analytics">
              <Checkbox
                checked={draft.analytics.enabled}
                onChange={(value) =>
                  setDraft((current) => ({ ...current, analytics: { ...current.analytics, enabled: value } }))
                }
              />
            </FieldRow>
          </Section>

          <Section title="Updates">
            <FieldRow label="Branch">
              <TextInput
                value={draft.updates.branch}
                onChange={(value) =>
                  setDraft((current) => ({ ...current, updates: { ...current.updates, branch: value } }))
                }
              />
            </FieldRow>
            <FieldRow label="Update Automatically">
              <Checkbox
                checked={draft.updates.updateAutomatically}
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    updates: { ...current.updates, updateAutomatically: value },
                  }))
                }
              />
            </FieldRow>
            <FieldRow label="Mechanism">
              <TextInput value="docker" disabled onChange={() => undefined} />
            </FieldRow>
            <FieldRow label="Update Script Path">
              <TextInput
                value={draft.updates.updateScriptPath}
                onChange={(value) =>
                  setDraft((current) => ({
                    ...current,
                    updates: { ...current.updates, updateScriptPath: value },
                  }))
                }
              />
            </FieldRow>
          </Section>

          <Section title="Backup" caption="Stored only. Replacearr does not run a backup job yet.">
            <FieldRow label="Folder">
              <TextInput
                value={draft.backup.folder}
                onChange={(value) =>
                  setDraft((current) => ({ ...current, backup: { ...current.backup, folder: value } }))
                }
              />
            </FieldRow>
            <FieldRow label="Interval">
              <NumberInput
                value={draft.backup.interval}
                onChange={(value) =>
                  setDraft((current) => ({ ...current, backup: { ...current.backup, interval: value } }))
                }
              />
            </FieldRow>
            <FieldRow label="Retention">
              <NumberInput
                value={draft.backup.retention}
                onChange={(value) =>
                  setDraft((current) => ({ ...current, backup: { ...current.backup, retention: value } }))
                }
              />
            </FieldRow>
          </Section>
        </>
      ) : null}

      <SaveBar
        disabled={!isDirty}
        saving={saving}
        onSave={() => void handleSave()}
        showAdvanced={showAdvanced}
        onToggleAdvanced={() => setShowAdvanced((current) => !current)}
      />
    </div>
  );
};
