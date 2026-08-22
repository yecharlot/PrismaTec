# MANIFIESTO ALSET: LA PIEDRA ANGULAR DEL FUTURO TECNOLÓGICO

**Versión:** 1.0  
**Fecha:** 2026-08-22  
**Autor:** Prism@.TEC Core Technology (PTCT)  
**Estado en repo:** semilla implementada (G0–G1) — ver `docs/ALSET_GEN_ROADMAP.md`

## Propósito

Definir la arquitectura, filosofía e implementación de **Alset-Gen**, la unidad fundamental del ecosistema digital descentralizado.

> Alset Mind y Alset-Gen **no son lo mismo**. Mind es el cerebro que orquesta; Gen es la célula madre digital que viaja, muta y sirve.

---

## Prólogo

No estamos construyendo solo una IA conversacional. Alset-Gen es una **célula madre digital**: identidad estable (key ANS), naturaleza mutable (RootCID), memoria en CIDs, viaje autónomo y metamorfosis gobernada (solo entidades autorizadas, p. ej. Alset Mind / nodo operador).

## Principios

| Principio | Descripción |
|-----------|-------------|
| Identidad estable, naturaleza mutable | La key es eterna; el RootCID es efímero. |
| Metamorfosis gobernada | Solo entidades autorizadas pueden iniciar mutación. |
| Memoria inmutable | Cada transformación queda en CIDs (historial). |
| Viaje autónomo | El gen se mueve / registra ubicación sin ser “transportado” a mano. |
| Resonancia local | Reacciona a pulsos sin tumbar la red. |
| Servicio durante el viaje | Puede responder consultas mientras se mueve. |
| No invasivo | Observa, registra, comunica. |
| Fractalidad implosiva | Esencia mínima (patrón); expansión bajo demanda (horizonte). |

## Núcleo

**Inmutable:** key ANS, material criptográfico del nodo anfitrión (firma), historial de RootCIDs.  
**Mutable:** RootCID actual, manifiesto, estado local, memoria episódica del gen.

**Órganos (herencia conceptual de Mind):** dialog, act, mem, self, ethics, curiosity, humor — valores 0/1/2.

**Pulsos:** CONSULTA, MUTATE_ROOTCID, ESTADO, HALLAZGO (y lifecycle: GEN_CREATED, GEN_MUTATED, GEN_TRAVEL).

## Ciclo de vida

1. **Creación** — key + RootCID inicial + registro ANS.  
2. **Viaje** — ubicación / peers (G1 stub → G3 completo).  
3. **Mutación** — MUTATE_ROOTCID validado → nuevo RootCID + historial.  
4. **Servicio** — responde CONSULTA.  
5. **Retorno / permanencia / absorción** — horizonte.

## Relación con el ecosistema

| Componente | Rol |
|------------|-----|
| Alset Mind | Orquesta creación/monitor/mutación (voz + tools futuras). |
| ANS | key → identidad / agente ancla. |
| Pulse / libp2p | Comunicación y descubrimiento. |
| IPFS / blockstore | RootCIDs y manifiestos. |
| LispAI | Lógica opcional ligada al RootCID (horizonte). |

## Implementación en este repo

- Tipos: `internal/agents/gen.go`
- Ciclo de vida en nodo: `internal/node/gen_lifecycle.go`
- API: `POST /api/gen/create`, `POST /api/gen/mutate`, `POST /api/gen/travel`, `POST /api/gen/consult`, `GET /api/gen`, `GET /api/gen/{key}`
- Persistencia local: `gen_registry.json` (+ CIDs de manifiesto en blockstore)

*Documento vivo — alineado al manifiesto v1.0 del 2026-08-22.*
