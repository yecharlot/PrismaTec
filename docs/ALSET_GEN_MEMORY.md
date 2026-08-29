> **Archivo histórico.** La documentación canónica es:
> - [README.md](../README.md) — mapa del repo
> - [HANDOFF.md](HANDOFF.md) — estado y gaps
> - [GUIA.md](GUIA.md) — ayuda operativa
>
> Si este texto contradice el HANDOFF, **gana el HANDOFF**.

---
> Incluido en [`ALSET_MIND_AND_GEN.md`](ALSET_MIND_AND_GEN.md).

# Gen-memoria — salva de información por CID

## Idea

Genes con **misión memoria** actúan como contenedores de CIDs (episodios, notas). No sustituyen a Mind; son **réplica/salva** opcional y móvil.

## Diálogo Mind

```
crea gen memoria mem-nodo
salva en gen mem-nodo: la red usa Cloudflare DO para persistir bloques
lista genes memoria
```

## API

```http
POST /api/gen/memory/create  {"key":"mem-nodo","description":"..."}
POST /api/gen/memory/save    {"key":"mem-nodo","text":"...","note":"..."}
POST /api/gen/memory/save    {"key":"mem-nodo","cid":"bafk...","note":"..."}
GET  /api/gen/memory/list
```

## Qué hacer / no hacer

- Sí: hechos de red, índices de episodios, notas de operación.
- No: passwords, service keys, datos de terceros sin permiso.
