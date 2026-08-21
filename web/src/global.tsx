import { Button, message, notification } from 'antd';
import { getIntl } from '@umijs/max';
import defaultSettings from '../config/defaultSettings';

const { pwa } = defaultSettings;

const clearCache = async () => {
  // remove all caches
  if (window.caches) {
    try {
      const keys = await caches.keys();
      await Promise.all(keys.map((key) => caches.delete(key)));
    } catch (e) {
      console.log(e);
    }
  }
};

const unregisterServiceWorkers = async () => {
  if (!('serviceWorker' in navigator)) {
    return;
  }
  const registrations = await navigator.serviceWorker.getRegistrations();
  await Promise.all(registrations.map((registration) => registration.unregister()));
};

const chunkReloadKey = 'xtrading-console-chunk-reload-at';
const isChunkLoadError = (value: unknown) => {
  const message = value instanceof Error ? value.message : String(value ?? '');
  return /ChunkLoadError|Loading chunk .+ failed|Failed to fetch dynamically imported module/i.test(
    message,
  );
};

const reloadLatestApplication = async () => {
  const now = Date.now();
  const lastReloadAt = Number(sessionStorage.getItem(chunkReloadKey) || 0);
  if (now - lastReloadAt < 10_000) {
    return;
  }

  sessionStorage.setItem(chunkReloadKey, String(now));
  await clearCache();
  await unregisterServiceWorkers();
  window.location.reload();
};

window.addEventListener('error', (event) => {
  if (isChunkLoadError(event.error || event.message)) {
    void reloadLatestApplication();
  }
});

window.addEventListener('unhandledrejection', (event) => {
  if (isChunkLoadError(event.reason)) {
    event.preventDefault();
    void reloadLatestApplication();
  }
});

// if pwa is true
if (pwa) {
  // Notify user if offline now
  window.addEventListener('sw.offline', () => {
    message.warning(getIntl().formatMessage({ id: 'app.pwa.offline' }));
  });

  // Pop up a prompt on the page asking the user if they want to use the latest version
  window.addEventListener('sw.updated', (event: Event) => {
    const e = event as CustomEvent;
    const reloadSW = async () => {
      // Check if there is sw whose state is waiting in ServiceWorkerRegistration
      // https://developer.mozilla.org/en-US/docs/Web/API/ServiceWorkerRegistration
      const worker = e.detail && e.detail.waiting;
      if (!worker) {
        return true;
      }
      // Send skip-waiting event to waiting SW with MessageChannel
      await new Promise((resolve, reject) => {
        const channel = new MessageChannel();
        channel.port1.onmessage = (msgEvent) => {
          if (msgEvent.data.error) {
            reject(msgEvent.data.error);
          } else {
            resolve(msgEvent.data);
          }
        };
        worker.postMessage({ type: 'skip-waiting' }, [channel.port2]);
      });

      await clearCache();
      window.location.reload();
      return true;
    };
    const key = `open${Date.now()}`;
    const btn = (
      <Button
        type="primary"
        onClick={() => {
          notification.destroy(key);
          reloadSW();
        }}
      >
        {getIntl().formatMessage({ id: 'app.pwa.serviceworker.updated.ok' })}
      </Button>
    );
    notification.open({
      message: getIntl().formatMessage({ id: 'app.pwa.serviceworker.updated' }),
      description: getIntl().formatMessage({ id: 'app.pwa.serviceworker.updated.hint' }),
      btn,
      key,
      onClose: async () => null,
    });
  });
} else if ('serviceWorker' in navigator) {
  // unregister service worker
  void unregisterServiceWorkers();
  void clearCache();
}
