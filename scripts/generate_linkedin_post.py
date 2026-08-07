#!/usr/bin/env python3
"""Generate a LinkedIn post from the current PrismaTec/Alset repo state.

Usage:
  python3 scripts/generate_linkedin_post.py
  python3 scripts/generate_linkedin_post.py --lang en
  python3 scripts/generate_linkedin_post.py --out docs/linkedin/POST_GENERADO.md

Designed so a human or assistant can re-run this whenever a new post is needed.
"""
from __future__ import annotations

import argparse
import datetime as dt
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]


def read(path: pathlib.Path) -> str:
    try:
        return path.read_text(encoding="utf-8")
    except FileNotFoundError:
        return ""


def recent_commits(n: int = 8) -> list[str]:
    try:
        out = subprocess.check_output(
            ["git", "log", f"-{n}", "--pretty=format:%s"],
            cwd=ROOT,
            text=True,
            stderr=subprocess.DEVNULL,
        )
        return [line.strip() for line in out.splitlines() if line.strip()]
    except Exception:
        return []


def detect_features(readme: str, guia: str) -> list[str]:
    blob = (readme + "\n" + guia).lower()
    catalog = [
        ("libp2p", "Nodos libp2p (local o relay) con el mismo binario"),
        ("agente", "Agentes con identidad, saldo de aplicación y contenido por CID"),
        ("app.ans", "Apps publicables en la malla (/w/nombre.app.ans)"),
        ("lisp", "Lisp embebido (LispAI) para automatizar el nodo por HTTP"),
        ("zyrion", "Zyrion: lógica ternaria (0/1/2) y DSL evaluar-zyrion"),
        ("embedding", "Modelos ligeros (embedding, capas, inferencia) en agentes"),
        ("auth", "Auth con tokens/roles, módulos y entidades"),
        ("api v2", "API v2 para agentes, transferencias y apps"),
        ("supabase", "Persistencia en disco o Supabase"),
        ("ci", "CI en GitHub Actions (vet, test, build) en cada PR"),
    ]
    found = []
    for key, line in catalog:
        if key in blob:
            found.append(line)
    # de-dup preserve order
    seen = set()
    out = []
    for f in found:
        if f not in seen:
            seen.add(f)
            out.append(f)
    return out or [line for _, line in catalog[:6]]


def build_es(features: list[str], commits: list[str]) -> tuple[str, str]:
    bullets = "\n".join(f"• {f}" for f in features)
    hitos = ""
    if commits:
        # pick up to 3 non-merge subjects
        picks = [c for c in commits if not c.lower().startswith("merge ")][:3]
        if picks:
            hitos = "\n**Últimos avances en el repo**\n" + "\n".join(f"• {c}" for c in picks) + "\n"

    full = f"""Construí **Alset (P.TEC-AN v4.0)** — una red peer-to-peer en **Go** donde cada proceso es un nodo capaz de coordinar datos, lógica y automatización en el borde.

No es un tutorial suelto: es un sistema desplegable, documentado y con CI.

**Qué resuelve**
{bullets}
{hitos}
**Cómo está hecho**
Arquitectura modular (`cmd/` + `internal/`), tests, guía de uso, documentación de API y landing pública. Relay en producción para probar la API sin instalar nada.

**Por qué lo comparto**
Me interesa el diseño de sistemas distribuidos reales: red, automatización, reglas explícitas y operación (CI, deploy, docs), no solo demos aisladas.

Si trabajas en backend, sistemas distribuidos, plataforma o edge computing, me encantará conectar.

Repo: https://github.com/yecharlot/PrismaTec
Guía: https://github.com/yecharlot/PrismaTec/blob/main/docs/GUIA.md
Demo / relay: https://prismatec.onrender.com
Landing: https://yecharlot.github.io/PrismaTec/
API v2: https://prismatec.onrender.com/api/v2/info

#Golang #DistributedSystems #libp2p #OpenSource #Backend #SystemsDesign #PeerToPeer #SoftwareEngineering
"""

    short = """Construí **Alset**: red P2P en Go con nodos libp2p, agentes, apps por CID, Lisp embebido, lógica ternaria (Zyrion), API v2, Supabase y CI en cada PR.

Mismo binario en local o como relay. Documentado y desplegado.

Repo → https://github.com/yecharlot/PrismaTec
Relay → https://prismatec.onrender.com

#Golang #DistributedSystems #OpenSource #Backend
"""
    return full.strip() + "\n", short.strip() + "\n"


