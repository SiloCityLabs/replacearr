import type { BrowserNotificationEvent } from './types';

const BROWSER_PLUGIN_ID = 'builtin-browser-notifications';

export async function requestPermission(): Promise<NotificationPermission> {
  if (typeof window === 'undefined' || !('Notification' in window)) {
    return 'denied';
  }
  if (Notification.permission === 'granted' || Notification.permission === 'denied') {
    return Notification.permission;
  }
  try {
    return await Notification.requestPermission();
  } catch {
    return 'denied';
  }
}

export function notifyBrowser(title: string, body?: string): void {
  if (typeof window === 'undefined' || !('Notification' in window)) {
    return;
  }
  if (Notification.permission !== 'granted') {
    return;
  }
  try {
    new Notification(title, {
      body: body || undefined,
      icon: '/icon-192.png',
    });
  } catch {
    // Ignore Notification constructor failures (e.g. insecure context).
  }
}

function titleForEvent(ev: BrowserNotificationEvent): { title: string; body?: string } {
  const payloadTitle =
    typeof ev.payload?.title === 'string' ? ev.payload.title : undefined;
  switch (ev.eventType) {
    case 'download.started':
      return { title: 'Download started', body: payloadTitle };
    case 'download.complete':
      return { title: 'Download complete', body: payloadTitle };
    case 'download.stalled':
      return { title: 'Download stalled', body: payloadTitle };
    case 'download.moved':
      return { title: 'Download moved', body: payloadTitle };
    case 'jellyfin.picked_up':
      return { title: 'Jellyfin picked up', body: payloadTitle };
    case 'notification.test':
      return { title: 'Test notification', body: payloadTitle || 'Replacearr test notification' };
    default:
      return { title: ev.eventType, body: payloadTitle };
  }
}

/** Subscribe to server-pushed browser notification events (SSE). Returns an unsubscribe fn. */
export function subscribeBrowserNotificationStream(
  enabled: boolean,
  onError?: (msg: string) => void
): () => void {
  if (!enabled || typeof window === 'undefined' || typeof EventSource === 'undefined') {
    return () => undefined;
  }

  const source = new EventSource('/api/v1/notifications/stream', { withCredentials: true });

  const onNotification = (msg: MessageEvent<string>) => {
    try {
      const ev = JSON.parse(msg.data) as BrowserNotificationEvent;
      const { title, body } = titleForEvent(ev);
      notifyBrowser(title, body);
    } catch {
      // Ignore malformed frames.
    }
  };

  source.addEventListener('notification', onNotification as EventListener);
  source.onerror = () => {
    // EventSource reconnects automatically; surface a soft warning once if closed permanently.
    if (source.readyState === EventSource.CLOSED) {
      onError?.('Browser notification stream closed');
    }
  };

  return () => {
    source.removeEventListener('notification', onNotification as EventListener);
    source.close();
  };
}

export function isBrowserNotificationPlugin(id: string): boolean {
  return id === BROWSER_PLUGIN_ID;
}

export { BROWSER_PLUGIN_ID };
