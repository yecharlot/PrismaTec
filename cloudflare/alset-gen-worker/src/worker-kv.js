/**
 * Alset Network — multi-gen edge on Cloudflare KV (API-deployable).
 */
const MAX_FINDINGS = 48;
const MAX_BODY = 32 * 1024;

export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    const path = url.pathname;
    if (request.method === "OPTIONS") {
      return new Response(null, {
        headers: {
          "Access-Control-Allow-Origin": "*",
          "Access-Control-Allow-Methods": "GET,POST,OPTIONS",
          "Access-Control-Allow-Headers": "Content-Type",
        },
      });
    }
    try {
      if (path === "/" || path === "/api/network/info") return networkHome(url.origin);
      if (path === "/api/network/gens") return listGens(env);
      if ((path === "/api/network/spawn" || path === "/api/network/dispatch") && request.method === "POST") {
        return spawnGen(request, env, url.origin);
      }
      const m = path.match(/^\/g\/([^/]+)(\/.*)?$/);
      if (m) {
        const key = normalizeKey(decodeURIComponent(m[1]));
        const rest = m[2] || "/";
        return handleGen(request, env, key, rest, url.origin);
      }
      return json({ ok: false, error: "not found" }, 404);
    } catch (e) {
      return json({ ok: false, error: String(e) }, 500);
    }
  },
};

function normalizeKey(k) {
  k = String(k || "").trim().toLowerCase();
  if (!k) return "";
  if (!k.endsWith(".ans")) k += ".ans";
  return k;
}

async function networkHome(origin) {
  return html(`<!DOCTYPE html><html lang="es"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>Alset Network</title>
<style>body{margin:0;font-family:system-ui;background:#070b14;color:#e8eefc;min-height:100vh;padding:2rem}
.card{max-width:640px;margin:0 auto;background:#121a2b;border:1px solid #243044;border-radius:16px;padding:2rem}
h1{color:#5eead4}code{color:#a5f3fc;font-size:.85rem}</style></head>
<body><div class="card"><div style="color:#5eead4;font-size:.75rem">ALSET NETWORK · CLOUDFLARE</div>
<h1>Red de genes</h1>
<p>Torrente de borde. Mind despacha células aquí.</p>
<p><code>${esc(origin)}</code></p>
<ul><li>POST /api/network/dispatch</li><li>GET /api/network/gens</li><li>/g/{key}/</li></ul>
</div></body></html>`);
}

async function listGens(env) {
  const list = await env.GEN_KV.list({ prefix: "meta:" });
  const gens = [];
  for (const k of list.keys) {
    const v = await env.GEN_KV.get(k.name, "json");
    if (v) gens.push(v);
  }
  return json({ ok: true, count: gens.length, gens });
}

async function spawnGen(request, env, origin) {
  const body = await request.json().catch(() => ({}));
  const key = normalizeKey(body.key || body.gen || "");
  if (!key) return json({ ok: false, error: "key requerida" }, 400);
  const short = key.replace(/\.ans$/, "");
  const reach = `${origin}/g/${encodeURIComponent(short)}`;
  const meta = {
    key,
    reach,
    package_cid: body.package_cid || "",
    root_cid: body.root_cid || "",
    mission: body.mission || "",
    description: body.description || "",
    spawned_at: Date.now(),
  };
  await env.GEN_KV.put("meta:" + key, JSON.stringify(meta));
  const existing = (await env.GEN_KV.get("data:" + key, "json")) || { findings: [] };
  if (body.package_cid) existing.package_cid = body.package_cid;
  if (body.root_cid) existing.root_cid = body.root_cid;
  if (body.mission) existing.mission = body.mission;
  if (Array.isArray(body.findings) && body.findings.length) {
    existing.findings = body.findings.slice(-MAX_FINDINGS);
  }
  if (body.last_hallazgo && typeof body.last_hallazgo === "string") {
    existing.last_hallazgo = body.last_hallazgo;
    if (!existing.findings || !existing.findings.length) {
      existing.findings = [{ url: "local-seed", title: "hallazgo local", status: 200, snippet: body.last_hallazgo }];
    }
  }
  existing.key = key;
  await env.GEN_KV.put("data:" + key, JSON.stringify(existing));

  let announced = null;
  const mind = (env.ANNOUNCE_URL || "").replace(/\/$/, "");
  if (mind) {
    try {
      const resp = await fetch(mind + "/api/gen/announce", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key, http_base: reach, root_cid: body.root_cid || "", findings: (existing.findings || []).length }),
      });
      announced = { ok: resp.ok, status: resp.status };
    } catch (e) {
      announced = { ok: false, error: String(e) };
    }
  }
  return json({ ok: true, key, reach, package_cid: body.package_cid || "", announced, note: "Gen en red Alset Cloudflare" });
}

