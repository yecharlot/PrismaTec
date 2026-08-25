#!/usr/bin/env python3
"""High-volume Mind dialogue probe + anomaly pattern report."""
from __future__ import annotations
import argparse, json, re, sys, time, urllib.request
from collections import Counter, defaultdict

ANOMALY_RULES = [
    ("html_css_junk", re.compile(r"(?i)prefers-color-scheme|function\s*\(|var\s+dc_enabled|header-wrap|@media\s*\(")),
    ("raw_scout_echo", re.compile(r"(?i)hallazgo\s+sonda\s+scout-")),
    ("disambiguation", re.compile(r"(?i)desambiguaci[oó]n|may refer to|varias entradas|puede referirse a")),
    ("empty_summary_body", re.compile(r"(?i)tengo este resumen[^\n]*:\s*\n\s*\n")),
    ("lab_jargon", re.compile(r"(?i)despach[eé]\s+la\s+sonda|sonda temporal eliminada")),
    ("soft_memory_hijack", re.compile(r"(?i)me suena esto:.*(?:scout-|hallazgo)")),
    ("lisp_false_corpus", re.compile(r"(?i)en lisp,\s*if es")),
    ("english_only_long", re.compile(r"(?i)\b(the|was|and|with|from)\b")),
]

FACTUAL = [
    "quién es michael jordan", "quién es michel jordan", "quién es benjamin franklin",
    "quién es juana de arco", "quién es marie curie", "quién es isaac newton",
    "quién es tiziano ferro", "quién es félix varela", "quién es cervantes",
    "quién es borges", "quién es ada lovelace", "quién es alan turing",
    "quién es galileo", "quién es hipatia", "quién es sor juana",
    "qué es el antagonismo", "qué es la fotosíntesis", "qué es la mitocondria",
    "qué es el feudalismo", "qué es un átomo", "qué es la democracia",
    "qué es la gravitación", "qué es un goroutine", "qué es la recursión",
]
CREATIVE = [
    "escribe un poema sobre el mar", "escribe un poema sobre la relatividad",
    "escribe un cuento sobre un gen", "haz un poema sobre el silencio",
]
ACTION = ["qué hice", "que hiciste", "porque lo hiciste", "por qué lo hiciste", "qué patrones aprendiste"]
MATH = ["cuánto es 15 + 27", "suma 8 y 9", "cuanto es 100-7"]
MEMORY = ["me llamo Esteban", "cómo me llamo", "hola"]
FOLLOW = ["quién es michael jordan", "como se llama su madre"]
NOISE = ["asdfghjkl", "???"]
CODE = ["genera código función sumar en go"]

def all_probes(n: int) -> list[str]:
    base = FACTUAL + CREATIVE + ACTION + MATH + MEMORY + FOLLOW + NOISE + CODE
    if n <= len(base):
        return base[:n] if n else base
    out = list(base)
    i = 0
    while len(out) < n:
        out.append(FACTUAL[i % len(FACTUAL)])
        i += 1
    return out

def tick(base: str, text: str, timeout: float = 50.0) -> dict:
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
            if len(voice) > 140 and rx.search(voice) and not re.search(r"(?i)\b(el|la|de|que|fue|una)\b", voice):
                hits.append(code)
            continue
        if rx.search(voice):
            hits.append(code)
    return hits

def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://localhost:8080")
    ap.add_argument("--n", type=int, default=0)
    ap.add_argument("--sleep", type=float, default=0.35)
    args = ap.parse_args()
    probes = all_probes(args.n)
    codes = Counter()
    by = defaultdict(list)
    ok = fail = 0
    print(f"base={args.base} probes={len(probes)}")
    for i, text in enumerate(probes):
        try:
            data = tick(args.base, text)
            voice = data.get("voice") or ""
            hits = detect(voice)
            for h in hits:
                codes[h] += 1
                by[h].append(text)
            st = "OK" if not hits else "ANOMALY:" + ",".join(hits)
            print(f"[{i+1}/{len(probes)}] {st} q={text!r} v={voice[:85]!r}…")
            ok += 1
        except Exception as e:
            fail += 1
            print(f"[{i+1}/{len(probes)}] FAIL q={text!r} err={e}")
        time.sleep(args.sleep)
    print("\n=== SUMMARY ===")
    print(f"ok={ok} fail={fail} anomalies={dict(codes)}")
    for c, qs in by.items():
        print(f"  {c}: {qs[:6]}")
    serious = sum(codes[c] for c in ("html_css_junk", "soft_memory_hijack", "empty_summary_body", "empty_voice", "lisp_false_corpus"))
    return 1 if fail or serious else 0

if __name__ == "__main__":
    sys.exit(main())
