# Red Alset en Cloudflare

## Idea

Una sola Worker `alset-network` es el **torrente de borde** donde coexisten muchos genes (un Durable Object por `key`).  
Alset Mind **crea** el gen en PrismaTec y lo **despacha** a Cloudflare (o lo deja local).

```text
Mind / API
   │  POST /api/gen/dispatch { key, destination: "cloudflare" }
   ▼
PrismaTec (sella package_cid)
   │  POST {CF}/api/network/dispatch
   ▼
alset-network.workers.dev
   │  /g/{key}/  → Durable Object del gen
   ▼
oficio en el edge (dialogue, explore, findings)
```

## Despliegue (tu cuenta Cloudflare)

```bash
cd cloudflare/alset-gen-worker
npm install
npx wrangler login          # o CLOUDFLARE_API_TOKEN
npx wrangler secret put ANNOUNCE_URL   # https://prismatec.onrender.com
npx wrangler deploy
```

Anota la URL, p.ej. `https://alset-network.<subdominio>.workers.dev`.

En **Render** (servicio PrismaTec):

```text
ALSET_CLOUDFLARE_NETWORK=https://alset-network.<subdominio>.workers.dev
```

## Uso

```bash
# Crear + enviar a la red CF
curl -s -X POST https://prismatec.onrender.com/api/gen/dispatch \
  -H "Content-Type: application/json" \
  -d '{"key":"sonda-1","destination":"cloudflare","mission":"explorar borde"}' | jq .

# Mind
curl -s -X POST https://prismatec.onrender.com/api/mind/tick \
  -d '{"text":"despacha gen sonda-1 a cloudflare"}' | jq -r .voice

# Hablar con el gen en el edge
curl -s -X POST https://prismatec.onrender.com/api/gen/dialogue \
  -d '{"key":"sonda-1","text":"quién eres"}' | jq .
```

Destinos: `cloudflare` | `local`.

## Tokens

Para que Grok (o CI) despliegue sin login interactivo:

1. Cloudflare Dashboard → My Profile → API Tokens  
2. Token con *Edit Cloudflare Workers*  
3. Variable: `CLOUDFLARE_API_TOKEN` (+ `CLOUDFLARE_ACCOUNT_ID` si hace falta)

Sin ese token no se puede crear la red **en tu cuenta** desde fuera.
