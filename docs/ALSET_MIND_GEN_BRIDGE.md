> Resumen operativo en [`ALSET_MIND_AND_GEN.md`](ALSET_MIND_AND_GEN.md). Este doc es detalle del puente.

# Puente Alset Mind ↔ Alset Gen

## Modelo

```text
Usuario ──► Mind (órganos + ethics) ──► Gen (local o borde)
                │                         │
                ├─ episode CID            ├─ EpisodeCIDs (gen-memoria)
                └─ orquesta               └─ dialogue / explore / dispatch
```

- **Mind decide** (ternario). **Gen ejecuta / reside / almacena CIDs**.
- Ethics de Mind **no se bypasea** al hablar con un gen.
- Gen-memoria = salva opcional de la memoria del nodo/red.

## Frases Mind

| Intento | Ejemplo |
|---------|---------|
| Crear célula | `crea gen sonda` |
| Memoria | `crea gen memoria mem-nodo` |
| Salvar | `salva en gen mem-nodo: hecho X` |
| Dialogar | `pregunta al gen demo-cell: quién eres` / `dile al gen X: estado` |
| Despachar | `despacha gen demo-cell a cloudflare` |
| Vincular | `vincula memoria` (último episodio → gen memoria) |
| Listar | `lista genes` / `lista genes memoria` |

## API

- Gen: `/api/gen/*`, `/api/gen/memory/*`, `/api/gen/dialogue`, `/api/gen/dispatch`
- Mind: `/api/mind/tick`, `/api/mind/self` → campo **`gen_bridge`**

## Env

| Variable | Efecto |
|----------|--------|
| `ALSET_CLOUDFLARE_NETWORK` | Edge para dispatch/dialogue remoto |
| `ALSET_AUTO_PIN_MEM=1` | Cada episodio Mind se ancla también en `mem-nodo` |

## Dónde estamos / destino / falta

| | |
|--|--|
| **Ahora** | Orquestación por voz + bridge status + auto-pin opcional + gen-memoria |
| **Destino** | Memoria de red coherente Mind↔Gen↔CF store; genes memoria despachables |
| **Falta** | `wrangler deploy` Store DO; despacho automático de gen-memoria al edge; UI lab panel gen |
