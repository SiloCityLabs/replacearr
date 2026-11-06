import React, { useCallback, useEffect, useState } from 'react';
import { MonitorSmartphone, ShieldOff, Trash2 } from 'lucide-react';
import type { AuthSession } from '../../types';
import { api, setStoredToken } from '../../api';
import { formatAppDate } from '../../uiPrefs';
import {
  dangerButtonClass,
  errorText,
  PageIntro,
  primaryButtonClass,
  quietButtonClass,
  Section,
} from './fields';
import { useConfirm } from '../modals/ConfirmModal';

interface SessionsPageProps {
  onNotify: (msg: string, isError?: boolean) => void;
  onSignedOut: () => void;
}

function summarizeUserAgent(ua: string): string {
  const value = ua.trim();
  if (!value) return 'Unknown device';
  if (/Edg\//i.test(value)) return 'Microsoft Edge';
  if (/Chrome\//i.test(value) && !/Chromium/i.test(value)) return 'Chrome';
  if (/Firefox\//i.test(value)) return 'Firefox';
  if (/Safari\//i.test(value) && !/Chrome/i.test(value)) return 'Safari';
  if (/Replacearr/i.test(value)) return value.slice(0, 48);
  return value.length > 64 ? `${value.slice(0, 61)}…` : value;
}

function formatWhen(iso: string): string {
  return formatAppDate(iso, null, 'short');
}

export const SessionsPage: React.FC<SessionsPageProps> = ({ onNotify, onSignedOut }) => {
  const { confirm, dialog } = useConfirm();
  const [sessions, setSessions] = useState<AuthSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    const rows = await api.getSessions();
    setSessions(rows);
  }, []);

  useEffect(() => {
    let active = true;
    api
      .getSessions()
      .then((rows) => {
        if (active) setSessions(rows);
      })
      .catch((err) => {
        if (active) onNotify(errorText(err, 'Failed to load sessions'), true);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [onNotify]);

  const handleRevoke = async (session: AuthSession) => {
    const label = session.current ? 'this device' : summarizeUserAgent(session.userAgent);
    const ok = await confirm({
      title: session.current ? 'Sign out this device' : 'Kill session',
      message: session.current
        ? 'End the session on this device? You will need to sign in again.'
        : `End the session for ${label}? That device will need to sign in again.`,
      confirmLabel: session.current ? 'Sign out' : 'Kill session',
    });
    if (!ok) return;

    setBusyId(session.id);
    try {
      const res = await api.revokeSession(session.id);
      if (res.currentSession) {
        setStoredToken(null);
        onSignedOut();
        onNotify('Signed out');
        return;
      }
      onNotify('Session ended');
      await load();
    } catch (err) {
      onNotify(errorText(err, 'Failed to kill session'), true);
    } finally {
      setBusyId(null);
    }
  };

  const handleRevokeOthers = async () => {
    const others = sessions.filter((s) => !s.current).length;
    if (others === 0) {
      onNotify('No other sessions to end');
      return;
    }
    const ok = await confirm({
      title: 'Sign out other devices',
      message: `End ${others} other active session${others === 1 ? '' : 's'}? This device stays signed in.`,
      confirmLabel: 'Sign out others',
    });
    if (!ok) return;

    setBusyId('others');
    try {
      const res = await api.revokeOtherSessions();
      onNotify(`Ended ${res.revoked} other session${res.revoked === 1 ? '' : 's'}`);
      await load();
    } catch (err) {
      onNotify(errorText(err, 'Failed to end other sessions'), true);
    } finally {
      setBusyId(null);
    }
  };

  return (
    <div>
      {dialog}
      <PageIntro
        title="Sessions"
        caption="Active sign-ins for your account. Kill a session to force that device to sign in again."
      />

      <Section
        title="Active sessions"
        caption="Sessions come from Sign In (including Keep me signed in). API keys are managed under Users & RBAC."
      >
        <div className="flex flex-wrap items-center justify-between gap-2 mt-3 mb-3">
          <p className="text-xs text-slate-500">
            {loading ? 'Loading…' : `${sessions.length} active session${sessions.length === 1 ? '' : 's'}`}
          </p>
          <button
            type="button"
            className={quietButtonClass}
            disabled={loading || busyId !== null || sessions.filter((s) => !s.current).length === 0}
            onClick={() => void handleRevokeOthers()}
          >
            <ShieldOff className="w-3.5 h-3.5" />
            Sign out other devices
          </button>
        </div>

        {loading ? (
          <p className="text-sm text-slate-400 py-4">Loading sessions…</p>
        ) : sessions.length === 0 ? (
          <p className="text-sm text-slate-400 py-4">No active sessions.</p>
        ) : (
          <div className="space-y-2.5">
            {sessions.map((session) => (
              <div
                key={session.id}
                className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 flex flex-col sm:flex-row sm:items-center gap-3"
              >
                <div className="flex items-start gap-3 min-w-0 flex-1">
                  <div className="mt-0.5 p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-300 shrink-0">
                    <MonitorSmartphone className="w-4 h-4" />
                  </div>
                  <div className="min-w-0 space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-sm font-semibold text-white">
                        {summarizeUserAgent(session.userAgent)}
                      </span>
                      {session.current ? (
                        <span className="text-[10px] uppercase tracking-wide px-2 py-0.5 rounded bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                          This device
                        </span>
                      ) : null}
                      {session.rememberMe ? (
                        <span className="text-[10px] uppercase tracking-wide px-2 py-0.5 rounded bg-indigo-500/15 text-indigo-300 border border-indigo-500/30">
                          Keep signed in
                        </span>
                      ) : (
                        <span className="text-[10px] uppercase tracking-wide px-2 py-0.5 rounded bg-slate-800 text-slate-400 border border-slate-700">
                          Browser session
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-slate-400 flex flex-wrap gap-x-3 gap-y-1">
                      <span>IP {session.ipAddress || '—'}</span>
                      <span>Signed in {formatWhen(session.createdAt)}</span>
                      <span>Last seen {formatWhen(session.lastSeenAt)}</span>
                      <span>Expires {formatWhen(session.expiresAt)}</span>
                    </div>
                  </div>
                </div>
                <button
                  type="button"
                  className={session.current ? dangerButtonClass : primaryButtonClass}
                  disabled={busyId !== null}
                  onClick={() => void handleRevoke(session)}
                  title={session.current ? 'Sign out this device' : 'Kill session'}
                >
                  <Trash2 className="w-3.5 h-3.5" />
                  {session.current ? 'Sign out' : 'Kill'}
                </button>
              </div>
            ))}
          </div>
        )}
      </Section>
    </div>
  );
};
