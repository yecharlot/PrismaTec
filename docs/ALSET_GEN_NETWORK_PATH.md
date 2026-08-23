# Gen en la frontera de conectividad (visión y límites)

## Qué significa de verdad

No se trata de habitar `example.com`. Se trata de **habitar el camino de la red**:

- routers / gateways / Pi en el borde (**equipos de conectividad**),
- **protocolo ligero** (UDP) además de HTTP,
- **identidad y estado** que viajan como **carga** (paquete CID / fragmentos CARGO),
- mini HTTP solo para que humanos y Mind entren cuando hace falta.

## Física de la red (límites honestos)

| Idea | Realidad |
|------|----------|
| “Vivir dentro de paquetes IP ajenos en routers cerrados” | **No** sin firmware propio (sería inyectar código en cajas que no controlas) |
| Proceso en OpenWrt / gateway / Pi en el camino | **Sí** — ahí el gen **reside** |
| Identidad + hallazgos viajando en datagramas UDP | **Sí** — Pulse-over-UDP + CARGO |
| Mini servidor HTTP en el borde | **Sí** — ya en `alset-gen` |
| Mind localiza el gen | **Sí** — `announce` + `remote_http` (+ `udp_port`) |

La “posición de tránsito” en software es: **daemon en el equipo intermedio**, no código ejecutándose mágicamente en cada frame Ethernet de un ISP.

## Pulse-over-UDP (implementado)

Puerto por defecto de ejemplo: `9091`.

```json
{ "v": 1, "type": "BEACON", "key": "demo-cell.ans", "ts": 123, "data": { "root_cid": "...", "findings": 2 } }
{ "v": 1, "type": "CONSULTA", "key": "demo-cell.ans", "text": "qué sabes", "ts": 123 }
{ "v": 1, "type": "RESPUESTA", "key": "demo-cell.ans", "text": "…", "ts": 123 }
{ "v": 1, "type": "CARGO", "key": "demo-cell.ans", "data": { "part": 1, "of": 4, "claim": "identity" } }
```

Arranque:

```bash
go run ./cmd/alset-gen \
  -package demo-cell.package.json \
  -http :9090 \
  -udp 9091 \
  -announce https://prismatec.onrender.com \
  -public-url https://TU-TUNEL
```

- **BEACON** cada ~20s en broadcast de subred (descubrimiento LAN).
- **CONSULTA/RESPUESTA** diálogo sin HTTP.
- **CARGO** demo de “viajar en fragmentos” (ensamblaje lógico en el borde).

## Camino histórico viable

1. OpenWrt/Pi con `alset-gen` en la casa o el ISP comunitario.  
2. UDP en LAN + HTTP público vía tunnel para Mind.  
3. Más adelante: red de bordes que reenvían CARGO entre sí (store-and-forward).  

Eso **es** habitar la conectividad. No es malware en routers ajenos.
