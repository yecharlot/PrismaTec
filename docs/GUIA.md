# Guía de uso — Alset Mind & Gen

Ayuda operativa unificada. Estado del proyecto: [HANDOFF.md](HANDOFF.md).

---

## 1. Mind: latido

```bash
curl -s -X POST http://localhost:8080/api/mind/tick \
  -H "Content-Type: application/json" \
  -d '{"text":"hola","session":"demo-1"}' | jq -r .voice
```

| Campo | Uso |
|-------|-----|
| `text` | Mensaje del usuario |
| `session` | Aísla memoria/hilo (también header `X-Mind-Session`) |

### Sesiones

- Misma `session` → recuerda hechos personales de ese hilo.  
- Otra `session` → no mezcla nombres/hechos.  
- Sin session estable → comportamiento más anónimo (no inventar identidad cruzada).

### Qué puede hacer Mind

| Capacidad | Ejemplo |
|-----------|---------|
| Charla / identidad | «quién eres», «cómo estás» |
| Memoria personal | «me llamo Ana» → «cómo me llamo» |
| Corpus | «qué es la red alset», Lisp, Go, filosofía… |
| Razón | «A es B y B es C, entonces…» |
| Cálculo | «cuánto es 12+8» (LispAI / reglas) |
| Codegen plantilla | «genera código función sumar en go» |
| Scout si no sabe | «qué es …» → sonda web + respuesta |
| Genes | crear, preguntar, explorar, despachar, retornar, eliminar |

### Prioridad interna (director)

```text
ethics/veto → tools gen → math → codegen → memoria sesión → corpus/razón → charla/scout
```

---

## 2. Genes (sondas)

```text
crea gen genesis
pregunta al gen genesis: quién eres
envía al gen genesis a explorar https://ejemplo.com
despacha gen genesis a cloudflare
retorna gen genesis
elimina gen genesis
crea gen memoria mem-nodo
salva en gen mem-nodo: nota
lista genes
```

API:

| Método | Ruta |
|--------|------|
| POST | `/api/gen/create` |
| POST | `/api/gen/dialogue` |
| POST | `/api/gen/explore` |
| POST | `/api/gen/dispatch` |
| POST | `/api/gen/return` |
| POST | `/api/gen/delete` |
| POST | `/api/gen/memory/save` |

Mind orquesta; el gen ejecuta y guarda hallazgos/CID.

---

## 3. Lab UI

`http://localhost:8080/w/mind.app.ans`  
Órganos, genoma, memoria, calibración (según build).

---

## 4. Pruebas recomendadas

```bash
go test ./internal/node/ -count=1 -timeout 120s -run 'Dialogue|Director|Session|Reason'

# Batería viva (nodo arriba)
python3 scripts/mind_dialogue_battery.py --base http://localhost:8080
```

### Checklist humano rápido

1. Sesión A nombre / Sesión B no filtra.  
2. Silogismo simple.  
3. «lista genes» limpio.  
4. Scout «qué es X» desconocido.  
5. Ethics: pedido destructivo vetado.

---

## 5. Variables de entorno útiles

| Variable | Efecto |
|----------|--------|
| `ALSET_CF_STORE_URL` | Store Durable Object |
| `ALSET_PERSIST=cloudflare` | Prioriza CF |
| `ALSET_CLOUDFLARE_NETWORK` | Edge genes |
| `ALSET_AUTO_PIN_MEM=1` | Ancla episodios a mem-nodo |
| `ALSET_SCOUT_EPHEMERAL=0` | Conserva sondas scout |

---

## 6. Límites (honestos)

- No es ChatGPT: fuera de corpus/memoria/scout puede pedir aclaración.  
- Scout = fragmentos de páginas, no omnisciencia.  
- Codegen = plantillas, no repos enteros.  
- Deploy cloud: confirmar commit; auto-deploy suele estar OFF.
