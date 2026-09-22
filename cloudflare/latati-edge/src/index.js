/**
 * Latati Edge — same Durable Object store as Render (AlsetStoreDO via alset-network).
 * Photos in DO keys latati/v1/photo/{id}
 */
const STORE_BASE = "https://alset-network.lhmolam-877.workers.dev";
let _env = null;
function storeURL(path) {
  return STORE_BASE + path;
}
async function storeFetch(path, init) {
  // Prefer service binding (same CF account, no WAF 1010)
  if (_env && _env.ALSET_NETWORK) {
    const u = new URL(path, "https://alset-network.internal");
    return _env.ALSET_NETWORK.fetch(u.toString(), init);
  }
  return fetch(STORE_BASE + path, init);
}
const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET,POST,OPTIONS,PUT,DELETE",
  "Access-Control-Allow-Headers": "Content-Type, X-LaTati-Token, X-LaTati-Thread",
};

function json(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json", ...CORS },
  });
}
function corsOk() {
  return new Response(null, { status: 204, headers: CORS });
}
function randHex(n = 8) {
  const a = new Uint8Array(n);
  crypto.getRandomValues(a);
  return [...a].map((b) => b.toString(16).padStart(2, "0")).join("");
}
function normTenant(s) {
  s = String(s || "").toLowerCase().trim().replace(/[^a-z0-9-]+/g, "-").replace(/-+/g, "-").replace(/^-|-$/g, "");
  if (!s || s === "gestion-latati") return "latati";
  if (s.startsWith("gestion-")) s = s.slice(8);
  return s || "latati";
}
function storeKey(tenant) {
  tenant = normTenant(tenant);
  return tenant === "latati" ? "latati/v1/store" : `latati/v1/t/${tenant}/store`;
}
function photoKey(tenant, id) {
  tenant = normTenant(tenant);
  const pref = tenant === "latati" ? "latati/v1/photo/" : `latati/v1/t/${tenant}/photo/`;
  return pref + id;
}
function defaultStore(tenant) {
  const isLatati = tenant === "latati";
  return {
    profile: {
      name: isLatati ? "Dayanis Perez Soria" : "Mi tienda",
      whatsapp: isLatati ? "5351069717" : "",
      address: "",
      bio: isLatati ? "La Tati · catálogo y pedidos en la app" : "Catálogo y pedidos",
      pin: isLatati ? "tati2026" : "1234",
      accent: "#e8b4c8",
      accent2: "#c4789a",
      tagline: "Tu pedido · tu vale",
    },
    products: {},
    orders: [],
    messages: [],
    interests: [],
    tokens: {},
    seq_order: 0,
    rev: 1,
    orders_seen: 0,
  };
}
function publicProfile(p) {
  return {
    name: p.name || "", whatsapp: p.whatsapp || "", phone: p.whatsapp || "",
    address: p.address || "", bio: p.bio || "",
    accent: p.accent || "#e8b4c8", accent2: p.accent2 || "#c4789a",
    bg: p.bg || "", surface: p.surface || "", tagline: p.tagline || "",
  };
}
function productAvailable(p) {
  return p && !p.sold_out;
}
function b64ToBytes(b64) {
  const bin = atob(b64);
  const u = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) u[i] = bin.charCodeAt(i);
  return u;
}
function bytesToB64(u8) {
  let s = "";
  const chunk = 0x8000;
  for (let i = 0; i < u8.length; i += chunk) {
    s += String.fromCharCode.apply(null, u8.subarray(i, i + chunk));
  }
  return btoa(s);
}

