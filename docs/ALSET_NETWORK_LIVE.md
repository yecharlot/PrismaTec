> **Archivo histórico.** La documentación canónica es:
> - [README.md](../README.md) — mapa del repo
> - [HANDOFF.md](HANDOFF.md) — estado y gaps
> - [GUIA.md](GUIA.md) — ayuda operativa
>
> Si este texto contradice el HANDOFF, **gana el HANDOFF**.

---
# Red Alset en Cloudflare — estado en producción

## URL viva

```text
https://alset-network.lhmolam-877.workers.dev
```

| Recurso | Valor |
|---------|--------|
| Account | `877ca6ef38275b318cd4ada835b2bae2` |
| Worker | `alset-network` |
| KV | `ALSET_GEN_KV` (`f17505e009c64d7a9f88edb8a0cc55ac`) |
| Announce | `https://prismatec.onrender.com` |
| Render env | `ALSET_CLOUDFLARE_NETWORK` = URL de arriba |

## Pruebas realizadas (2026-08-23)

| Prueba | Resultado |
|--------|-----------|
| GET `/` portada red | OK |
| POST `/api/network/dispatch` demo-cell | OK · reach `/g/demo-cell` |
| Announce → PrismaTec | HTTP 200 |
| GET `/api/network/gens` | 1 gen listado |
| POST `/g/demo-cell/api/dialogue` | Voz identidad OK |
| POST `/g/demo-cell/api/explore` example.com | status 200, title Example Domain |
| Dialogue post-explore | reporta 1 hallazgo |

## Uso desde Mind / API (tras deploy Render)

```bash
curl -s -X POST https://prismatec.onrender.com/api/gen/dispatch \
  -H "Content-Type: application/json" \
  -d '{"key":"sonda-edge","destination":"cloudflare","mission":"oficio"}' | jq .

curl -s -X POST https://prismatec.onrender.com/api/mind/tick \
  -d '{"text":"despacha gen sonda-edge a cloudflare"}' | jq -r .voice

curl -s -X POST https://prismatec.onrender.com/api/gen/dialogue \
  -d '{"key":"sonda-edge","text":"quién eres"}' | jq .
```

Directo en el edge:

```bash
curl -s https://alset-network.lhmolam-877.workers.dev/api/network/gens | jq .
curl -s -X POST https://alset-network.lhmolam-877.workers.dev/g/demo-cell/api/dialogue \
  -d '{"text":"qué sabes"}' | jq .
```

## Seguridad

El token de API de Cloudflare se usó solo para el despliegue. **Rótalo** en el dashboard si quedó expuesto en chat. No se guarda en el repositorio.