def build_en(features: list[str], commits: list[str]) -> tuple[str, str]:
    # map features to rough English by regenerating from keys
    en_map = {
        "libp2p": "libp2p nodes (local or cloud relay) from one binary",
        "Agentes": "Agents with identity, app-level balance, and CID-anchored content",
        "Apps": "Mesh-published apps (/w/name.app.ans)",
        "Lisp": "Embedded Lisp (LispAI) to automate the node over HTTP",
        "Zyrion": "Zyrion ternary logic (0/1/2) and evaluar-zyrion decision DSL",
        "Modelos": "Lightweight models (embedding, layers, inference) stored as agents",
        "Auth": "Token/role auth, modules, and entities",
        "API v2": "v2 API for agents, transfers, and apps",
        "Persistencia": "Disk or Supabase persistence",
        "CI": "GitHub Actions CI (vet, test, build) on every PR",
    }
    bullets = []
    for f in features:
        for k, v in en_map.items():
            if k.lower() in f.lower() or f.lower().startswith(k.lower()[:4]):
                bullets.append(f"• {v}")
                break
        else:
            bullets.append(f"• {f}")
    # unique
    seen = set()
    b2 = []
    for b in bullets:
        if b not in seen:
            seen.add(b)
            b2.append(b)
    bl = "\n".join(b2)
    full = f"""I built **Alset (P.TEC-AN v4.0)** — a peer-to-peer network in **Go** where every process is a node that can coordinate data, logic, and automation at the edge.

This is a deployable system with docs and CI, not a one-off demo.

**What it covers**
{bl}

**How it's built**
Modular layout (`cmd/` + `internal/`), tests, a full user guide, API docs, and a public landing page. A live relay is available to try the API without installing anything.

**Why I'm sharing**
I care about real distributed systems work: networking, automation, explicit rules, and operations (CI, deploy, documentation).

If you work on backend, distributed systems, platform, or edge computing, I'd be glad to connect.

Repo: https://github.com/yecharlot/PrismaTec
Guide: https://github.com/yecharlot/PrismaTec/blob/main/docs/GUIA.md
Live relay: https://prismatec.onrender.com
Landing: https://yecharlot.github.io/PrismaTec/

#Golang #DistributedSystems #libp2p #OpenSource #Backend #SystemsDesign
"""
    short = """I built **Alset**: a Go P2P network with libp2p nodes, agents, CID-backed apps, embedded Lisp, ternary logic (Zyrion), v2 API, Supabase, and CI on every PR.

One binary for local node or cloud relay. Documented and deployed.

Repo → https://github.com/yecharlot/PrismaTec
Relay → https://prismatec.onrender.com

#Golang #DistributedSystems #OpenSource #Backend
"""
    return full.strip() + "\n", short.strip() + "\n"


def main() -> int:
    ap = argparse.ArgumentParser(description="Generate LinkedIn post for PrismaTec/Alset")
    ap.add_argument("--lang", choices=("es", "en"), default="es")
    ap.add_argument("--out", type=pathlib.Path, default=None)
    args = ap.parse_args()

    readme = read(ROOT / "README.md")
    guia = read(ROOT / "docs" / "GUIA.md")
    features = detect_features(readme, guia)
    commits = recent_commits()

    if args.lang == "en":
        full, short = build_en(features, commits)
    else:
        full, short = build_es(features, commits)

    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    doc = f"""# Publicación LinkedIn generada automáticamente

Generado: {stamp}
Idioma: {args.lang}

## Versión completa

{full}

## Versión corta

{short}
"""
    out = args.out or (ROOT / "docs" / "linkedin" / "POST_GENERADO.md")
    out.parent.mkdir(parents=True, exist_ok=True)
    out.write_text(doc, encoding="utf-8")
    print(doc)
    print(f"\n[written] {out}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