async function kvGet(key) {
  const res = await storeFetch(`/api/store/kv?key=${encodeURIComponent(key)}`, {
    headers: { Accept: "application/json" },
  });
  if (res.status === 404) return null;
  if (!res.ok) throw new Error("store get " + res.status);
  const j = await res.json();
  if (!j.ok || j.data == null) return null;
  return b64ToBytes(j.data);
}
async function kvPut(key, bytes) {
  const res = await storeFetch(`/api/store/kv?key=${encodeURIComponent(key)}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ data: bytesToB64(bytes instanceof Uint8Array ? bytes : new TextEncoder().encode(bytes)) }),
  });
  if (!res.ok) throw new Error("store put " + res.status);
  return true;
}
async function kvDel(key) {
  await storeFetch(`/api/store/kv?key=${encodeURIComponent(key)}`, { method: "DELETE" });
}

async function loadStore(tenant) {
  const raw = await kvGet(storeKey(tenant));
  if (!raw) {
    const st = defaultStore(tenant);
    await saveStore(tenant, st, false);
    return st;
  }
  const st = JSON.parse(new TextDecoder().decode(raw));
  st.products = st.products || {};
  st.tokens = st.tokens || {};
  st.orders = st.orders || [];
  st.messages = st.messages || [];
  st.interests = st.interests || [];
  return st;
}
async function saveStore(tenant, st, bump = true) {
  if (bump) st.rev = (st.rev || 0) + 1;
  const raw = new TextEncoder().encode(JSON.stringify(st));
  await kvPut(storeKey(tenant), raw);
  return st.rev;
}
function auth(request, st) {
  const tok = request.headers.get("X-LaTati-Token") || "";
  if (!tok) return false;
  const exp = st.tokens[tok];
  return exp && Date.now() / 1000 <= exp;
}

function sniffImage(u8) {
  if (u8.length >= 3 && u8[0] === 0xff && u8[1] === 0xd8) return "image/jpeg";
  if (u8.length >= 8 && u8[0] === 0x89 && u8[1] === 0x50) return "image/png";
  if (u8.length >= 6 && u8[0] === 0x47 && u8[1] === 0x49) return "image/gif";
  if (u8.length >= 12 && u8[8] === 0x57 && u8[9] === 0x45) return "image/webp";
  return "application/octet-stream";
}

export default {
  async fetch(request, env) {
    _env = env;
    if (request.method === "OPTIONS") return corsOk();
    const url = new URL(request.url);
    let path = url.pathname.replace(/\/+$/, "") || "/";
    if (path.startsWith("/api/latati")) path = path.slice("/api/latati".length) || "/";
    path = path.replace(/^\//, "");

    if (path === "" || path === "/") {
      return json({
        ok: true,
        name: "La Tati edge",
        store: "AlsetStoreDO (shared with Render)",
        endpoints: ["catalog", "profile", "order", "orders", "chat", "gestor/login", "photo"],
      });
    }
    if (path === "sw.js") {
      return new Response("self.addEventListener('install',e=>self.skipWaiting());", {
        headers: { "Content-Type": "application/javascript", ...CORS },
      });
    }
    if (path === "manifest.webmanifest" || path === "manifest.json") {
      const gestion = url.searchParams.get("gestion") === "1";
      return json({
        name: "La Tati",
        short_name: "La Tati",
        start_url: gestion ? "https://yecharlot.github.io/Gestion-Latati/" : "https://yecharlot.github.io/Latati/",
        display: "standalone",
        theme_color: "#e8b4c8",
        background_color: "#0c0a0f",
        icons: [{ src: "https://latati-edge.lhmolam-877.workers.dev/api/latati/icon-192.png", sizes: "192x192", type: "image/png" }],
      });
    }
    if (path === "icon-192.png" || path === "icon.png" || path === "icon-512.png") {
      // 1x1 pink placeholder; clients may use local icon
      const b64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==";
      return new Response(b64ToBytes(b64), { headers: { "Content-Type": "image/png", ...CORS } });
    }

    let tenant = "latati";
    const parts0 = path.split("/").filter(Boolean);
    if (parts0[0] === "t" && parts0[1]) {
      tenant = normTenant(parts0[1]);
      parts0.splice(0, 2);
    }
    if (parts0[0] === "tenants") {
      return json({ ok: true, items: [{ slug: "latati", name: "La Tati", active: true }] });
    }

    const parts = parts0;

    // photo GET (binary)
    if (parts[0] === "photo" && parts[1] && request.method === "GET") {
      const data = await kvGet(photoKey(tenant, parts[1]));
      if (!data || !data.length) return json({ ok: false, error: "not found" }, 404);
      return new Response(data, {
        headers: { "Content-Type": sniffImage(data), "Cache-Control": "public,max-age=300", ...CORS },
      });
    }

    let st;
    try {
      st = await loadStore(tenant);
    } catch (e) {
      return json({ ok: false, error: "store: " + (e.message || e) }, 503);
    }

    try {
      if (parts[0] === "catalog" && request.method === "GET") {
        let list = Object.values(st.products).filter(productAvailable);
        list.sort((a, b) => (b.created_at || "").localeCompare(a.created_at || ""));
        return json({ ok: true, items: list, profile: publicProfile(st.profile), rev: st.rev });
      }
      if (parts[0] === "profile" && request.method === "GET") {
        return json({ ok: true, profile: publicProfile(st.profile), rev: st.rev });
      }
      if (parts[0] === "product" && parts[1] && request.method === "GET") {
        const p = st.products[parts[1]];
        if (!p) return json({ ok: false, error: "not found" }, 404);
        return json({ ok: true, item: p, profile: publicProfile(st.profile) });
      }
      if (parts[0] === "gestor" && parts[1] === "login" && request.method === "POST") {
        const body = await request.json().catch(() => ({}));
        if (String(body.pin || "") !== String(st.profile.pin || "")) return json({ ok: false, error: "pin" }, 401);
        const token = randHex(16);
        st.tokens[token] = Math.floor(Date.now() / 1000) + 86400 * 30;
        await saveStore(tenant, st);
        return json({ ok: true, token, profile: publicProfile(st.profile) });
      }
      if (parts[0] === "gestor" && parts[1] === "profile" && request.method === "POST") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        const body = await request.json().catch(() => ({}));
        const p = st.profile;
        ["name", "whatsapp", "address", "bio", "tagline", "accent", "accent2", "pin"].forEach((k) => {
          if (body[k] != null) p[k] = String(body[k]);
        });
        const rev = await saveStore(tenant, st);
        return json({ ok: true, profile: publicProfile(st.profile), rev });
      }
      if (parts[0] === "gestor" && parts[1] === "products") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        if (request.method === "GET" && parts.length === 2) {
          const list = Object.values(st.products).sort((a, b) => (b.created_at || "").localeCompare(a.created_at || ""));
          return json({ ok: true, items: list });
        }
        // photo upload: POST .../products/:id/photo
        if (request.method === "POST" && parts.length >= 4 && parts[3] === "photo") {
          const id = parts[2];
          if (!st.products[id]) return json({ ok: false, error: "not found" }, 404);
          const ct = request.headers.get("Content-Type") || "";
          let bytes;
          if (ct.includes("multipart/form-data")) {
            const form = await request.formData();
            const f = form.get("photo") || form.get("file") || form.get("image");
            if (!f || typeof f.arrayBuffer !== "function") return json({ ok: false, error: "no file" }, 400);
            bytes = new Uint8Array(await f.arrayBuffer());
          } else {
            bytes = new Uint8Array(await request.arrayBuffer());
          }
          if (bytes.length < 32) return json({ ok: false, error: "empty" }, 400);
          if (bytes.length > 4_500_000) return json({ ok: false, error: "too large" }, 400);
          await kvPut(photoKey(tenant, id), bytes);
          st.products[id].photo = true;
          st.products[id].updated_at = new Date().toISOString();
          const rev = await saveStore(tenant, st);
          return json({ ok: true, photo: `/api/latati/photo/${id}`, rev });
        }
        if (request.method === "POST" && parts.length === 2) {
          const body = await request.json().catch(() => ({}));
          let id = body.id || randHex(8);
          let p = st.products[id] || {
            id, created_at: new Date().toISOString(), unlimited: true, stock: 0,
            sold: false, sold_out: false, photo: false,
          };
          const now = new Date().toISOString();
          if (body.title != null) p.title = String(body.title);
          if (body.description != null) p.description = String(body.description);
          if (body.price != null) { p.price = Number(body.price) || 0; p.price_pending = !(p.price > 0); }
          if (body.currency) p.currency = String(body.currency);
          if (body.category != null) p.category = String(body.category || "General");
          if (body.delivery_mode) p.delivery_mode = String(body.delivery_mode);
          if (body.pickup_address != null) p.pickup_address = String(body.pickup_address);
          if (body.owner_name != null) p.owner_name = String(body.owner_name);
          p.updated_at = now;
          st.products[id] = p;
          const rev = await saveStore(tenant, st);
          return json({ ok: true, item: p, rev });
        }
        if (request.method === "POST" && parts.length >= 4) {
          const id = parts[2], action = parts[3];
          const p = st.products[id];
          if (!p) return json({ ok: false, error: "not found" }, 404);
          const now = new Date().toISOString();
          if (["soldout", "sold", "agotado"].includes(action)) {
            p.sold = true; p.sold_out = true; p.sold_out_at = now; p.updated_at = now;
          } else if (["restore", "unsold"].includes(action)) {
            p.sold = false; p.sold_out = false; p.sold_out_at = ""; p.updated_at = now;
          } else if (action === "delete") {
            delete st.products[id];
            try { await kvDel(photoKey(tenant, id)); } catch (_) {}
            const rev = await saveStore(tenant, st);
            return json({ ok: true, rev });
          } else return json({ ok: false, error: "action" }, 400);
          const rev = await saveStore(tenant, st);
          return json({ ok: true, item: p, rev });
        }
      }
      if (parts[0] === "order" && request.method === "POST" && parts.length === 1) {
        const body = await request.json().catch(() => ({}));
        const thread = String(body.thread || request.headers.get("X-LaTati-Thread") || randHex(6));
        const lines = [];
        for (const it of body.items || []) {
          const pr = st.products[it.product_id];
          if (!pr || !productAvailable(pr)) return json({ ok: false, error: "producto no disponible" }, 400);
          const mode = pr.delivery_mode || "both";
          const dtype = body.delivery_type || "pickup";
          if (mode === "delivery" && dtype === "pickup") return json({ ok: false, error: `«${pr.title}» solo se entrega a domicilio` }, 400);
          if (mode === "pickup" && dtype === "delivery") return json({ ok: false, error: `«${pr.title}» solo recogida` }, 400);
          lines.push({ product_id: pr.id, title: pr.title, qty: Number(it.qty) || 1, price: pr.price || 0, currency: pr.currency || "CUP" });
        }
        if (!lines.length) return json({ ok: false, error: "sin items" }, 400);
        st.seq_order = (st.seq_order || 0) + 1;
        const order = {
          id: randHex(8), code: `LT-${String(st.seq_order).padStart(4, "0")}`, thread,
          client_name: String(body.client_name || ""), client_phone: String(body.client_phone || ""),
          delivery_type: body.delivery_type || "pickup", delivery_address: String(body.delivery_address || ""),
          delivery_note: String(body.delivery_note || body.note || ""), note: String(body.note || ""),
          status: "requested", items: lines,
          gestor_name: st.profile.name || "", gestor_phone: st.profile.whatsapp || "",
          created_at: new Date().toISOString(), updated_at: new Date().toISOString(),
        };
        st.orders.unshift(order);
        st.messages.push({ id: randHex(6), thread, from: "system", text: `Pedido ${order.code} recibido · pendiente de confirmación`, created_at: order.created_at });
        const rev = await saveStore(tenant, st);
        return json({ ok: true, order, rev });
      }
      if (parts[0] === "order" && parts[1] && parts[2] === "status" && request.method === "POST") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        const body = await request.json().catch(() => ({}));
        const o = st.orders.find((x) => x.id === parts[1] || x.code === parts[1]);
        if (!o) return json({ ok: false, error: "not found" }, 404);
        let stt = String(body.status || "").toLowerCase();
        if (stt === "confirm" || stt === "confirmed") stt = "pending";
        if (stt === "cancel") stt = "cancelled";
        o.status = stt; o.updated_at = new Date().toISOString();
        if (stt === "cancelled") {
          o.cancel_reason = String(body.reason || body.cancel_reason || "");
          st.messages.push({ id: randHex(6), thread: o.thread, from: "system", text: `Pedido ${o.code} cancelado.` + (o.cancel_reason ? ` Motivo: ${o.cancel_reason}` : ""), created_at: o.updated_at });
        } else if (stt === "pending") {
          st.messages.push({ id: randHex(6), thread: o.thread, from: "system", text: `Pedido ${o.code} confirmado · preparando entrega`, created_at: o.updated_at });
        } else if (stt === "ready") {
          st.messages.push({ id: randHex(6), thread: o.thread, from: "system", text: `Pedido ${o.code} listo`, created_at: o.updated_at });
        }
        const rev = await saveStore(tenant, st);
        return json({ ok: true, order: o, rev });
      }
      if (parts[0] === "order" && parts[1] && request.method === "GET") {
        const thread = url.searchParams.get("thread") || request.headers.get("X-LaTati-Thread") || "";
        const o = st.orders.find((x) => x.id === parts[1] || x.code === parts[1]);
        if (!o) return json({ ok: false, error: "not found" }, 404);
        if (!auth(request, st) && thread && o.thread !== thread) return json({ ok: false, error: "not found" }, 404);
        return json({ ok: true, order: o });
      }
      if (parts[0] === "orders" && request.method === "GET") {
        const thread = url.searchParams.get("thread") || request.headers.get("X-LaTati-Thread") || "";
        let items = st.orders;
        if (!auth(request, st)) {
          if (!thread) return json({ ok: false, error: "thread" }, 400);
          items = items.filter((o) => o.thread === thread);
        }
        return json({ ok: true, items, rev: st.rev, orders_seen: st.orders_seen });
      }
      if (parts[0] === "chat") {
        if (request.method === "GET") {
          const thread = url.searchParams.get("thread") || "";
          let msgs = st.messages;
          if (thread) msgs = msgs.filter((m) => m.thread === thread);
          return json({ ok: true, items: msgs, messages: msgs });
        }
        if (request.method === "POST") {
          const body = await request.json().catch(() => ({}));
          const thread = String(body.thread || request.headers.get("X-LaTati-Thread") || "");
          if (!thread) return json({ ok: false, error: "thread" }, 400);
          const from = body.from === "gestor" ? "gestor" : "client";
          if (from === "gestor" && !auth(request, st)) return json({ ok: false, error: "auth" }, 401);
          const msg = { id: randHex(6), thread, from, text: String(body.text || "").slice(0, 2000), client_name: body.client_name || "", created_at: new Date().toISOString() };
          st.messages.push(msg);
          if (st.messages.length > 2000) st.messages = st.messages.slice(-2000);
          const rev = await saveStore(tenant, st);
          return json({ ok: true, item: msg, rev });
        }
      }
      if (parts[0] === "threads" && request.method === "GET") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        const map = {};
        for (const m of st.messages) {
          if (!m.thread) continue;
          if (!map[m.thread] || (m.created_at || "") > (map[m.thread].created_at || "")) map[m.thread] = m;
        }
        const items = Object.keys(map).map((th) => ({ thread: th, last: map[th].text, name: map[th].client_name || th, created_at: map[th].created_at }));
        items.sort((a, b) => (b.created_at || "").localeCompare(a.created_at || ""));
        return json({ ok: true, items });
      }
      if (parts[0] === "gestor" && parts[1] === "chat-clear" && request.method === "POST") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        const body = await request.json().catch(() => ({}));
        if (body.all) st.messages = [];
        else if (body.thread) st.messages = st.messages.filter((m) => m.thread !== body.thread);
        else return json({ ok: false, error: "thread or all" }, 400);
        const rev = await saveStore(tenant, st);
        return json({ ok: true, rev });
      }
      if (parts[0] === "gestor" && parts[1] === "reset" && request.method === "POST") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        const body = await request.json().catch(() => ({}));
        if (String(body.confirm || "").toUpperCase() !== "RESET") return json({ ok: false, error: "escribe confirm: RESET" }, 400);
        st.orders = []; st.messages = []; st.interests = []; st.tokens = {}; st.seq_order = 0; st.orders_seen = 0;
        const rev = await saveStore(tenant, st);
        return json({ ok: true, rev, message: "Reset OK · próximo LT-0001" });
      }
      if (parts[0] === "gestor" && parts[1] === "orders-seen" && request.method === "POST") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        st.orders_seen = st.rev;
        await saveStore(tenant, st);
        return json({ ok: true, orders_seen: st.orders_seen, rev: st.rev });
      }
      if (parts[0] === "interest" && request.method === "POST") {
        const body = await request.json().catch(() => ({}));
        const it = { id: randHex(6), product_id: body.product_id || "", client_name: body.client_name || "", client_phone: body.client_phone || "", thread: body.thread || "", text: body.text || "", seen: false, created_at: new Date().toISOString() };
        st.interests.unshift(it);
        const rev = await saveStore(tenant, st);
        return json({ ok: true, item: it, rev });
      }
      if (parts[0] === "gestor" && parts[1] === "interests") {
        if (!auth(request, st)) return json({ ok: false, error: "auth" }, 401);
        if (request.method === "GET") return json({ ok: true, items: st.interests });
        if (request.method === "POST") {
          const body = await request.json().catch(() => ({}));
          if (body.delete && body.id) st.interests = st.interests.filter((x) => x.id !== body.id);
          else if (body.id && body.seen) { const x = st.interests.find((i) => i.id === body.id); if (x) x.seen = true; }
          const rev = await saveStore(tenant, st);
          return json({ ok: true, rev });
        }
      }
      if (parts[0] === "events") {
        const enc = new TextEncoder();
        const stream = new ReadableStream({
          async start(controller) {
            controller.enqueue(enc.encode(`event: hello\ndata: ${JSON.stringify({ ok: true, rev: st.rev })}\n\n`));
            for (let i = 0; i < 30; i++) {
              await new Promise((r) => setTimeout(r, 2000));
              try {
                const cur = await loadStore(tenant);
                controller.enqueue(enc.encode(`event: ping\ndata: ${JSON.stringify({ rev: cur.rev })}\n\n`));
              } catch (_) { break; }
            }
            controller.close();
          },
        });
        return new Response(stream, { headers: { "Content-Type": "text/event-stream", "Cache-Control": "no-cache", ...CORS } });
      }
      return json({ ok: false, error: "not found", path: parts.join("/") }, 404);
    } catch (e) {
      return json({ ok: false, error: String(e && e.message ? e.message : e) }, 500);
    }
  },
};
