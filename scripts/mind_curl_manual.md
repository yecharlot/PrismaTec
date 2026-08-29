# Curl manual (selección profunda)

```bash
BASE=http://localhost:8080

# Salud
curl -s $BASE/api/v2/info | head -c 200; echo

# Sesiones
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"me llamo Diego","session":"s-a"}' | jq -r .voice
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"cómo me llamo?","session":"s-a"}' | jq -r .voice
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"cómo me llamo?","session":"s-b"}' | jq -r .voice

# Razón
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"el perro es un animal y pepe es un perro entonces qué deduces","session":"r1"}' | jq -r .voice

# Math
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"cuánto es 12+7","session":"m1"}' | jq -r .voice

# Genes
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"crea gen manual-probe","session":"g1"}' | jq -r .voice
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"lista genes","session":"g1"}' | jq -r .voice
curl -s -X POST $BASE/api/mind/tick -H 'Content-Type: application/json' \
  -d '{"text":"elimina gen manual-probe","session":"g1"}' | jq -r .voice
```
