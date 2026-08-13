const CACHE = 'alset-sales-test-v3';
const PRECACHE = [
  '/w/sales-test.app.ans',
  '/static/apps/sales-test/manifest.webmanifest',
  '/static/apps/sales-test/icon-192.png',
  '/static/apps/sales-test/icon-512.png'
];
self.addEventListener('install', (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(PRECACHE).catch(() => {})).then(() => self.skipWaiting()));
});
self.addEventListener('activate', (e) => {
  e.waitUntil(caches.keys().then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k)))).then(() => self.clients.claim()));
});
self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.hostname.includes('supabase') || url.pathname.includes('/rest/v1') || url.pathname.includes('/auth/v1')) return;
  e.respondWith(
    fetch(req).then((res) => {
      const copy = res.clone();
      caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
      return res;
    }).catch(() => caches.match(req).then((r) => r || caches.match('/w/sales-test.app.ans')))
  );
});
