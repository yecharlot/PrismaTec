const CACHE='valesplus-v2';
const ASSETS=['/w/valesplus.app.ans','/static/apps/valesplus/manifest.webmanifest','/static/apps/valesplus/icon-192.png','/static/apps/valesplus/icon-512.png'];
self.addEventListener('install',e=>{e.waitUntil(caches.open(CACHE).then(c=>c.addAll(ASSETS).catch(()=>{})).then(()=>self.skipWaiting()))});
self.addEventListener('activate',e=>{e.waitUntil(caches.keys().then(keys=>Promise.all(keys.filter(k=>k!==CACHE).map(k=>caches.delete(k)))).then(()=>self.clients.claim()))});
self.addEventListener('fetch',e=>{
  const u=e.request.url;
  if(e.request.method!=='GET') return;
  e.respondWith(
    caches.open(CACHE).then(async cache=>{
      try{
        const net=await fetch(e.request);
        if(net&&net.ok&&(u.includes('valesplus')||u.includes('icon')||u.includes('manifest'))) try{cache.put(e.request,net.clone())}catch(x){}
        return net;
      }catch(err){
        const hit=await cache.match(e.request);
        return hit||cache.match('/w/valesplus.app.ans');
      }
    })
  );
});