async function handleGen(request, env, key, path, origin) {
  let data = (await env.GEN_KV.get("data:" + key, "json")) || { key, findings: [] };
  if (path === "/api/seed" && request.method === "POST") {
    const body = await request.json().catch(() => ({}));
    if (body.package_cid) data.package_cid = body.package_cid;
    if (body.root_cid) data.root_cid = body.root_cid;
    if (body.mission) data.mission = body.mission;
    if (Array.isArray(body.findings) && body.findings.length) {
      data.findings = body.findings.slice(-MAX_FINDINGS);
    }
    if (body.last_hallazgo && typeof body.last_hallazgo === "string") {
      data.last_hallazgo = body.last_hallazgo;
      if (!data.findings || !data.findings.length) {
        data.findings = [{ url: "local-seed", title: "hallazgo local", status: 200, snippet: body.last_hallazgo }];
      }
    }
    data.key = key;
    await env.GEN_KV.put("data:" + key, JSON.stringify(data));
    const short = key.replace(/\.ans$/, "");
    const meta = { key, reach: `${origin}/g/${encodeURIComponent(short)}`, package_cid: data.package_cid || "", root_cid: data.root_cid || "", mission: data.mission || "", spawned_at: Date.now() };
    await env.GEN_KV.put("meta:" + key, JSON.stringify(meta));
    return json({ ok: true, key, seeded: true });
  }
  if (path === "/health") return json({ ok: true, key, mode: "alset-kv" });
  if (path === "/" || path === "") {
    return html(servicePage(key, data.root_cid || "", data.findings || []));
  }
  if (path === "/api/info") {
    return json({
      ok: true, key, mode: "alset-network-cloudflare",
      root_cid: data.root_cid || "", package_cid: data.package_cid || "",
      mission: data.mission || "", findings_count: (data.findings || []).length,
    });
  }
  if (path === "/api/findings") {
    const f = data.findings || [];
    return json({ ok: true, count: f.length, findings: f });
  }
  if (path === "/api/dialogue" && request.method === "POST") {
    const body = await request.json().catch(() => ({}));
    const text = (body.text || body.stimulus || "").toLowerCase();
    const findings = data.findings || [];
    return json({
      ok: true, key,
      voice: composeVoice(key, text, findings, data.root_cid || ""),
      findings_count: findings.length, mode: "alset-network-cloudflare",
    });
  }
  if (path === "/api/explore" && request.method === "POST") {
    const body = await request.json().catch(() => ({}));
    const report = await exploreURL(body.url || "", body.mission || "explore");
    report.key = key;
    if (report.url) {
      data.findings = data.findings || [];
      data.findings.push(report);
      if (data.findings.length > MAX_FINDINGS) data.findings = data.findings.slice(-MAX_FINDINGS);
      await env.GEN_KV.put("data:" + key, JSON.stringify(data));
    }
    return json(report);
  }
  if (path === "/api/announce-now" && request.method === "POST") {
    const mind = (env.ANNOUNCE_URL || "").replace(/\/$/, "");
    if (!mind) return json({ ok: false, error: "ANNOUNCE_URL missing" }, 400);
    const short = key.replace(/\.ans$/, "");
    const reach = `${origin}/g/${encodeURIComponent(short)}`;
    try {
      const resp = await fetch(mind + "/api/gen/announce", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ key, http_base: reach, root_cid: data.root_cid || "", findings: (data.findings || []).length }),
      });
      return json({ ok: resp.ok, status: resp.status, reach });
    } catch (e) {
      return json({ ok: false, error: String(e) }, 502);
    }
  }
  return new Response("not found", { status: 404 });
}

