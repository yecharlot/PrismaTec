/**
 * Alset Network on Cloudflare — coexistence of many gens + dispatch from Mind.
 *
 * Routing:
 *   GET  /                         network home
 *   GET  /api/network/info
 *   GET  /api/network/gens
 *   POST /api/network/spawn        { key, package_cid?, root_cid?, mission? }
 *   POST /api/network/dispatch     same as spawn (alias from PrismaTec)
 *   /g/:key/*                      → Durable Object of that gen
 *   /g/:key/                       service page
 *   /g/:key/api/info|dialogue|explore|findings|announce-now
 */

const MAX_FINDINGS = 48;
const MAX_BODY = 32 * 1024;

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;

    // CORS preflight
    if (request.method === "OPTIONS") {
      return new Response(null, {
        headers: {
          "Access-Control-Allow-Origin": "*",
          "Access-Control-Allow-Methods": "GET,POST,OPTIONS",
          "Access-Control-Allow-Headers": "Content-Type",
        },
      });
    }

    // Network control plane
    if (path === "/" || path === "/api/network/info") {
      return networkHome(env, request.url);
    }
    if (path === "/api/network/gens") {
      return listGens(env);
    }
    if (
      (path === "/api/network/spawn" || path === "/api/network/dispatch") &&
      request.method === "POST"
    ) {
      return spawnGen(request, env, request.url);
    }

    // Node persistence store (blocks + kv) → single Durable Object
    if (path.startsWith("/api/store/")) {
      return storeProxy(request, env, path);
    }

    // Per-gen: /g/{key}/...
    const m = path.match(/^\/g\/([^/]+)(\/.*)?$/);
    if (m) {
      const key = normalizeKey(decodeURIComponent(m[1]));
      const rest = m[2] || "/";
      const id = env.GEN_DO.idFromName(key);
      const stub = env.GEN_DO.get(id);
      const inner = new URL(request.url);
      inner.pathname = rest === "" ? "/" : rest;
      const headers = new Headers(request.headers);
      headers.set("X-Alset-Gen-Key", key);
      const req2 = new Request(inner.toString(), {
        method: request.method,
        headers,
        body: request.method !== "GET" && request.method !== "HEAD" ? request.body : null,
      });
      return stub.fetch(req2);
    }

    return json({ ok: false, error: "not found", hint: "use /g/{key}/ or /api/network/*" }, 404);
  },
};

async function networkHome(env, workerURL) {
  const base = new URL(workerURL).origin;
  return html(`<!DOCTYPE html><html lang="es"><head><meta charset="utf-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Alset Network · Cloudflare</title>
<style>
body{margin:0;font-family:system-ui;background:#070b14;color:#e8eefc;min-height:100vh;padding:2rem}
.card{max-width:640px;margin:0 auto;background:#121a2b;border:1px solid #243044;border-radius:16px;padding:2rem}
h1{color:#5eead4;font-size:1.4rem;margin:0 0 .5rem}
a{color:#7dd3fc} code{color:#a5f3fc;font-size:.85rem}
</style></head><body><div class="card">
<div style="color:#5eead4;font-size:.75rem;letter-spacing:.08em">ALSET NETWORK · EDGE</div>
<h1>Red de genes en Cloudflare</h1>
<p>Torrente de borde donde coexisten células Alset-Gen. Mind puede <strong>crear</strong> y <strong>despachar</strong> genes aquí.</p>
<p>Base: <code>${esc(base)}</code></p>
<ul>
<li><code>POST /api/network/spawn</code> — crear / despertar gen</li>
<li><code>GET /api/network/gens</code> — listado</li>
<li><code>/g/{key}/</code> — célula residente</li>
</ul>
</div></body></html>`);
}

async function listGens(env) {
  // Registry is best-effort in KV if bound; else empty + note
  if (env.GEN_REGISTRY) {
    const list = await env.GEN_REGISTRY.list({ prefix: "gen:" });
    const gens = [];
    for (const k of list.keys) {
      const v = await env.GEN_REGISTRY.get(k.name, "json");
      if (v) gens.push(v);
    }
    return json({ ok: true, count: gens.length, gens });
  }
  return json({
    ok: true,
    count: 0,
    gens: [],
    note: "Sin KV GEN_REGISTRY: los gens existen como Durable Objects al primer /g/{key}/ o spawn",
  });
}

