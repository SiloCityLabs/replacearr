import React, { useState } from 'react';
import {
  Users,
  ShieldCheck,
  Key,
  Plus,
  Trash2,
  Copy,
  Check,
  X,
  Lock,
} from 'lucide-react';
import type { APIKey, Role, User } from '../types';
import { api } from '../api';

interface UsersRbacTabProps {
  users: User[];
  roles: Role[];
  apiKeys: APIKey[];
  permissions: string[];
  onRefresh: () => Promise<void>;
  onNotify: (msg: string, isError?: boolean) => void;
}

const DEFAULT_PERMISSIONS = [
  'tv:read',
  'tv:write',
  'tv:delete',
  'movies:read',
  'movies:write',
  'movies:delete',
  'indexers:read',
  'indexers:write',
  'indexers:search',
  'downloads:read',
  'downloads:write',
  'profiles:read',
  'profiles:write',
  'plugins:read',
  'plugins:manage',
  'users:read',
  'users:manage',
  'system:read',
  'system:manage',
];

export const UsersRbacTab: React.FC<UsersRbacTabProps> = ({
  users,
  roles,
  apiKeys,
  permissions,
  onRefresh,
  onNotify,
}) => {
  const allPerms = permissions.length > 0 ? permissions : DEFAULT_PERMISSIONS;

  // Create User Modal State
  const [showUserModal, setShowUserModal] = useState(false);
  const [newUsername, setNewUsername] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [newRoleId, setNewRoleId] = useState<number>(roles[0]?.id || 1);

  // Role Editor State
  const [selectedRole, setSelectedRole] = useState<Role | null>(roles[0] || null);
  const [showNewRoleModal, setShowNewRoleModal] = useState(false);
  const [roleName, setRoleName] = useState('');
  const [roleDesc, setRoleDesc] = useState('');
  const [rolePerms, setRolePerms] = useState<string[]>([
    'tv:read',
    'movies:read',
    'indexers:search',
  ]);

  // Scoped API Key State
  const [keyName, setKeyName] = useState('');
  const [keyScope, setKeyScope] = useState('all');
  const [latestCreatedKey, setLatestCreatedKey] = useState<string | null>(null);
  const [copiedKeyId, setCopiedKeyId] = useState<string | null>(null);

  const handleCreateUser = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const created = await api.createUser({
        username: newUsername,
        email: newEmail,
        password: newPassword,
        roleId: newRoleId || roles[0]?.id || 1,
      });
      onNotify(`Created user "${created.username}" with role "${created.roleName}"`);
      setShowUserModal(false);
      setNewUsername('');
      setNewEmail('');
      setNewPassword('');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to create user', true);
    }
  };

  const handleToggleUserRole = async (u: User, roleId: number) => {
    try {
      await api.updateUser(u.id, {
        email: u.email,
        roleId,
        isActive: u.isActive,
      });
      onNotify(`Updated role for "${u.username}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to update user', true);
    }
  };

  const handleDeleteUser = async (u: User) => {
    try {
      await api.deleteUser(u.id);
      onNotify(`Deleted user "${u.username}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to delete user', true);
    }
  };

  const handleToggleRolePermission = async (role: Role, perm: string) => {
    const current = role.permissions || [];
    const exists = current.includes(perm);
    const updated = exists ? current.filter((p) => p !== perm) : [...current, perm];
    try {
      const saved = await api.updateRole(role.id, {
        description: role.description,
        permissions: updated,
      });
      setSelectedRole(saved);
      onNotify(`Updated permissions for role "${role.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to update role', true);
    }
  };

  const handleCreateRole = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const created = await api.createRole({
        name: roleName,
        description: roleDesc,
        permissions: rolePerms,
      });
      onNotify(`Created custom RBAC role "${created.name}"`);
      setShowNewRoleModal(false);
      setRoleName('');
      setRoleDesc('');
      setSelectedRole(created);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to create role', true);
    }
  };

  const handleCreateApiKey = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!keyName.trim()) return;
    try {
      const created = await api.createApiKey({
        name: keyName.trim(),
        scope: keyScope,
      });
      if (created.rawKey) {
        setLatestCreatedKey(created.rawKey);
      }
      onNotify(`Generated scoped API key "${created.name}" (${created.scope})`);
      setKeyName('');
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to create API key', true);
    }
  };

  const handleDeleteApiKey = async (k: APIKey) => {
    try {
      await api.deleteApiKey(k.id);
      onNotify(`Revoked API key "${k.name}"`);
      await onRefresh();
    } catch (err) {
      onNotify(err instanceof Error ? err.message : 'Failed to revoke API key', true);
    }
  };

  const copyText = (text: string, id: string) => {
    void navigator.clipboard.writeText(text);
    setCopiedKeyId(id);
    setTimeout(() => setCopiedKeyId(null), 2000);
  };

  const activeRole =
    roles.find((r) => r.id === selectedRole?.id) || roles[0] || null;

  return (
    <div className="space-y-6">
      {/* Users Section */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-indigo-500/15 text-indigo-400 border border-indigo-500/30">
              <Users className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-lg font-bold text-white">Users & Access Control</h2>
              <p className="text-xs text-slate-400">
                Multi-user accounts with granular role-based permissions (`tv:write`, `movies:write`, `indexers:search`)
              </p>
            </div>
          </div>

          <button
            onClick={() => {
              setNewRoleId(roles[0]?.id || 1);
              setShowUserModal(true);
            }}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold rounded-lg transition"
          >
            <Plus className="w-4 h-4" />
            <span>Add User</span>
          </button>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse">
            <thead>
              <tr className="border-b border-slate-800 text-[11px] font-semibold uppercase tracking-wider text-slate-400 bg-slate-950/60">
                <th className="py-3 px-4">User</th>
                <th className="py-3 px-4">Email</th>
                <th className="py-3 px-4">Role</th>
                <th className="py-3 px-4">Effective Permissions</th>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/70 text-xs">
              {users.map((u) => (
                <tr key={u.id} className="hover:bg-slate-800/30 transition">
                  <td className="py-3 px-4 font-semibold text-white">{u.username}</td>
                  <td className="py-3 px-4 text-slate-400 font-mono">
                    {u.email || `${u.username}@replacearr.local`}
                  </td>
                  <td className="py-3 px-4">
                    <select
                      value={u.roleId}
                      onChange={(e) => handleToggleUserRole(u, Number(e.target.value))}
                      className="px-2.5 py-1 bg-slate-950 border border-slate-800 rounded-lg text-xs text-indigo-300 font-medium"
                    >
                      {roles.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.name}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="py-3 px-4">
                    <div className="flex flex-wrap gap-1 max-w-md">
                      {(u.permissions || []).slice(0, 5).map((perm) => (
                        <span
                          key={perm}
                          className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-slate-950 border border-slate-800 text-slate-300"
                        >
                          {perm}
                        </span>
                      ))}
                      {(u.permissions || []).length > 5 && (
                        <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-indigo-500/15 text-indigo-300">
                          +{(u.permissions || []).length - 5} more
                        </span>
                      )}
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                      {u.isActive ? 'Active' : 'Disabled'}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-right">
                    {u.username !== 'admin' && (
                      <button
                        onClick={() => handleDeleteUser(u)}
                        className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10 transition"
                        title="Delete user"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Roles & Interactive Permission Matrix + Scoped API Keys */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Interactive Role Matrix */}
        <div className="lg:col-span-7 bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <ShieldCheck className="w-5 h-5 text-emerald-400" />
              <div>
                <h3 className="text-base font-bold text-white">RBAC Roles & Permission Matrix</h3>
                <p className="text-xs text-slate-400">
                  Select a role to inspect or toggle granular permissions
                </p>
              </div>
            </div>

            <button
              onClick={() => setShowNewRoleModal(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold rounded-lg transition"
            >
              <Plus className="w-3.5 h-3.5" />
              <span>New Role</span>
            </button>
          </div>

          {/* Role Selector Pills */}
          <div className="flex flex-wrap gap-2">
            {roles.map((r) => (
              <button
                key={r.id}
                onClick={() => setSelectedRole(r)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition ${
                  activeRole?.id === r.id
                    ? 'bg-emerald-500/20 border-emerald-500/40 text-emerald-300'
                    : 'bg-slate-950 border-slate-800 text-slate-400 hover:text-white'
                }`}
              >
                <span>{r.name}</span>
                {r.builtIn && (
                  <span className="ml-1.5 text-[10px] opacity-60 font-mono">(built-in)</span>
                )}
              </button>
            ))}
          </div>

          {activeRole && (
            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h4 className="text-sm font-bold text-white capitalize">{activeRole.name}</h4>
                  <p className="text-xs text-slate-400">{activeRole.description}</p>
                </div>
                <span className="text-xs font-mono text-emerald-400">
                  {activeRole.permissions?.length || 0} permissions
                </span>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-2">
                {allPerms.map((perm) => {
                  const checked =
                    activeRole.permissions?.includes('*') ||
                    activeRole.permissions?.includes(perm);
                  return (
                    <label
                      key={perm}
                      className={`flex items-center gap-2 p-2 rounded-lg border text-xs font-mono cursor-pointer transition ${
                        checked
                          ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-200'
                          : 'bg-slate-900/70 border-slate-800 text-slate-400'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={Boolean(checked)}
                        disabled={activeRole.name === 'admin'}
                        onChange={() => handleToggleRolePermission(activeRole, perm)}
                        className="w-3.5 h-3.5 accent-emerald-500 rounded"
                      />
                      <span className="truncate">{perm}</span>
                    </label>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {/* Scoped API Keys */}
        <div className="lg:col-span-5 bg-slate-900/90 border border-slate-800 rounded-xl p-5 space-y-4">
          <div className="flex items-center gap-2.5">
            <Key className="w-5 h-5 text-amber-400" />
            <div>
              <h3 className="text-base font-bold text-white">Scoped API Keys</h3>
              <p className="text-xs text-slate-400">
                Create keys scoped to <code className="font-mono">all</code>,{' '}
                <code className="font-mono">sonarr</code>, <code className="font-mono">radarr</code>, or{' '}
                <code className="font-mono">prowlarr</code>
              </p>
            </div>
          </div>

          <form onSubmit={handleCreateApiKey} className="space-y-3 bg-slate-950 p-3.5 rounded-xl border border-slate-800">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <input
                type="text"
                value={keyName}
                onChange={(e) => setKeyName(e.target.value)}
                placeholder="Key label (e.g. Overseerr, Bazarr)..."
                className="sm:col-span-2 px-3 py-2 bg-slate-900 border border-slate-800 rounded-lg text-xs text-white"
              />
              <select
                value={keyScope}
                onChange={(e) => setKeyScope(e.target.value)}
                className="px-2.5 py-2 bg-slate-900 border border-slate-800 rounded-lg text-xs text-amber-300 font-mono"
              >
                <option value="all">scope: all</option>
                <option value="sonarr">scope: sonarr</option>
                <option value="radarr">scope: radarr</option>
                <option value="prowlarr">scope: prowlarr</option>
              </select>
            </div>
            <button
              type="submit"
              className="w-full py-2 bg-amber-500 hover:bg-amber-400 text-slate-950 text-xs font-bold rounded-lg transition"
            >
              + Generate Scoped API Key
            </button>
          </form>

          {latestCreatedKey && (
            <div className="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 space-y-1.5">
              <div className="flex items-center justify-between text-xs text-emerald-300 font-semibold">
                <span>New API Key Generated</span>
                <button
                  onClick={() => copyText(latestCreatedKey, 'latest')}
                  className="inline-flex items-center gap-1 underline"
                >
                  {copiedKeyId === 'latest' ? 'Copied!' : 'Copy Key'}
                </button>
              </div>
              <code className="block text-xs font-mono text-white bg-slate-950 p-2 rounded border border-slate-800 break-all">
                {latestCreatedKey}
              </code>
            </div>
          )}

          <div className="space-y-2.5">
            {apiKeys.map((k) => (
              <div
                key={k.id}
                className="p-3 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-between gap-2"
              >
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-bold text-white truncate">{k.name}</span>
                    <span className="text-[10px] font-mono uppercase px-2 py-0.5 rounded bg-amber-500/15 text-amber-300 border border-amber-500/30">
                      {k.scope}
                    </span>
                  </div>
                  <div className="text-[11px] font-mono text-slate-400 mt-1 flex items-center gap-2 min-w-0">
                    <Lock className="w-3 h-3 text-slate-500 shrink-0" />
                    <span className="truncate">{k.rawKey ? k.rawKey : `${k.keyPrefix}••••••••••••••••`}</span>
                  </div>
                </div>

                <div className="flex items-center gap-1.5 shrink-0">
                  {k.rawKey && (
                    <button
                      onClick={() => copyText(k.rawKey!, String(k.id))}
                      className="p-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 text-slate-300"
                      title="Copy API Key"
                    >
                      {copiedKeyId === String(k.id) ? (
                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                      ) : (
                        <Copy className="w-3.5 h-3.5" />
                      )}
                    </button>
                  )}
                  <button
                    onClick={() => handleDeleteApiKey(k)}
                    className="p-1.5 rounded-lg text-slate-500 hover:text-rose-400 hover:bg-rose-500/10"
                    title="Revoke API Key"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Create User Modal */}
      {showUserModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <h3 className="text-base font-bold text-white">Create User Account</h3>
              <button
                onClick={() => setShowUserModal(false)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <form onSubmit={handleCreateUser} className="p-5 space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Username</label>
                <input
                  type="text"
                  required
                  value={newUsername}
                  onChange={(e) => setNewUsername(e.target.value)}
                  placeholder="jellyseerr-bot"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Email</label>
                <input
                  type="email"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  placeholder="user@replacearr.local"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Password</label>
                <input
                  type="password"
                  required
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  placeholder="••••••••"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Assign Role</label>
                <select
                  value={newRoleId}
                  onChange={(e) => setNewRoleId(Number(e.target.value))}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                >
                  {roles.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name} — {r.description}
                    </option>
                  ))}
                </select>
              </div>
              <div className="pt-3 border-t border-slate-800 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowUserModal(false)}
                  className="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold"
                >
                  Create User
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Create Custom Role Modal */}
      {showNewRoleModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm overflow-y-auto flex items-start justify-center p-4">
          <div className="w-full max-w-lg bg-slate-900 border border-slate-800 rounded-2xl shadow-2xl max-h-[calc(100dvh-2rem)] overflow-y-auto my-auto">
            <div className="p-5 border-b border-slate-800 flex items-center justify-between">
              <h3 className="text-base font-bold text-white">Create Custom RBAC Role</h3>
              <button
                onClick={() => setShowNewRoleModal(false)}
                className="p-1.5 rounded-lg bg-slate-800 text-slate-400 hover:text-white"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <form onSubmit={handleCreateRole} className="p-5 space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Role Name</label>
                <input
                  type="text"
                  required
                  value={roleName}
                  onChange={(e) => setRoleName(e.target.value)}
                  placeholder="e.g. family-requester"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description</label>
                <input
                  type="text"
                  value={roleDesc}
                  onChange={(e) => setRoleDesc(e.target.value)}
                  placeholder="Can request movies and series without deleting files"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-2">
                  Permissions
                </label>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 max-h-52 overflow-y-auto p-2 bg-slate-950 rounded-lg border border-slate-800">
                  {allPerms.map((perm) => {
                    const checked = rolePerms.includes(perm);
                    return (
                      <label
                        key={perm}
                        className="flex items-center gap-2 text-xs font-mono text-slate-300 cursor-pointer"
                      >
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() =>
                            setRolePerms(
                              checked
                                ? rolePerms.filter((p) => p !== perm)
                                : [...rolePerms, perm]
                            )
                          }
                          className="w-3.5 h-3.5 accent-emerald-500"
                        />
                        <span className="truncate">{perm}</span>
                      </label>
                    );
                  })}
                </div>
              </div>
              <div className="pt-3 border-t border-slate-800 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setShowNewRoleModal(false)}
                  className="px-4 py-2 rounded-lg bg-slate-800 text-slate-300 text-xs font-medium"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold"
                >
                  Save Role
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