function composeVoice(key, s, findings, root) {
  if (!s || s.includes("quién eres") || s.includes("quien eres") || s.includes("identidad")) {
    return `Soy ${key} en la red Alset (Cloudflare). Root ${(root || "").slice(0, 18)}… Hallazgos ${findings.length}. Oficio en el edge global.`;
  }
  if (s.includes("sabes") || s.includes("hallazgo") || s.includes("explor") || s.includes("viste")) {
    if (!findings.length) return "Sin hallazgos aún. Pide explorar una URL pública.";
    return `Hallazgos (${findings.length}):\n` + findings.slice(-3).map((f) => {
      const sn = f.snippet || f.title || f.url || "";
      return `- ${String(sn).slice(0, 180)}`;
    }).join("\n");
  }
  if (s.includes("estado") || s.includes("oficio") || s.includes("misión") || s.includes("mision")) {
    return `Oficio CF: key=${key} hallazgos=${findings.length}.`;
  }
  return `Semilla ${key} en red Alset Cloudflare. Hallazgos ${findings.length}.`;
}

async function exploreURL(raw, mission) {
  const u = (raw || "").trim();
  if (!u) return { ok: false, error: "url requerida" };
  let parsed;
  try { parsed = new URL(u); } catch { return { ok: false, error: "url inválida" }; }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return { ok: false, error: "solo http/https" };
  const host = parsed.hostname.toLowerCase();
  if (host === "localhost" || host.endsWith(".local") || host === "127.0.0.1") return { ok: false, error: "host no permitido" };
  const start = Date.now();
  try {
    const resp = await fetch(u, { headers: { "User-Agent": "Alset-Network-CF/1.0" }, redirect: "follow" });
    const buf = await resp.arrayBuffer();
    const slice = buf.byteLength > MAX_BODY ? buf.slice(0, MAX_BODY) : buf;
    const text = new TextDecoder().decode(slice);
    const title = (text.match(/<title[^>]*>([^<]*)<\/title>/i) || [])[1] || "";
    const snippet = text.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim().slice(0, 400);
    return { ok: resp.status >= 200 && resp.status < 400, url: u, mission, status: resp.status, title: title.trim(), snippet, latency_ms: Date.now() - start, ts: new Date().toISOString() };
  } catch (e) {
    return { ok: false, url: u, mission, error: String(e), latency_ms: Date.now() - start };
  }
}

function servicePage(key, root, findings) {
  return `<!DOCTYPE html><html lang="es"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>${esc(key)}</title>
<style>body{margin:0;font-family:system-ui;background:#070b14;color:#e8eefc;min-height:100vh;display:flex;align-items:center;justify-content:center}
.card{background:#121a2b;border:1px solid #243044;border-radius:16px;padding:2rem;max-width:440px}
h1{color:#5eead4;font-size:1.15rem}</style></head>
<body><div class="card"><div style="color:#5eead4;font-size:.7rem">ALSET GEN · RED CLOUDFLARE</div>
<h1>${esc(key)}</h1><p>Célula en la red Alset (edge global).</p>
<p style="font-size:.85rem;opacity:.8">Root: ${esc((root || "").slice(0, 24))}</p>
<p>Hallazgos: ${findings.length}</p></div></body></html>`;
}
function esc(s) { return String(s).replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;"); }
function json(obj, status = 200) {
  return new Response(JSON.stringify(obj), { status, headers: { "Content-Type": "application/json", "Access-Control-Allow-Origin": "*" } });
}
function html(s) { return new Response(s, { headers: { "Content-Type": "text/html; charset=utf-8" } }); }
