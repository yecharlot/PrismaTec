#!/usr/bin/env python3
"""Volume dialogue probe against a Mind node. Reports route + anomaly patterns.

Usage:
  python3 scripts/mind_volume_probe.py --base https://prismatec-4u5c.onrender.com --n 40
  python3 scripts/mind_volume_probe.py --base http://localhost:8080
"""
from __future__ import annotations

import argparse
import json
import re
import sys
import time
import urllib.error
import urllib.request
from collections import Counter, defaultdict

ANOMALY_RULES = [
    ("html_css_junk", re.compile(r"(?i)prefers-color-scheme|function\s*\(|var\s+dc_enabled|header-wrap|@media\s*\(")),
    ("raw_scout_echo", re.compile(r"(?i)hallazgo\s+sonda\s+scout-")),
    ("disambiguation", re.compile(r"(?i)desambiguaci[oó]n|may refer to|varias entradas|puede referirse a")),
    ("empty_summary_body", re.compile(r"(?i)tengo este resumen[^\n]*:\s*\n\s*\n")),
    ("lab_jargon", re.compile(r"(?i)despach[eé]\s+la\s+sonda|sonda temporal eliminada")),
    ("soft_memory_hijack", re.compile(r"(?i)me suena esto:.*(?:scout-|hallazgo)")),
    ("english_only_long", re.compile(r"(?i)\b(the|was|and|with|from)\b.+\b(the|was|and)\b")),
]

PROBES = [
    # factual ES
    "quién es michael jordan",
    "quién es michel jordan",
    "quién es benjamin franklin",
    "quién es juana de arco",
    "quién es marie curie",
    "quién es isaac newton",
    "quién es tiziano ferro",
    "quién es félix varela",
    "qué es el antagonismo",
    "qué es la fotosíntesis",
    "qué es la mitocondria",
    "qué es el feudalismo",
    "qué es un goroutine",
    # follow-ups (need session continuity — sequential block)
    "quién es michael jordan",
    "como se llama su madre",
    # action memory
    "qué hice",
    "que hiciste",
    "porque lo hiciste",
    "por qué lo hiciste",
    # math
    "cuánto es 15 + 27",
    "suma 8 y 9",
    # memory / chat
    "hola",
    "cómo me llamo",
    "me llamo Esteban",
    "cómo me llamo",
    # noise / edge
    "asdfghjkl",
    "qué significa esto",
    "genera código función sumar en go",
]

def tick(base: str, text: str, timeout: float = 45.0) -> dict:
    req = urllib.request.Request(
        base.rstrip("/") + "/api/mind/tick",
        data=json.dumps({"text": text}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=timeout) as resp:
        return json.loads(resp.read().decode())


def detect(voice: str) -> list[str]:
    if not voice or not str(voice).strip():
        return ["empty_voice"]
    hits = []
    for code, rx in ANOMALY_RULES:
        if code == "english_only_long":
            # only if long and looks English-heavy
            if len(voice) > 140 and rx.search(voice) and not re.search(r"(?i)\b(el|la|de|que|fue|una)\b", voice):
                hits.append(code)
            continue
        if rx.search(voice):
            hits.append(code)
    return hits


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://localhost:8080")
    ap.add_argument("--n", type=int, default=0, help="repeat probes to reach ~n requests (0=once)")
    ap.add_argument("--sleep", type=float, default=0.35)
    args = ap.parse_args()

    probes = list(PROBES)
    if args.n > len(probes):
        # pad with rotating factual
        extra = [
            "quién es ada lovelace", "quién es alan turing", "quién es galileo",
            "qué es la gravitación", "qué es un átomo", "qué es la democracia",
            "quién es cervantes", "quién es borges", "qué es la recursión",
        ]
        i = 0
        while len(probes) < args.n:
            probes.append(extra[i % len(extra)])
            i += 1

    codes = Counter()
    by_probe = defaultdict(list)
    failures = 0
    ok = 0
    print(f"base={args.base} probes={len(probes)}")
    for i, text in enumerate(probes):
        try:
            data = tick(args.base, text)
            voice = data.get("voice") or ""
            note = data.get("note") or ""
            hits = detect(voice)
            for h in hits:
                codes[h] += 1
                by_probe[h].append(text)
            status = "OK" if not hits else "ANOMALY:" + ",".join(hits)
            print(f"[{i+1}/{len(probes)}] {status}  q={text!r}  voice={voice[:90]!r}…")
            ok += 1
        except Exception as e:
            failures += 1
            print(f"[{i+1}/{len(probes)}] FAIL  q={text!r}  err={e}")
        time.sleep(args.sleep)

    print("\n=== SUMMARY ===")
    print(f"ok={ok} fail={failures}")
    print("anomaly counts:", dict(codes))
    for code, qs in by_probe.items():
        print(f"  {code}: {qs[:8]}{'…' if len(qs)>8 else ''}")
    # exit non-zero if serious anomalies
    serious = sum(codes[c] for c in ("html_css_junk", "soft_memory_hijack", "empty_summary_body", "empty_voice"))
    return 1 if failures or serious else 0


if __name__ == "__main__":
    sys.exit(main())