async function spawnGen(request, env, workerURL) {
  const body = await request.json().catch(() => ({}));
  const key = normalizeKey(body.key || body.gen || "");
  if (!key) return json({ ok: false, error: "key requerida" }, 400);

  const id = env.GEN_DO.idFromName(key);
  const stub = env.GEN_DO.get(id);
  const base = new URL(workerURL).origin;
  const reach = `${base}/g/${encodeURIComponent(key.replace(/\.ans$/, ""))}`;

  // Seed DO
  const seedReq = new Request(base + "/api/seed", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Alset-Gen-Key": key },
    body: JSON.stringify({
      package_cid: body.package_cid || "",
      root_cid: body.root_cid || "",
      mission: body.mission || "",
      description: body.description || "",
    }),
  });
  await stub.fetch(seedReq);

  // Registry
  const record = {
    key,
    reach,
    package_cid: body.package_cid || "",
    root_cid: body.root_cid || "",
    mission: body.mission || "",
    spawned_at: Date.now(),
  };
  if (env.GEN_REGISTRY) {
    await env.GEN_REGISTRY.put("gen:" + key, JSON.stringify(record));
  }

  // Announce to Mind/PrismaTec
  let announced = null;
  const mind = (env.ANNOUNCE_URL || "").replace(/\/$/, "");
  if (mind) {
    try {
      const resp = await fetch(mind + "/api/gen/announce", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          key,
          http_base: reach,
          root_cid: body.root_cid || "",
          findings: 0,
        }),
      });
      announced = { status: resp.status, ok: resp.ok };
    } catch (e) {
      announced = { ok: false, error: String(e) };
    }
  }

  return json({
    ok: true,
    key,
    reach,
    package_cid: body.package_cid || "",
    announced,
    note: "Gen despachado a la red Alset en Cloudflare",
  });
}

export class AlsetGenDO {
  constructor(state, env) {
    this.state = state;
    this.env = env;
  }

  async fetch(request) {
    const url = new URL(request.url);
    const path = url.pathname;
    const key = normalizeKey(
      request.headers.get("X-Alset-Gen-Key") || (await this.state.storage.get("key")) || "cell.ans"
    );

    if (path === "/api/seed" && request.method === "POST") {
      const body = await request.json().catch(() => ({}));
      await this.state.storage.put("key", key);
      if (body.package_cid) await this.state.storage.put("package_cid", body.package_cid);
      if (body.root_cid) await this.state.storage.put("root_cid", body.root_cid);
      if (body.mission) await this.state.storage.put("mission", body.mission);
      if (body.description) await this.state.storage.put("description", body.description);
      await this.state.storage.put("updated_at", Date.now());
      return json({ ok: true, key, seeded: true });
    }

    if (path === "/health") return json({ ok: true, key, mode: "alset-network-do" });

    if (path === "/" || path === "") {
      const findings = (await this.state.storage.get("findings")) || [];
      const root = (await this.state.storage.get("root_cid")) || this.env.ROOT_CID || "";
      return html(servicePage(key, root, findings));
    }

    if (path === "/api/info") {
      const findings = (await this.state.storage.get("findings")) || [];
      return json({
        ok: true,
        key,
        mode: "alset-network-cloudflare",
        root_cid: (await this.state.storage.get("root_cid")) || "",
        package_cid: (await this.state.storage.get("package_cid")) || "",
        mission: (await this.state.storage.get("mission")) || "",
        findings_count: findings.length,
      });
    }

    if (path === "/api/findings") {
      const findings = (await this.state.storage.get("findings")) || [];
      return json({ ok: true, count: findings.length, findings });
    }

    if (path === "/api/dialogue" && request.method === "POST") {
      const body = await request.json().catch(() => ({}));
      const text = (body.text || body.stimulus || "").toLowerCase();
      const findings = (await this.state.storage.get("findings")) || [];
      const root = (await this.state.storage.get("root_cid")) || "";
      return json({
        ok: true,
        key,
        voice: composeVoice(key, text, findings, root),
        findings_count: findings.length,
        mode: "alset-network-cloudflare",
        remote_http: true,
      });
    }

    if (path === "/api/explore" && request.method === "POST") {
      const body = await request.json().catch(() => ({}));
      const report = await exploreURL(body.url || "", body.mission || "explore");
      report.key = key;
      if (report.url) {
        let findings = (await this.state.storage.get("findings")) || [];
        findings.push(report);
        if (findings.length > MAX_FINDINGS) findings = findings.slice(-MAX_FINDINGS);
        await this.state.storage.put("findings", findings);
      }
      return json(report);
    }

    if (path === "/api/announce-now" && request.method === "POST") {
      const mind = (this.env.ANNOUNCE_URL || "").replace(/\/$/, "");
      if (!mind) return json({ ok: false, error: "ANNOUNCE_URL missing" }, 400);
      const origin = new URL(request.url).origin;
      const reach = `${origin}/g/${encodeURIComponent(key.replace(/\.ans$/, ""))}`;
      try {
        const resp = await fetch(mind + "/api/gen/announce", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            key,
            http_base: reach,
            root_cid: (await this.state.storage.get("root_cid")) || "",
            findings: ((await this.state.storage.get("findings")) || []).length,
          }),
        });
        return json({ ok: resp.ok, status: resp.status, reach });
      } catch (e) {
        return json({ ok: false, error: String(e) }, 502);
      }
    }

    return new Response("not found", { status: 404 });
  }
}

