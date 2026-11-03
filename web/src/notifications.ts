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
