import React, { useState } from 'react';
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
} from 'lucide-react';
import type { DiscoveredPlugin } from '../types';
import { api } from '../api';

interface PluginsTabProps {
  plugins: DiscoveredPlugin[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

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
  onRefresh,
  onNotify,
}) => {
  const [reloading, setReloading] = useState(false);
  const [editingPlugin, setEditingPlugin] = useState<DiscoveredPlugin | null>(null);
  const [formSettings, setFormSettings] = useState<Record<string, unknown>>({});
  const [formEnabled, setFormEnabled] = useState(true);
  const [saving, setSaving] = useState(false);
  const [copiedSnippet, setCopiedSnippet] = useState(false);

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
      await api.updatePlugin(p.manifest.id, {
        enabled: !p.enabled,
        settings: p.settings || {},
      });
      onNotify(
        `${!p.enabled ? 'Enabled' : 'Disabled'} plugin "${p.manifest.name}"`
      );
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
    setEditingPlugin(p);
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingPlugin) return;
    setSaving(true);
    try {
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

  return (
    <div className="space-y-6">
      {/* Header */}
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
              Zero-CGO subprocess & built-in Go plugins • Hot-reloadable configuration schemas
            </p>
          </div>
        </div>

        <button
          onClick={handleReload}
          disabled={reloading}
          className="inline-flex items-center gap-2 px-3.5 py-2 bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white text-xs font-semibold rounded-lg transition"
        >
          <RefreshCw className={`w-4 h-4 ${reloading ? 'animate-spin' : ''}`} />
          <span>{reloading ? 'Scanning...' : 'Re-Scan Plugin Directory'}</span>
        </button>
      </div>

      {/* Discovered Plugins Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {plugins.map((p) => (
          <div
            key={p.manifest.id}
            className="bg-slate-900/90 border border-slate-800 hover:border-purple-500/40 rounded-xl p-5 flex flex-col justify-between space-y-4 transition"
          >
            <div>
              <div className="flex items-start justify-between gap-3">
                <div>
                  <div className="flex flex-wrap items-center gap-2">
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
                  onClick={() => handleTogglePlugin(p)}
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

              {/* Capabilities & Binary Info */}
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

              {/* ConfigSchema Preview */}
              {p.manifest.configSchema && p.manifest.configSchema.length > 0 && (
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
                onClick={() => openConfigureModal(p)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium transition"
              >
                <Settings className="w-3.5 h-3.5 text-purple-400" />
                <span>Configure Settings</span>
              </button>
            </div>
          </div>
        ))}
      </div>

      {/* Developer SDK Guide Card */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl overflow-hidden">
        <div className="p-4 border-b border-slate-800 flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-slate-950/50">
          <div className="flex items-center gap-2.5">
            <Code2 className="w-5 h-5 text-purple-400" />
            <div>
              <h3 className="text-sm font-bold text-white">
                Write a Go Plugin in 1 File (`pkg/pluginsdk`)
              </h3>
              <p className="text-xs text-slate-400">
                Compile any Go binary with <code className="font-mono text-purple-300">pluginsdk.Serve(...)</code> into{' '}
                <code className="font-mono text-slate-300">./config/plugins/</code> and click Re-Scan
              </p>
            </div>
          </div>

          <button
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

        <div className="p-4 bg-slate-950 overflow-x-auto">
          <pre className="text-xs font-mono text-slate-300 leading-relaxed">
            {SDK_EXAMPLE_SNIPPET}
          </pre>
        </div>

        <div className="px-4 py-3 bg-slate-900/90 border-t border-slate-800 flex items-center gap-2 text-xs text-slate-400 font-mono overflow-x-auto">
          <Terminal className="w-4 h-4 text-emerald-400 shrink-0" />
          <span className="whitespace-nowrap">
            go build -o ./config/plugins/replacearr-plugin-custom ./plugins/custom && curl -X POST http://localhost:8989/api/v1/plugins/reload
          </span>
        </div>
      </div>

      {/* Dynamic Plugin Configuration Modal */}
      {editingPlugin && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <div className="min-w-0">
                <h3 className="text-base font-bold text-white break-words">
                  Configure {editingPlugin.manifest.name}
                </h3>
                <p className="text-xs text-slate-400 font-mono break-all">
                  PUT /api/v1/plugins/{editingPlugin.manifest.id}
                </p>
              </div>
              <button
                onClick={() => setEditingPlugin(null)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <form onSubmit={handleSaveConfig} className="p-5 space-y-4">
              <label className="flex items-center justify-between p-3 rounded-xl bg-slate-950 border border-slate-800 cursor-pointer">
                <div>
                  <div className="text-xs font-semibold text-white">Enable Plugin</div>
                  <div className="text-[11px] text-slate-400">
                    Include this plugin in federated search & automation hooks
                  </div>
                </div>
                <input
                  type="checkbox"
                  checked={formEnabled}
                  onChange={(e) => setFormEnabled(e.target.checked)}
                  className="w-4 h-4 accent-purple-500 rounded"
                />
              </label>

              {(editingPlugin.manifest.configSchema || []).map((field) => (
                <div key={field.name}>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    {field.label || field.name}{' '}
                    {field.required && <span className="text-rose-400">*</span>}
                  </label>
                  {field.type === 'checkbox' ? (
                    <input
                      type="checkbox"
                      checked={Boolean(formSettings[field.name])}
                      onChange={(e) =>
                        setFormSettings({ ...formSettings, [field.name]: e.target.checked })
                      }
                      className="w-4 h-4 accent-purple-500"
                    />
                  ) : (
                    <input
                      type={
                        field.type === 'password'
                          ? 'password'
                          : field.type === 'number'
                          ? 'number'
                          : 'text'
                      }
                      value={String(formSettings[field.name] ?? '')}
                      onChange={(e) =>
                        setFormSettings({
                          ...formSettings,
                          [field.name]:
                            field.type === 'number'
                              ? Number(e.target.value)
                              : e.target.value,
                        })
                      }
                      className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                    />
                  )}
                  {field.helpText && (
                    <p className="text-[11px] text-slate-500 mt-1">{field.helpText}</p>
                  )}
                </div>
              ))}

              <div className="pt-3 border-t border-slate-800 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setEditingPlugin(null)}
                  className="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={saving}
                  className="px-4 py-2 rounded-lg bg-purple-600 hover:bg-purple-500 text-white text-xs font-semibold transition"
                >
                  {saving ? 'Hot-Reloading...' : 'Save & Hot-Reload'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
