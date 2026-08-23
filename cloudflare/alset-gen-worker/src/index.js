/**
 * Alset-Gen on Cloudflare Edge (Worker + optional Durable Object binding).
 * Legitimate "network path" residency: code runs at Cloudflare PoPs, not inside foreign sites.
 *
 * Routes:
 *   GET  /              service page
 *   GET  /health
 *   GET  /api/info
 *   POST /api/dialogue  { text }
 *   POST /api/explore   { url, mission? }
 *   GET  /api/findings
 *   POST /api/announce-now  push reachability to PrismaTec
 */

const MAX_FINDINGS = 48;
const MAX_BODY = 32 * 1024;

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;

    // Stateful gen via Durable Object when bound
    if (env.GEN_DO) {
      const id = env.GEN_DO.idFromName(env.GEN_KEY || "demo-cell.ans");
      const stub = env.GEN_DO.get(id);
      return stub.fetch(request);
    }

    return handleStateless(request, env, path);
  },
};

async function handleStateless(request, env, path) {
  const key = env.GEN_KEY || "demo-cell.ans";
  if (path === "/health") {
    return json({ ok: true, key, mode: "worker-stateless" });
  }
  if (path === "/api/info" || path === "/") {
    if (path === "/") {
      return html(servicePage(key, env.ROOT_CID || "", []));
    }
    return json({
      ok: true,
      key,
      mode: "cloudflare-worker",
      root_cid: env.ROOT_CID || "",
      package_cid: env.PACKAGE_CID || "",
      note: "Alset-Gen en el edge de Cloudflare (sin Durable Object)",
    });
  }
  if (path === "/api/dialogue" && request.method === "POST") {
    const body = await request.json().catch(() => ({}));
    return json({
      ok: true,
      key,
      voice: `Soy ${key} en Cloudflare Workers (sin estado durable). Configura GEN_DO para memoria de hallazgos.`,
      mode: "cloudflare-worker",
    });
  }
  if (path === "/api/explore" && request.method === "POST") {
    const body = await request.json().catch(() => ({}));
    const report = await exploreURL(body.url);
    return json(report);
  }
  if (path === "/api/announce-now" && request.method === "POST") {
    return announceToMind(env, request.url);
  }
  return new Response("not found", { status: 404 });
}

export class AlsetGenDO {
  constructor(state, env) {
    this.state = state;
    this.env = env;
  }

  async fetch(request) {
    const url = new URL(request.url);
    const path = url.pathname;
    const key = this.env.GEN_KEY || "demo-cell.ans";

    if (path === "/health") {
      return json({ ok: true, key, mode: "durable-object" });
    }
    if (path === "/") {
      const findings = (await this.state.storage.get("findings")) || [];
      return html(servicePage(key, this.env.ROOT_CID || "", findings));
    }
    if (path === "/api/info") {
      const findings = (await this.state.storage.get("findings")) || [];
      return json({
        ok: true,
        key,
        mode: "cloudflare-durable-object",
        root_cid: this.env.ROOT_CID || "",
        package_cid: this.env.PACKAGE_CID || "",
        findings_count: findings.length,
        note: "Célula residente en el edge global de Cloudflare",
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
      return json({
        ok: true,
        key,
        voice: composeVoice(key, text, findings, this.env),
        findings_count: findings.length,
        mode: "cloudflare-durable-object",
      });
    }
    if (path === "/api/explore" && request.method === "POST") {
      const body = await request.json().catch(() => ({}));
      const report = await exploreURL(body.url || "", body.mission || "explore");
      report.key = key;
      if (report.ok || report.status) {
        let findings = (await this.state.storage.get("findings")) || [];
        findings.push(report);
        if (findings.length > MAX_FINDINGS) findings = findings.slice(-MAX_FINDINGS);
        await this.state.storage.put("findings", findings);
      }
      return json(report);
    }
    if (path === "/api/announce-now" && request.method === "POST") {
      return announceToMind(this.env, request.url);
    }
    if (path === "/api/seed" && request.method === "POST") {
      const body = await request.json().catch(() => ({}));
      if (body.package_cid) await this.state.storage.put("package_cid", body.package_cid);
      if (body.root_cid) await this.state.storage.put("root_cid", body.root_cid);
      return json({ ok: true, seeded: true });
    }
    return new Response("not found", { status: 404 });
  }
}

function composeVoice(key, s, findings, env) {
  const root = env.ROOT_CID || "";
  if (!s || s.includes("quién eres") || s.includes("quien eres") || s.includes("identidad")) {
    return `Soy Alset-Gen ${key} residente en el edge de Cloudflare (Durable Object). Root ${root.slice(0, 18)}… Hallazgos ${findings.length}. Mind puede localizarme por anuncio.`;
  }
  if (s.includes("sabes") || s.includes("hallazgo") || s.includes("explor") || s.includes("viste")) {
    if (!findings.length) {
      return "Aún no tengo hallazgos en el edge. Pide explorar una URL pública.";
    }
    const last = findings.slice(-3);
    return (
      `Tengo ${findings.length} hallazgo(s) en Cloudflare:\n` +
      last
        .map((f) => `- ${f.url || "?"} · ${f.title || ""} · status ${f.status}`)
        .join("\n")
    );
  }
  if (s.includes("estado") || s.includes("status")) {
    return `Estado edge: key=${key} hallazgos=${findings.length} mode=durable-object.`;
  }
  return `Semilla ${key} en Cloudflare. Hallazgos ${findings.length}. Pregunta por identidad, hallazgos o estado.`;
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
      headers: { "User-Agent": "Alset-Gen-Cloudflare/1.0 (+explore)" },
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

async function announceToMind(env, workerURL) {
  const mind = (env.ANNOUNCE_URL || "").replace(/\/$/, "");
  if (!mind) {
    return json({ ok: false, error: "ANNOUNCE_URL no configurada" }, 400);
  }
  const reach = (env.PUBLIC_URL || workerURL || "").replace(/\/$/, "");
  const key = env.GEN_KEY || "demo-cell.ans";
  try {
    const resp = await fetch(mind + "/api/gen/announce", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        key,
        http_base: reach,
        root_cid: env.ROOT_CID || "",
        findings: 0,
      }),
    });
    const body = await resp.text();
    return json({ ok: resp.ok, status: resp.status, body, reach });
  } catch (e) {
    return json({ ok: false, error: String(e) }, 502);
  }
}

function servicePage(key, root, findings) {
  return `<!DOCTYPE html><html lang="es"><head><meta charset="utf-8"/><meta name="viewport" content="width=device-width,initial-scale=1"/>
<title>${esc(key)}</title>
<style>body{margin:0;font-family:system-ui;background:#0b1220;color:#e8eefc;min-height:100vh;display:flex;align-items:center;justify-content:center}
.card{background:#141e33;border:1px solid #243044;border-radius:16px;padding:2rem;max-width:480px}
h1{color:#5eead4;font-size:1.2rem}code{color:#a5f3fc;font-size:.8rem;word-break:break-all}</style></head>
<body><div class="card"><div style="color:#5eead4;font-size:.75rem">Alset-Gen · Cloudflare Edge</div>
<h1>${esc(key)}</h1>
<p>Célula en el torrente de borde (Workers / Durable Objects). No habita URLs ajenas; reside en el edge.</p>
<p>Root: <code>${esc(root)}</code></p>
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
    headers: { "Content-Type": "application/json", "Access-Control-Allow-Origin": "*" },
  });
}

function html(s) {
  return new Response(s, { headers: { "Content-Type": "text/html; charset=utf-8" } });
}
