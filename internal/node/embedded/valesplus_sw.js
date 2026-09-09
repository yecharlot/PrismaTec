const CACHE='valesplus-v6';
const SHELL=[
  '/w/valesplus.app.ans',
  '/static/apps/valesplus/manifest.webmanifest',
  '/static/apps/valesplus/icon-192.png',
  '/static/apps/valesplus/icon-512.png',
  '/static/apps/valesplus/apple-touch-icon.png',
  'https://cdn.jsdelivr.net/npm/qrcodejs@1.0.0/qrcode.min.js',
  'https://fonts.googleapis.com/css2?family=Archivo:wght@400;500;600;700;800&family=Instrument+Serif:ital@0;1&display=swap'
];
self.addEventListener('install',e=>{
  e.waitUntil(caches.open(CACHE).then(c=>c.addAll(SHELL).catch(()=>{})).then(()=>self.skipWaiting()));
});
self.addEventListener('activate',e=>{
  e.waitUntil(caches.keys().then(ks=>Promise.all(ks.filter(k=>k!==CACHE).map(k=>caches.delete(k)))).then(()=>self.clients.claim()));
});
self.addEventListener('fetch',e=>{
  if(e.request.method!=='GET') return;
  const u=e.request.url;
  // API network-first
  if(u.includes('/api/valesplus/')){
    e.respondWith(fetch(e.request).catch(()=>caches.match(e.request)));
    return;
  }
  // app shell: network first then cache (hot update)
  if(u.includes('valesplus')||u.includes('fonts.g')||u.includes('qrcode')||u.includes('googleapis')||u.includes('gstatic')){
    e.respondWith(
      fetch(e.request).then(res=>{
        const copy=res.clone();
        caches.open(CACHE).then(c=>{try{c.put(e.request,copy)}catch(x){}});
        return res;
      }).catch(()=>caches.match(e.request).then(m=>m||caches.match('/w/valesplus.app.ans')))
    );
  }
});
