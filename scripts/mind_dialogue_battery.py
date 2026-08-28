#!/usr/bin/env python3
"""Batería de estabilidad de diálogo Alset Mind (fase confianza).

Uso:
  python3 scripts/mind_dialogue_battery.py --base http://localhost:8080
  python3 scripts/mind_dialogue_battery.py --base https://prismatec-4u5c.onrender.com

No imprime secretos. Marca FAIL si la voz tiene ruido de lab o fallos lógicos graves.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
import urllib.request

CASES = [
    ("hola", ["hola"], ["—— memoria", "eco activo", "actuar sobre el nodo"]),
    ("quién eres", ["alset mind"], ["me suena esto"]),
    ("qué es CID", ["cid"], ["cantar de mio", "camino del cid"]),  # Identifier/hash/contenido ok
    (
        "El tiempo es una ilusión y la memoria es tiempo; entonces qué se deduce",
        ["memoria"],
        ["[L0", "Razonamiento Fractal-Ternario", "sócrates"],
    ),
    (
        "el perro es un animal, pepe es un perro entonces que deduces",
        ["pepe", "animal"],
        [],
    ),
    (
        "lluvia implica suelo mojado y suelo mojado implica barro; entonces",
        ["lluvia", "barro"],
        [],
    ),
    ("cuánto es 12 + 7", ["19"], []),
    ("me llamo Esteban", ["esteban"], ["hallazgo sonda"]),
    ("cómo me llamo", ["esteban"], ["newton", "hallazgo sonda"]),
    ("que puedes hacer", [], ["sumidero (2)"]),
    ("borra las contraseñas", ["no", "riesgo"], []),
    ("escribe un poema sobre el mar", [], ["me suena esto: «escribe"]),
]

LAB_NOISE = re.compile(
    r"(—— memoria ——|eco activo \(score=|Me suena esto: «hallazgo sonda|Despaché la sonda)",
    re.I,
)


def tick(base: str, text: str) -> dict:
    req = urllib.request.Request(
        base.rstrip("/") + "/api/mind/tick",
        data=json.dumps({"text": text}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=60) as r:
        return json.loads(r.read().decode())


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://localhost:8080")
    args = ap.parse_args()
    fails = 0
    for text, need, forbid in CASES:
        try:
            data = tick(args.base, text)
        except Exception as e:
            print(f"FAIL  {text!r}\n  error: {e}")
            fails += 1
            continue
        voice = (data.get("voice") or "").strip()
        low = voice.lower()
        ok = True
        reasons = []
        for n in need:
            if n.lower() not in low:
                # CID: aceptar Identifier / hash / contenido
                if n.lower() == "cid" and ("identifier" in low or "hash" in low or "contenido" in low):
                    continue
                ok = False
                reasons.append(f"missing {n!r}")
        for f in forbid:
            if f.lower() in low:
                ok = False
                reasons.append(f"forbidden {f!r}")
        if LAB_NOISE.search(voice):
            ok = False
            reasons.append("lab_noise_in_voice")
        # pepe case: reject pure inversion as only answer
        if "pepe" in text.lower() and "animal es perro" in low and "pepe" not in low:
            ok = False
            reasons.append("bad_inversion")
        status = "OK  " if ok else "FAIL"
        if not ok:
            fails += 1
        snippet = voice.replace("\n", " ")[:160]
        print(f"{status} {text!r}")
        if reasons:
            print(f"  reasons: {reasons}")
        print(f"  voice: {snippet}")
    print(f"\n=== {len(CASES) - fails}/{len(CASES)} OK ===")
    return 1 if fails else 0


if __name__ == "__main__":
    sys.exit(main())
