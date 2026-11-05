import React, { useEffect, useState } from 'react';
import {
  Puzzle,
  RefreshCw,
  Settings,
  Power,
  Code2,
  CheckCircle2,
  Terminal,
  X,
  Copy,
  Check,
  Key,
  Link2,
  Tv,
  Film,
  Radio,
  ChevronDown,
  FlaskConical,
  Download,
  Trash2,
  Store,
  ExternalLink,
} from 'lucide-react';
import type { CatalogPlugin, DiscoveredPlugin } from '../types';
import { api } from '../api';
import { isBrowserNotificationPlugin, notifyBrowser, requestPermission } from '../notifications';
import {
  type ConnectionTestResult,
  PasswordInput,
  SecretDisplay,
  testButtonClass,
  TestResultMessage,
} from './settings/fields';
import { useConfirm } from './modals/ConfirmModal';

interface PluginsTabProps {
  plugins: DiscoveredPlugin[];
  masterApiKey?: string;
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

const COMPAT_BASE: Record<string, { path: string; label: string; icon: typeof Tv }> = {
  'builtin-compat-sonarr': { path: '/sonarr', label: 'Sonarr v3', icon: Tv },
  'builtin-compat-radarr': { path: '/radarr', label: 'Radarr v3', icon: Film },
  'builtin-compat-prowlarr': { path: '/prowlarr', label: 'Prowlarr v1', icon: Radio },
};

const SDK_EXAMPLE_SNIPPET = `package main

import (
	"context"
	"github.com/replacearr/replacearr/pkg/pluginsdk"
)

type CustomTorznabPlugin struct {
	endpoint string
	apiKey   string
}

func (p *CustomTorznabPlugin) Manifest() pluginsdk.Manifest {
	return pluginsdk.Manifest{
		ID:          "custom-indexer",
		Name:        "Custom Go Indexer",
		Version:     "1.0.0",
		Author:      "Replacearr Community",
		Description: "High-speed Go JSON-RPC indexer plugin",
		Types:       []pluginsdk.PluginType{pluginsdk.PluginTypeIndexer},
		ConfigSchema: []pluginsdk.ConfigFieldSchema{
			{Name: "endpoint", Label: "API Endpoint", Type: "text", Default: "https://tracker.local/api", Required: true},
			{Name: "apiKey", Label: "API Key", Type: "password", Required: false},
		},
	}
}

func (p *CustomTorznabPlugin) Configure(settings map[string]any) error {
	p.endpoint, _ = settings["endpoint"].(string)
	p.apiKey, _ = settings["apiKey"].(string)
	return nil
}

func (p *CustomTorznabPlugin) Search(ctx context.Context, req pluginsdk.SearchRequest) ([]pluginsdk.Release, error) {
	return []pluginsdk.Release{
		{
			Guid:        "custom-1",
			Title:       req.Query + ".2026.2160p.WEB-DL.DDP5.1.H.265-REPLACEARR",
			Protocol:    "torrent",
			Size:        4294967296,
			Seeders:     140,
			Peers:       155,
			DownloadURL: "magnet:?xt=urn:btih:replacearr",
			Quality:     "WEBDL-2160p",
		},
	}, nil
}

func main() {
	pluginsdk.Serve(&CustomTorznabPlugin{})
}`;

export const PluginsTab: React.FC<PluginsTabProps> = ({
  plugins,
  masterApiKey = '',
  onRefresh,
  onNotify,
}) => {
  const { confirm, dialog } = useConfirm();
  const [reloading, setReloading] = useState(false);
  const [editingPlugin, setEditingPlugin] = useState<DiscoveredPlugin | null>(null);
  const [formSettings, setFormSettings] = useState<Record<string, unknown>>({});
  const [formEnabled, setFormEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null);
  const [copiedSnippet, setCopiedSnippet] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [sdkExpanded, setSdkExpanded] = useState(false);
  const [catalog, setCatalog] = useState<CatalogPlugin[]>([]);
  const [catalogUrl, setCatalogUrl] = useState('');
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [catalogBusyId, setCatalogBusyId] = useState<string | null>(null);
  const [githubURL, setGithubURL] = useState('');
  const [githubPreview, setGithubPreview] = useState<{
    name: string;
    id: string;
    version: string;
    author: string;
    description: string;
    hasPlatformAsset: boolean;
    repoUrl: string;
  } | null>(null);
  const [githubBusy, setGithubBusy] = useState(false);

  const isNotificationPlugin = (p: DiscoveredPlugin | null) =>
    Boolean(p?.manifest.types?.includes('notification'));

  const loadCatalog = async (refresh = false) => {
    setCatalogLoading(true);
    try {
      const res = refresh ? await api.refreshPluginCatalog() : await api.getPluginCatalog();
      setCatalog(res.plugins || []);
      setCatalogUrl(res.catalogUrl || '');
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to load plugin marketplace', true);
    } finally {
      setCatalogLoading(false);
    }
  };

  useEffect(() => {
    void loadCatalog(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- load once on mount
  }, []);

  const handleInstallCatalog = async (entry: CatalogPlugin) => {
    setCatalogBusyId(entry.id);
    try {
      if (entry.source === 'github' && entry.repoUrl) {
        await api.installCatalogPluginFromURL(entry.repoUrl);
      } else {
        await api.installCatalogPlugin(entry.id);
      }
      onNotify(`Installed "${entry.name}" v${entry.version}`);
      await onRefresh();
      await loadCatalog(true);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Install failed', true);
    } finally {
      setCatalogBusyId(null);
    }
  };

  const handleUninstallCatalog = async (entry: CatalogPlugin) => {
    const ok = await confirm({
      title: 'Uninstall plugin',
      message: `Uninstall plugin "${entry.name}"?`,
      confirmLabel: 'Uninstall',
    });
    if (!ok) return;
    setCatalogBusyId(entry.id);
    try {
      await api.uninstallCatalogPlugin(entry.id);
      onNotify(`Uninstalled "${entry.name}"`);
      await onRefresh();
      await loadCatalog(true);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Uninstall failed', true);
    } finally {
      setCatalogBusyId(null);
    }
  };

  const handleResolveGitHub = async () => {
    const url = githubURL.trim();
    if (!url) return;
    setGithubBusy(true);
    setGithubPreview(null);
    try {
      const res = await api.resolveCatalogPluginURL(url);
      setGithubPreview({
        name: res.plugin.name,
        id: res.plugin.id,
        version: res.plugin.version,
        author: res.plugin.author,
        description: res.plugin.description,
        hasPlatformAsset: res.hasPlatformAsset,
        repoUrl: res.repoUrl,
      });
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Could not resolve GitHub plugin', true);
    } finally {
      setGithubBusy(false);
    }
  };

  const handleInstallGitHub = async () => {
    const url = githubURL.trim();
    if (!url) return;
    setGithubBusy(true);
    try {
      const res = await api.installCatalogPluginFromURL(url);
      onNotify(`Installed "${res.id}" from GitHub`);
      setGithubURL('');
      setGithubPreview(null);
      await onRefresh();
      await loadCatalog(true);
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'GitHub install failed', true);
    } finally {
      setGithubBusy(false);
    }
  };

  const origin =
    typeof window !== 'undefined' && window.location.origin
      ? window.location.origin
      : 'http://localhost:8989';

  const copy = (value: string, id: string) => {
    void navigator.clipboard.writeText(value);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const handleReload = async () => {
    setReloading(true);
    try {
      await api.reloadPlugins();
      await onRefresh();
      onNotify('Re-scanned plugin directory and refreshed Go JSON-RPC plugins');
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to reload plugins', true);
    } finally {
      setReloading(false);
    }
  };

  const handleTogglePlugin = async (p: DiscoveredPlugin) => {
    try {
      const enabling = !p.enabled;
      if (enabling && isBrowserNotificationPlugin(p.manifest.id)) {
        const perm = await requestPermission();
        if (perm !== 'granted') {
          onNotify('Browser notification permission was not granted', true);
          return;
        }
      }
      await api.updatePlugin(p.manifest.id, {
        enabled: enabling,
        settings: p.settings || {},
      });
      onNotify(`${enabling ? 'Enabled' : 'Disabled'} plugin "${p.manifest.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to update plugin', true);
    }
  };

  const openConfigureModal = (p: DiscoveredPlugin) => {
    const initial: Record<string, unknown> = {};
    for (const field of p.manifest.configSchema || []) {
      initial[field.name] =
        p.settings?.[field.name] !== undefined
          ? p.settings[field.name]
          : field.default ?? '';
    }
    setFormSettings({ ...initial, ...(p.settings || {}) });
    setFormEnabled(p.enabled);
    setTestResult(null);
    setEditingPlugin(p);
  };

  const handleTestNotification = async () => {
    if (!editingPlugin || !isNotificationPlugin(editingPlugin)) return;
    setTestResult(null);
    setTesting(true);
    try {
      if (isBrowserNotificationPlugin(editingPlugin.manifest.id)) {
        const perm = await requestPermission();
        if (perm !== 'granted') {
          const message = 'Browser notification permission was not granted';
          setTestResult({ ok: false, message });
          onNotify(message, true);
          return;
        }
      }
      const result = await api.testNotificationPlugin(editingPlugin.manifest.id, formSettings);
      setTestResult({
        ok: result.ok,
        message: `${result.ok ? 'OK' : 'Failed'} — ${result.message} (${result.latencyMs} ms)`,
      });
      if (result.ok) {
        // When the browser channel is disabled, the SSE stream is not subscribed —
        // show the OS notification locally so Test still proves permission + rendering.
        if (
          isBrowserNotificationPlugin(editingPlugin.manifest.id) &&
          !editingPlugin.enabled
        ) {
          notifyBrowser('Test notification', 'Replacearr test notification');
        }
        onNotify(`Test notification sent via "${editingPlugin.manifest.name}"`);
      } else {
        onNotify(result.message, true);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Notification test failed';
      setTestResult({ ok: false, message });
      onNotify(message, true);
    } finally {
      setTesting(false);
    }
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingPlugin) return;
    setSaving(true);
    try {
      if (formEnabled && isBrowserNotificationPlugin(editingPlugin.manifest.id)) {
        const perm = await requestPermission();
        if (perm !== 'granted') {
          onNotify('Browser notification permission was not granted', true);
          setSaving(false);
          return;
        }
      }
      await api.updatePlugin(editingPlugin.manifest.id, {
        enabled: formEnabled,
        settings: formSettings,
      });
      onNotify(`Updated configuration & hot-reloaded "${editingPlugin.manifest.name}"`);
      setEditingPlugin(null);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to save plugin settings', true);
    } finally {
      setSaving(false);
    }
  };

  const copySnippet = () => {
    void navigator.clipboard.writeText(SDK_EXAMPLE_SNIPPET);
    setCopiedSnippet(true);
    setTimeout(() => setCopiedSnippet(false), 2000);
  };

  const sortedPlugins = [...plugins].sort((a, b) =>
    a.manifest.name.localeCompare(b.manifest.name, undefined, { sensitivity: 'base' })
  );

  const renderPluginCard = (p: DiscoveredPlugin) => {
    const compatMeta = COMPAT_BASE[p.manifest.id];
    const CompatIcon = compatMeta?.icon;
    const baseUrl = compatMeta ? `${origin}${compatMeta.path}` : '';

    return (
      <div
        key={p.manifest.id}
        className="bg-slate-900/90 border border-slate-800 hover:border-purple-500/40 rounded-xl p-5 flex flex-col justify-between space-y-4 transition"
      >
        <div>
          <div className="flex items-start justify-between gap-3">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                {CompatIcon ? (
                  <CompatIcon className="w-4 h-4 text-indigo-400 shrink-0" />
                ) : null}
                <h3 className="text-base font-bold text-white">{p.manifest.name}</h3>
                <span className="text-[11px] font-mono px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-purple-300">
                  v{p.manifest.version}
                </span>
                {p.enabled ? (
                  <span className="inline-flex items-center gap-1 text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                    <CheckCircle2 className="w-3 h-3" /> {p.status || 'active'}
                  </span>
                ) : (
                  <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-slate-800 text-slate-400">
                    disabled
                  </span>
                )}
              </div>
              <p className="text-xs text-slate-400 mt-1">{p.manifest.description}</p>
            </div>

            <button
              type="button"
              onClick={() => void handleTogglePlugin(p)}
              title={p.enabled ? 'Disable Plugin' : 'Enable Plugin'}
              className={`p-2 rounded-lg border transition ${
                p.enabled
                  ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-400 hover:bg-emerald-500/20'
                  : 'bg-slate-950 border-slate-800 text-slate-500 hover:text-slate-300'
              }`}
            >
              <Power className="w-4 h-4" />
            </button>
          </div>

          <div className="mt-4 flex flex-wrap items-center gap-2">
            {(p.manifest.types || []).map((t) => (
              <span
                key={t}
                className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-purple-500/15 text-purple-300 border border-purple-500/30"
              >
                {t}
              </span>
            ))}
            <span className="text-[11px] font-mono text-slate-500 truncate">
              ID: <strong className="text-slate-300">{p.manifest.id}</strong> • Author:{' '}
              {p.manifest.author || 'Replacearr'}
            </span>
          </div>

          {compatMeta && (
            <div className="mt-4 p-3 rounded-lg bg-slate-950 border border-slate-800/80 space-y-2">
              <div className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">
                Client base URL
              </div>
              <div className="flex items-center justify-between gap-2">
                <code className="text-xs font-mono text-indigo-300 truncate min-w-0">
                  {baseUrl}
                </code>
                <button
                  type="button"
                  onClick={() => copy(baseUrl, p.manifest.id)}
                  className="p-1 text-slate-400 hover:text-white shrink-0"
                >
                  {copiedId === p.manifest.id ? (
                    <Check className="w-3.5 h-3.5 text-emerald-400" />
                  ) : (
                    <Copy className="w-3.5 h-3.5" />
                  )}
                </button>
              </div>
              {!p.enabled && (
                <p className="text-[11px] text-amber-400/90">
                  Disabled — routes return HTTP 404 until re-enabled (no restart needed).
                </p>
              )}
            </div>
          )}

          {!compatMeta && p.manifest.configSchema && p.manifest.configSchema.length > 0 && (
            <div className="mt-4 p-3 rounded-lg bg-slate-950 border border-slate-800/80 space-y-1.5">
              <div className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">
                ConfigSchema Fields ({p.manifest.configSchema.length})
              </div>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                {p.manifest.configSchema.map((field) => {
                  const val =
                    p.settings?.[field.name] !== undefined
                      ? p.settings[field.name]
                      : field.default;
                  return (
                    <div
                      key={field.name}
                      className="text-xs flex items-center justify-between bg-slate-900/90 px-2.5 py-1.5 rounded border border-slate-800"
                    >
                      <span className="text-slate-400">{field.label || field.name}:</span>
                      <code className="font-mono text-purple-300 truncate max-w-[130px]">
                        {field.type === 'password'
                          ? '••••••••'
                          : String(val ?? '') || 'default'}
                      </code>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        <div className="pt-3 border-t border-slate-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-xs">
          <code className="font-mono text-[11px] text-slate-500 truncate min-w-0">
            {p.binaryPath || 'builtin://sdk'}
          </code>
          <button
            type="button"
            onClick={() => openConfigureModal(p)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium transition"
          >
            <Settings className="w-3.5 h-3.5 text-purple-400" />
            <span>Configure Settings</span>
          </button>
        </div>
      </div>
    );
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900/90 border border-slate-800 rounded-xl p-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-purple-500/15 text-purple-400 border border-purple-500/30">
            <Puzzle className="w-5 h-5" />
          </div>
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-lg font-bold text-white">Go Plugin System</h2>
              <span className="text-xs px-2 py-0.5 rounded-full bg-purple-500/15 text-purple-300 border border-purple-500/30 font-mono">
                stdin/stdout JSON-RPC
              </span>
            </div>
            <p className="text-xs text-slate-400">
              Built-in *arr shims, network proxies, and external Go plugins — toggle without restart
            </p>
          </div>
        </div>

        <button
          type="button"
          onClick={() => void handleReload()}
          disabled={reloading}
          className="inline-flex items-center gap-2 px-3.5 py-2 bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white text-xs font-semibold rounded-lg transition"
        >
          <RefreshCw className={`w-4 h-4 ${reloading ? 'animate-spin' : ''}`} />
          <span>{reloading ? 'Scanning...' : 'Re-Scan Plugin Directory'}</span>
        </button>
      </div>

      <div className="bg-gradient-to-r from-slate-900 via-indigo-950/40 to-slate-900 border border-slate-800 rounded-2xl p-5 space-y-4">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
          <div className="space-y-1">
            <div className="inline-flex items-center gap-2 text-xs font-medium text-indigo-300">
              <Key className="w-4 h-4 text-amber-400" />
              <span>*arr client connection</span>
            </div>
            <h3 className="text-base font-bold text-white">
              Connect Overseerr, Bazarr &amp; nzb360
            </h3>
            <p className="text-xs text-slate-400 max-w-2xl">
              Enable the Sonarr / Radarr / Prowlarr plugins below, then point external tools at the
              base URLs with your Master API Key (
              <code className="font-mono text-slate-200">X-Api-Key</code> or{' '}
              <code className="font-mono text-slate-200">?apikey=</code>).
            </p>
          </div>

          <div className="bg-slate-950 border border-slate-800 rounded-xl p-3.5 space-y-2 min-w-0 w-full lg:max-w-md">
            <div className="flex items-center justify-between gap-2">
              <div className="text-[10px] font-mono uppercase tracking-wider text-slate-400">
                System Master API Key
              </div>
              <button
                type="button"
                onClick={() => copy(masterApiKey || 'replacearr-master-api-key', 'master-key')}
                className="px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold inline-flex items-center gap-1.5 transition shrink-0"
              >
                {copiedId === 'master-key' ? (
                  <>
                    <Check className="w-3.5 h-3.5" />
                    <span>Copied</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3.5 h-3.5" />
                    <span>Copy Key</span>
                  </>
                )}
              </button>
            </div>
            <SecretDisplay
              value={masterApiKey || 'replacearr-master-api-key'}
              className="block text-sm font-mono font-bold text-emerald-400 select-all break-all"
            />
          </div>
        </div>

        <div className="flex items-start gap-2 text-xs text-slate-400">
          <Link2 className="w-4 h-4 text-indigo-400 shrink-0 mt-0.5" />
          <span>
            Example: Overseerr → Add Sonarr → Base URL{' '}
            <code className="font-mono text-indigo-300">/sonarr</code>, same host/port as Replacearr,
            paste the API key above.
          </span>
        </div>
      </div>

      <div>
        <h2 className="text-sm font-semibold uppercase tracking-wider text-slate-400 mb-3">
          Installed
        </h2>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {sortedPlugins.map(renderPluginCard)}
        </div>
      </div>

      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <Store className="w-4 h-4 text-indigo-400" />
            <h2 className="text-sm font-semibold uppercase tracking-wider text-slate-400">
              Marketplace
            </h2>
          </div>
          <button
            type="button"
            onClick={() => void loadCatalog(true)}
            disabled={catalogLoading}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-200 font-medium transition disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${catalogLoading ? 'animate-spin' : ''}`} />
            Refresh catalog
          </button>
        </div>
        {catalogUrl ? (
          <p className="text-[11px] font-mono text-slate-500 truncate">
            Registry: {catalogUrl}
          </p>
        ) : null}

        <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-xs font-semibold uppercase tracking-wider text-slate-400">
              Install from GitHub
            </h3>
            <span className="text-[11px] text-amber-400/90">
              Third-party repos are not reviewed by Replacearr — install at your own risk.
            </span>
          </div>
          <div className="flex flex-col sm:flex-row gap-2">
            <input
              type="text"
              value={githubURL}
              onChange={(e) => {
                setGithubURL(e.target.value);
                setGithubPreview(null);
              }}
              placeholder="owner/repo or https://github.com/owner/repo"
              className="flex-1 min-w-0 px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-sm text-slate-100 font-mono placeholder:text-slate-600 focus:outline-none focus:border-indigo-500"
            />
            <button
              type="button"
              disabled={githubBusy || !githubURL.trim()}
              onClick={() => void handleResolveGitHub()}
              className="inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-200 font-semibold transition disabled:opacity-40"
            >
              Preview
            </button>
            <button
              type="button"
              disabled={githubBusy || !githubURL.trim() || (githubPreview != null && !githubPreview.hasPlatformAsset)}
              onClick={() => void handleInstallGitHub()}
              className="inline-flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs text-white font-semibold transition disabled:opacity-40"
            >
              <Download className="w-3.5 h-3.5" />
              Install
            </button>
          </div>
          {githubPreview ? (
            <div className="text-xs text-slate-300 space-y-1">
              <p>
                <span className="font-semibold text-white">{githubPreview.name}</span>{' '}
                <span className="font-mono text-indigo-300">v{githubPreview.version}</span>{' '}
                <span className="text-slate-500">
                  {githubPreview.author} • {githubPreview.id}
                </span>
              </p>
              <p className="text-slate-400">{githubPreview.description}</p>
              <p className="font-mono text-[11px] text-slate-500 truncate">{githubPreview.repoUrl}</p>
              {!githubPreview.hasPlatformAsset ? (
                <p className="text-amber-400">No checksummed asset for this OS/arch.</p>
              ) : null}
            </div>
          ) : null}
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          {catalog.map((entry) => {
            const busy = catalogBusyId === entry.id;
            return (
              <div
                key={entry.id}
                className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-3"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-bold text-white">{entry.name}</h3>
                      <span className="text-[11px] font-mono px-2 py-0.5 rounded bg-slate-950 border border-slate-800 text-indigo-300">
                        v{entry.version}
                      </span>
                      {entry.installed ? (
                        <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                          installed{entry.installedVersion ? ` ${entry.installedVersion}` : ''}
                        </span>
                      ) : null}
                      {entry.updateAvailable ? (
                        <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-300 border border-amber-500/30">
                          update available
                        </span>
                      ) : null}
                      {entry.source === 'github' ? (
                        <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-rose-500/15 text-rose-300 border border-rose-500/30">
                          github
                        </span>
                      ) : null}
                    </div>
                    <p className="text-xs text-slate-400 mt-1">{entry.description}</p>
                    <div className="mt-2 flex flex-wrap gap-2">
                      {(entry.types || []).map((t) => (
                        <span
                          key={t}
                          className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30"
                        >
                          {t}
                        </span>
                      ))}
                      <span className="text-[11px] font-mono text-slate-500">
                        {entry.author} • {entry.id}
                      </span>
                    </div>
                    {entry.repoUrl ? (
                      <p className="mt-1 text-[11px] font-mono text-slate-500 truncate">
                        {entry.repoUrl}
                      </p>
                    ) : null}
                  </div>
                  {entry.homepage || entry.repoUrl ? (
                    <a
                      href={entry.homepage || entry.repoUrl}
                      target="_blank"
                      rel="noreferrer"
                      className="p-2 rounded-lg border border-slate-800 text-slate-400 hover:text-white shrink-0"
                      title="Open homepage"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </a>
                  ) : null}
                </div>
                <div className="flex flex-wrap gap-2">
                  {!entry.installed || entry.updateAvailable ? (
                    <button
                      type="button"
                      disabled={busy || !entry.hasPlatformAsset}
                      onClick={() => void handleInstallCatalog(entry)}
                      title={
                        entry.hasPlatformAsset
                          ? undefined
                          : 'No release binary for this platform yet'
                      }
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-xs text-white font-semibold transition disabled:opacity-40"
                    >
                      <Download className="w-3.5 h-3.5" />
                      {entry.installed ? 'Update' : 'Install'}
                    </button>
                  ) : null}
                  {entry.installed ? (
                    <button
                      type="button"
                      disabled={busy}
                      onClick={() => void handleUninstallCatalog(entry)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-rose-900/40 text-xs text-slate-200 font-semibold transition disabled:opacity-40"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                      Uninstall
                    </button>
                  ) : null}
                  {!entry.hasPlatformAsset ? (
                    <span className="text-[11px] text-amber-400/90 self-center">
                      No checksummed asset for this OS/arch yet — tag a marketplace release.
                    </span>
                  ) : null}
                </div>
              </div>
            );
          })}
          {!catalogLoading && catalog.length === 0 ? (
            <p className="text-sm text-slate-500 col-span-full">
              Marketplace registry is empty or unreachable.
            </p>
          ) : null}
        </div>
      </div>

      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <button
          type="button"
          onClick={() => setSdkExpanded((v) => !v)}
          aria-expanded={sdkExpanded}
          className="w-full p-4 flex items-center justify-between gap-3 bg-slate-950/50 hover:bg-slate-950/80 text-left transition"
        >
          <div className="flex items-center gap-2.5 min-w-0">
            <Code2 className="w-5 h-5 text-purple-400 shrink-0" />
            <div className="min-w-0">
              <h3 className="text-sm font-bold text-white">
                Write a Go Plugin in 1 File (`pkg/pluginsdk`)
              </h3>
              <p className="text-xs text-slate-400">
                Publish to{' '}
                <code className="font-mono text-indigo-300">replacearr-plugins</code> or drop a
                binary into{' '}
                <code className="font-mono text-slate-300">./config/plugins/</code> and click Re-Scan
              </p>
            </div>
          </div>
          <ChevronDown
            className={`w-4 h-4 text-slate-400 shrink-0 transition-transform ${
              sdkExpanded ? 'rotate-180' : ''
            }`}
          />
        </button>

        {sdkExpanded && (
          <>
            <div className="px-4 py-3 border-t border-slate-800 flex justify-end bg-slate-950/30">
              <button
                type="button"
                onClick={copySnippet}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-200 font-medium transition"
              >
                {copiedSnippet ? (
                  <>
                    <Check className="w-3.5 h-3.5 text-emerald-400" />
                    <span>Copied Go Code</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3.5 h-3.5 text-slate-400" />
                    <span>Copy SDK Template</span>
                  </>
                )}
              </button>
            </div>

            <div className="p-4 bg-slate-950 overflow-x-auto border-t border-slate-800">
              <pre className="text-xs font-mono text-slate-300 leading-relaxed">
                {SDK_EXAMPLE_SNIPPET}
              </pre>
            </div>

            <div className="px-4 py-3 bg-slate-900/90 border-t border-slate-800 flex items-center gap-2 text-xs text-slate-400 font-mono overflow-x-auto">
              <Terminal className="w-4 h-4 text-emerald-400 shrink-0" />
              <span className="whitespace-nowrap">
                go build -o ./config/plugins/replacearr-plugin-custom ./plugins/custom && curl -X POST
                http://localhost:8989/api/v1/plugins/reload
              </span>
            </div>
          </>
        )}
      </div>

      {editingPlugin && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-4 sm:p-5 border-b border-slate-800 flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="text-base font-bold text-white break-words">
                  Configure {editingPlugin.manifest.name}
                </h3>
                <p className="text-[11px] sm:text-xs text-slate-400 font-mono break-all">
                  PUT /api/v1/plugins/{editingPlugin.manifest.id}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setEditingPlugin(null)}
                className="shrink-0 p-2 rounded-lg bg-slate-800 text-slate-400 hover:text-white touch-manipulation"
                aria-label="Close"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={(e) => void handleSaveConfig(e)} className="p-4 sm:p-5 space-y-3">
              <label className="flex items-start justify-between gap-3 p-3 rounded-xl bg-slate-950 border border-slate-800 cursor-pointer touch-manipulation">
                <div className="min-w-0">
                  <div className="text-xs font-semibold text-white">Enable Plugin</div>
                  <div className="text-[11px] text-slate-400 mt-0.5">
                    {(editingPlugin.manifest.types || []).includes('compat')
                      ? 'When off, compatibility routes return HTTP 404 immediately'
                      : 'Include this plugin in federated search & automation hooks'}
                  </div>
                </div>
                <input
                  type="checkbox"
                  checked={formEnabled}
                  onChange={(e) => setFormEnabled(e.target.checked)}
                  className="mt-0.5 w-5 h-5 shrink-0 accent-purple-500 rounded"
                />
              </label>

              {COMPAT_BASE[editingPlugin.manifest.id] && (
                <div className="p-3 rounded-xl bg-slate-950 border border-slate-800 space-y-1">
                  <div className="text-[11px] text-slate-400">Base URL for clients</div>
                  <code className="text-xs font-mono text-indigo-300 break-all">
                    {origin}
                    {COMPAT_BASE[editingPlugin.manifest.id].path}
                  </code>
                </div>
              )}

              {(editingPlugin.manifest.inboundWebhooks || []).length > 0 && (
                <div className="p-3 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
                  <div className="text-[11px] font-semibold uppercase tracking-wider text-slate-400">
                    Inbound webhooks
                  </div>
                  {(editingPlugin.manifest.inboundWebhooks || []).map((wh, idx) => {
                    const paths = [
                      `/api/v1/webhooks/${editingPlugin.manifest.id}${wh.slug ? `/${wh.slug}` : ''}`,
                      ...(wh.pathAliases || []).map((a) => `/api/v1/webhooks/${a}`),
                    ];
                    return (
                      <div key={`${wh.secretSetting}-${idx}`} className="space-y-1">
                        {wh.description ? (
                          <p className="text-[11px] text-slate-400">{wh.description}</p>
                        ) : null}
                        {paths.map((path) => (
                          <code
                            key={path}
                            className="block text-xs font-mono text-indigo-300 break-all"
                          >
                            POST {origin}
                            {path}
                          </code>
                        ))}
                        <p className="text-[10px] text-slate-500">
                          Auth: header X-Replacearr-Webhook-Secret or ?secret= (setting{' '}
                          <span className="font-mono">{wh.secretSetting}</span>)
                        </p>
                      </div>
                    );
                  })}
                </div>
              )}

              {(editingPlugin.manifest.configSchema || []).map((field) =>
                field.type === 'checkbox' ? (
                  <label
                    key={field.name}
                    className="flex items-start justify-between gap-3 p-3 rounded-xl bg-slate-950 border border-slate-800 cursor-pointer touch-manipulation"
                  >
                    <div className="min-w-0">
                      <div className="text-xs font-semibold text-white">
                        {field.label || field.name}
                        {field.required ? <span className="text-rose-400"> *</span> : null}
                      </div>
                      {field.helpText ? (
                        <div className="text-[11px] text-slate-400 mt-0.5">{field.helpText}</div>
                      ) : null}
                    </div>
                    <input
                      type="checkbox"
                      checked={Boolean(formSettings[field.name])}
                      onChange={(e) =>
                        setFormSettings({ ...formSettings, [field.name]: e.target.checked })
                      }
                      className="mt-0.5 w-5 h-5 shrink-0 accent-purple-500 rounded"
                    />
                  </label>
                ) : (
                  <div key={field.name}>
                    <label
                      htmlFor={`plugin-field-${field.name}`}
                      className="block text-xs font-medium text-slate-300 mb-1"
                    >
                      {field.label || field.name}{' '}
                      {field.required ? <span className="text-rose-400">*</span> : null}
                    </label>
                    {field.type === 'password' ? (
                      <PasswordInput
                        id={`plugin-field-${field.name}`}
                        value={String(formSettings[field.name] ?? '')}
                        onChange={(value) =>
                          setFormSettings({ ...formSettings, [field.name]: value })
                        }
                        readOnly={Boolean(COMPAT_BASE[editingPlugin.manifest.id])}
                        className="w-full min-h-10 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                      />
                    ) : (
                      <input
                        id={`plugin-field-${field.name}`}
                        type={field.type === 'number' ? 'number' : 'text'}
                        value={String(formSettings[field.name] ?? '')}
                        onChange={(e) =>
                          setFormSettings({
                            ...formSettings,
                            [field.name]:
                              field.type === 'number' ? Number(e.target.value) : e.target.value,
                          })
                        }
                        readOnly={Boolean(COMPAT_BASE[editingPlugin.manifest.id])}
                        className="w-full min-h-10 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                      />
                    )}
                    {field.helpText ? (
                      <p className="text-[11px] text-slate-500 mt-1">{field.helpText}</p>
                    ) : null}
                  </div>
                )
              )}

              {isNotificationPlugin(editingPlugin) && testResult ? (
                <TestResultMessage result={testResult} />
              ) : null}

              <div className="pt-3 border-t border-slate-800 flex flex-col-reverse sm:flex-row sm:justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setEditingPlugin(null)}
                  className="w-full sm:w-auto px-4 py-2.5 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium touch-manipulation"
                >
                  Cancel
                </button>
                {isNotificationPlugin(editingPlugin) ? (
                  <button
                    type="button"
                    disabled={testing || saving}
                    onClick={() => void handleTestNotification()}
                    className={`w-full sm:w-auto justify-center px-4 py-2.5 transition disabled:opacity-50 touch-manipulation ${testButtonClass(testResult?.ok)}`}
                  >
                    <FlaskConical
                      className={`w-3.5 h-3.5 ${
                        testResult?.ok === true
                          ? 'text-emerald-300'
                          : testResult?.ok === false
                            ? 'text-rose-300'
                            : 'text-amber-400'
                      }`}
                    />
                    <span>{testing ? 'Testing…' : 'Test'}</span>
                  </button>
                ) : null}
                <button
                  type="submit"
                  disabled={saving || testing}
                  className="w-full sm:w-auto px-4 py-2.5 rounded-lg bg-purple-600 hover:bg-purple-500 text-white text-xs font-semibold transition disabled:opacity-50 touch-manipulation"
                >
                  {saving ? 'Hot-Reloading...' : 'Save & Hot-Reload'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
      {dialog}
    </div>
  );
};
