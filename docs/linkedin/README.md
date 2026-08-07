# LinkedIn — escenario automático

## Objetivo

Cuando digas **“genera la publicación de LinkedIn”** (o similar), se produce un post orientado a reclutadores a partir del estado del repo, sin inventar métricas.

## Piezas

| Archivo | Rol |
|---------|-----|
| [PLANTILLA.md](PLANTILLA.md) | Reglas de tono y estructura |
| [POST_ACTUAL.md](POST_ACTUAL.md) | Última versión lista para copiar/pegar |
| [POST_GENERADO.md](POST_GENERADO.md) | Salida del script (se regenera) |
| `scripts/generate_linkedin_post.py` | Generador automático desde README/guía/commits |

## Cómo regenerar en local

```bash
python3 scripts/generate_linkedin_post.py
python3 scripts/generate_linkedin_post.py --lang en
python3 scripts/generate_linkedin_post.py --out docs/linkedin/POST_GENERADO.md
```

Luego copia el bloque “Versión completa” o “Versión corta” en LinkedIn.

## Cómo pedirlo en el chat (con Grok / asistente)

Frases que activan el escenario:

- “Genera la publicación de LinkedIn”
- “Artículo LinkedIn para reclutadores”
- “Actualiza el post de LinkedIn del repo”

El asistente debe:

1. Leer `docs/linkedin/PLANTILLA.md`
2. Ejecutar o seguir la lógica de `scripts/generate_linkedin_post.py`
3. Actualizar `docs/linkedin/POST_ACTUAL.md` si el texto queda como versión oficial
4. Devolver el texto listo para pegar (completa + corta)

## Qué no hace este escenario

- No publica solo en LinkedIn (hace falta tu sesión o la API oficial con OAuth).
- No inventa estrellas, usuarios o clientes.
- No afirma que Alset es una blockchain o un LLM de producción.

## Publicación real en LinkedIn (opcional, más adelante)

1. App en LinkedIn Developers + OAuth  
2. Token en GitHub Secrets  
3. Action que solo publique en **release** o cuando se dispare `workflow_dispatch`  
4. El cuerpo del post puede ser la salida de este script  

Hasta entonces: **generación automática del texto** + pegado manual (o API cuando la configures).
