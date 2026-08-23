# Alset-Gen autónomo (`cmd/alset-gen`)

El gen puede vivir **fuera del monolito PrismaTec** como un proceso pequeño: misma identidad ANS, página de servicio, pulsos HTTP y (opcional) host libp2p propio.

## Idea en una frase

**Paquete CID** = la semilla en un frasco.  
**`alset-gen`** = la maceta donde se planta y sigue viva.

## Requisitos

- Un archivo JSON `FrontierPackage` (`type: alset_gen_frontier_package`).
- Export desde el nodo:

```bash
curl -s "https://prismatec.onrender.com/api/gen/export?key=demo-cell" -o demo-cell.package.json
```

O sella y recupera el bloque por CID si lo tienes en el blockstore.

## Arranque

```bash
go run ./cmd/alset-gen -package demo-cell.package.json -http :9090
# con red P2P propia:
go run ./cmd/alset-gen -package demo-cell.package.json -http :9090 -p2p
```

## Endpoints del daemon

| Ruta | Uso |
|------|-----|
| `GET /` | Página de servicio del gen |
| `GET /api/info` | Identidad, root, peer id |
| `POST /api/pulse` | Pulso JSON directo |
| `GET /api/resolve?key=` | Resolución local de nombres |
| `GET /health` | Salud |

## ¿Puede vivir en un router?

| Entorno | ¿Viable? |
|---------|----------|
| Router doméstico cerrado (firmware de fábrica) | **No** en la práctica |
| OpenWrt / firmware libre con espacio y CPU | **Sí**, binario Go para la arquitectura del router (mips/arm), poca RAM |
| Raspberry Pi / mini-PC en el borde | **Sí**, ideal |
| Solo el `package_cid` en el router sin ejecutar nada | **No “vive”**: el CID es dato; la vida es el proceso |

El paquete **sobrevive** como archivo o CID en el router.  
El gen **vive** solo si algo ejecuta `alset-gen` (u otro runtime Alset).

## Relación con PrismaTec

- PrismaTec sigue siendo laboratorio + Mind + `/api/gen/*`.
- El daemon no reemplaza el nodo; es la **mudanza** de una célula a su propio proceso.
