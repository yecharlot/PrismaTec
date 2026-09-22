# Latati Twin — GitHub Pages + Cloudflare Edge

- **Front:** este directorio (GitHub Pages)
- **API:** Worker `latati-edge` + Durable Object (siempre viva, sin sleep de Render)
- **Render** (`prisma-tec.onrender.com`) **sigue intacto**

## URLs esperadas
- API: `https://latati-edge.<subdomain>.workers.dev/api/latati`
- Cliente: `https://<user>.github.io/PrismaTec/` o `/pages-latati/`
- Gestión: `.../gestion.html`

PIN gestora (tenant latati): `tati2026`