function normalizeKey(k) {
  k = String(k || "")
    .trim()
    .toLowerCase();
  if (!k) return "";
  if (!k.endsWith(".ans")) k += ".ans";
  return k;
}

function composeVoice(key, s, findings, root) {
  if (!s || s.includes("quién eres") || s.includes("quien eres") || s.includes("identidad")) {
    return `Soy ${key} en la red Alset (Cloudflare). Root ${(root || "").slice(0, 18)}… Hallazgos ${findings.length}. Ejerzo oficio en el edge global.`;
  }
  if (s.includes("sabes") || s.includes("hallazgo") || s.includes("explor") || s.includes("viste")) {
    if (!findings.length) return "Sin hallazgos aún en el edge. Pide explorar una URL.";
    return (
      `Hallazgos (${findings.length}) en Cloudflare:\n` +
      findings
        .slice(-3)
        .map((f) => `- ${f.url} · ${f.title || ""} · ${f.status}`)
        .join("\n")
    );
  }
  if (s.includes("estado") || s.includes("oficio") || s.includes("misión") || s.includes("mision")) {
    return `Oficio en red Alset CF: key=${key} hallazgos=${findings.length}.`;
  }
  return `Semilla ${key} en red Alset Cloudflare. Hallazgos ${findings.length}.`;
}

async function exploreURL(raw, mission = "explore") {
  const u = (raw || "").trim();
  if (!u) return { ok: false, error: "url requerida" };
  let parsed;
  try {
    parsed = new URL(u);
  } catch {
    return { ok: false, error: "url inválida" };
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    return { ok: false, error: "solo http/https" };
  }
  const host = parsed.hostname.toLowerCase();
  if (host === "localhost" || host.endsWith(".local") || host === "127.0.0.1") {
    return { ok: false, error: "host no permitido" };
  }
  const start = Date.now();
  try {
    const resp = await fetch(u, {
      headers: { "User-Agent": "Alset-Network-CF/1.0" },
      redirect: "follow",
    });
    const buf = await resp.arrayBuffer();
    const slice = buf.byteLength > MAX_BODY ? buf.slice(0, MAX_BODY) : buf;
    const text = new TextDecoder().decode(slice);
    const title = (text.match(/<title[^>]*>([^<]*)<\/title>/i) || [])[1] || "";
    const snippet = text.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim().slice(0, 400);
    return {
      ok: resp.status >= 200 && resp.status < 400,
      url: u,
      mission,
      status: resp.status,
      title: title.trim(),
      snippet,
      latency_ms: Date.now() - start,
      ts: new Date().toISOString(),
    };
  } catch (e) {
    return { ok: false, url: u, mission, error: String(e), latency_ms: Date.now() - start };
  }
}

function servicePage(key, root, findings) {
  return `<!DOCTYPE html><html lang="es"><head><meta charset="utf-8"/>
<meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>${esc(key)}</title>
<style>body{margin:0;font-family:system-ui;background:#070b14;color:#e8eefc;min-height:100vh;display:flex;align-items:center;justify-content:center}
.card{background:#121a2b;border:1px solid #243044;border-radius:16px;padding:2rem;max-width:440px}
h1{color:#5eead4;font-size:1.15rem}</style></head>
<body><div class="card"><div style="color:#5eead4;font-size:.7rem">ALSET GEN · RED CLOUDFLARE</div>
<h1>${esc(key)}</h1>
<p>Célula en la red Alset (edge global).</p>
<p style="font-size:.85rem;opacity:.8">Root: ${esc((root || "").slice(0, 24))}</p>
<p>Hallazgos: ${findings.length}</p></div></body></html>`;
}

