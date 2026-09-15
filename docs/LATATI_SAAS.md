# La Tati core · multi-tenant (SaaS)

Un solo código sirve N tiendas de gestores. Cada una tiene datos, PIN, catálogo y marca aislados.

## URLs

| Rol | La Tati (default) | Otro gestor (`{slug}`) |
|-----|-------------------|-------------------------|
| Tienda | `/w/latati.app.ans` | `/w/{slug}.app.ans` |
| Gestión | `/w/gestion-latati.app.ans` | `/w/gestion-{slug}.app.ans` |
| API | `/api/latati/...` | `/api/latati/t/{slug}/...` |

## Crear un gestor nuevo

```bash
curl -s -X POST https://TU-NODO/api/latati/tenants \
  -H "Content-Type: application/json" \
  -d '{
    "slug": "maria",
    "name": "Boutique María",
    "pin": "maria2026",
    "whatsapp": "5350000000",
    "tagline": "Moda y pedidos",
    "accent": "#e8b4c8",
    "accent2": "#c4789a"
  }'
```

La respuesta incluye `shop`, `gestion`, `api` y `pin`.

## Personalizar

En gestión → Perfil: nombre, WhatsApp, dirección, bio, tagline, colores y PIN.

## Aislamiento

- Store: `latati/v1/store` (La Tati) o `latati/v1/t/{slug}/store`
- Fotos, pedidos, chat e intereses no se mezclan
- Sesión y carrito del navegador van por tenant

## Listar tenants

```bash
curl -s https://TU-NODO/api/latati/tenants
```