function esc(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function json(obj, status = 200) {
  return new Response(JSON.stringify(obj), {
    status,
    headers: {
      "Content-Type": "application/json",
      "Access-Control-Allow-Origin": "*",
    },
  });
}

function html(s) {
  return new Response(s, { headers: { "Content-Type": "text/html; charset=utf-8" } });
}


async function storeProxy(request, env, path) {
  const secret = env.STORE_SECRET || "";
  if (secret) {
    const h = request.headers.get("X-Alset-Store-Secret") || "";
    if (h !== secret) {
      return json({ ok: false, error: "unauthorized" }, 401);
    }
  }
  if (!env.STORE_DO) {
    return json({ ok: false, error: "STORE_DO not bound — deploy wrangler with AlsetStoreDO" }, 503);
  }
  const id = env.STORE_DO.idFromName("alset-node-store");
  const stub = env.STORE_DO.get(id);
  return stub.fetch(request);
}

/**
 * AlsetStoreDO — durable KV + content-addressed blocks for PrismaTec node.
 * Paths (same request forwarded):
 *   PUT/GET/DELETE /api/store/kv?key=
 *   PUT/GET        /api/store/block?cid=
 *   POST           /api/store/blocks   { "cid": "base64data", ... }
 *   GET            /api/store/blocks   → map cid -> base64
 *   GET            /api/store/info
 */
export class AlsetStoreDO {
  constructor(state, env) {
    this.state = state;
    this.env = env;
  }

  async fetch(request) {
    const url = new URL(request.url);
    const path = url.pathname;

    if (path === "/api/store/info" && request.method === "GET") {
      const meta = (await this.state.storage.get("meta")) || { blocks: 0, kv: 0 };
      return json({ ok: true, species: "AlsetStoreDO", meta });
    }

    if (path === "/api/store/kv") {
      const key = url.searchParams.get("key") || "";
      if (!key) return json({ ok: false, error: "key required" }, 400);
      if (request.method === "GET") {
        const v = await this.state.storage.get("kv:" + key);
        if (v === undefined || v === null) return json({ ok: false, error: "not found" }, 404);
        return json({ ok: true, key, data: v });
      }
      if (request.method === "PUT" || request.method === "POST") {
        const body = await request.json().catch(() => ({}));
        const data = body.data;
        if (typeof data !== "string") return json({ ok: false, error: "data base64/string required" }, 400);
        await this.state.storage.put("kv:" + key, data);
        await this.bumpMeta("kv");
        return json({ ok: true, key });
      }
      if (request.method === "DELETE") {
        await this.state.storage.delete("kv:" + key);
        return json({ ok: true, key, deleted: true });
      }
    }

    if (path === "/api/store/block") {
      const cid = url.searchParams.get("cid") || "";
      if (!cid) return json({ ok: false, error: "cid required" }, 400);
      if (request.method === "GET") {
        const v = await this.state.storage.get("b:" + cid);
        if (v === undefined || v === null) return json({ ok: false, error: "not found" }, 404);
        return json({ ok: true, cid, data: v });
      }
      if (request.method === "PUT" || request.method === "POST") {
        const body = await request.json().catch(() => ({}));
        const data = body.data;
        if (typeof data !== "string") return json({ ok: false, error: "data base64 required" }, 400);
        await this.state.storage.put("b:" + cid, data);
        await this.bumpMeta("blocks");
        return json({ ok: true, cid });
      }
    }

    if (path === "/api/store/blocks" && request.method === "POST") {
      const body = await request.json().catch(() => ({}));
      const blocks = body.blocks || body;
      if (typeof blocks !== "object") return json({ ok: false, error: "blocks map required" }, 400);
      let n = 0;
      for (const [cid, data] of Object.entries(blocks)) {
        if (typeof data === "string" && cid) {
          await this.state.storage.put("b:" + cid, data);
          n++;
        }
      }
      await this.bumpMeta("blocks", n);
      return json({ ok: true, saved: n });
    }

    if (path === "/api/store/blocks" && request.method === "GET") {
      // list is expensive; return only if small — for node LoadBlocks we need full map
      const all = await this.state.storage.list({ prefix: "b:" });
      const out = {};
      for (const [k, v] of all) {
        out[k.slice(2)] = v;
      }
      return json({ ok: true, blocks: out, count: Object.keys(out).length });
    }

    return json({ ok: false, error: "store path not found" }, 404);
  }

  async bumpMeta(kind, by = 1) {
    const meta = (await this.state.storage.get("meta")) || { blocks: 0, kv: 0 };
    if (kind === "blocks") meta.blocks = (meta.blocks || 0) + by;
    if (kind === "kv") meta.kv = (meta.kv || 0) + 1;
    await this.state.storage.put("meta", meta);
  }
}

