package lisp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	mathrand "math/rand"

	"redalset/internal/agents"
)

// registerPowerBuiltins recovers high-value Lisp primitives from historical
// Alset saves (Apr–Jul 2026): agent economics, embeddings, ternary models.
func (e *Evaluator) registerPowerBuiltins() {
	// --- Identity / network ---
	e.globalEnv.SetFunction("host-id", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		return e.host.PeerID()
	}))

	e.globalEnv.SetFunction("net-peers", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		return float64(e.host.PeerCount())
	}))

	// --- Agent economics & roots ---
	e.globalEnv.SetFunction("get-agent-root", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return ""
		}
		id := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		e.host.RLock()
		defer e.host.RUnlock()
		if a, ok := e.host.GetAgent(id); ok {
			return a.RootCID
		}
		return ""
	}))

	e.globalEnv.SetFunction("set-agent-balance", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: set-agent-balance requiere id y valor"
		}
		id := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		val := toFloat(e.eval(args[1], env))
		e.host.Lock()
		a, ok := e.host.GetAgent(id)
		if !ok {
			e.host.Unlock()
			return "error: agente no encontrado"
		}
		a.BalanceUTXO = val
		a.UltimaActual = time.Now().Unix()
		e.host.PutAgent(a)
		e.host.Unlock()
		e.host.PersistirLocamente()
		e.host.SincronizarConPares()
		return val
	}))
	// Alias from early Pulse Core saves
	e.globalEnv.SetFunction("set-balance", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		fn, _ := e.globalEnv.LookupFunction("set-agent-balance")
		if fn == nil {
			return "error"
		}
		return e.apply(fn, args, env)
	}))
	e.globalEnv.SetFunction("get-balance", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		fn, _ := e.globalEnv.LookupFunction("get-agent-balance")
		if fn == nil {
			return 0.0
		}
		return e.apply(fn, args, env)
	}))

	// --- Crypto / randomness ---
	e.globalEnv.SetFunction("random", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		return mathrand.Float64()
	}))

	e.globalEnv.SetFunction("sha256", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return ""
		}
		s := fmt.Sprintf("%v", e.eval(args[0], env))
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}))

	// --- List helpers ---
	e.globalEnv.SetFunction("elemento", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return nil
		}
		listVal := e.eval(args[0], env)
		list, ok := listVal.(LispList)
		if !ok {
			return "error: primer argumento debe ser lista"
		}
		idx := int(toFloat(e.eval(args[1], env)))
		if idx < 0 || idx >= len(list) {
			return nil
		}
		return list[idx]
	}))

	e.globalEnv.SetFunction("drop", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return LispList{}
		}
		list, ok := e.eval(args[0], env).(LispList)
		if !ok {
			return "error: drop requiere lista"
		}
		n := int(toFloat(e.eval(args[1], env)))
		if n <= 0 {
			return list
		}
		if n >= len(list) {
			return LispList{}
		}
		return list[n:]
	}))

	e.globalEnv.SetFunction("range", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return LispList{}
		}
		n := int(toFloat(e.eval(args[0], env)))
		if n < 0 {
			n = 0
		}
		if n > 10000 {
			n = 10000
		}
		out := make(LispList, n)
		for i := 0; i < n; i++ {
			out[i] = float64(i)
		}
		return out
	}))

	// --- Ternary encoding ---
	e.globalEnv.SetFunction("ternarizar", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return LispList{}
		}
		val := e.eval(args[0], env)
		list, ok := val.(LispList)
		if !ok {
			f := toFloat(val)
			if f < 0.33 {
				return 0.0
			}
			if f < 0.66 {
				return 1.0
			}
			return 2.0
		}
		out := make(LispList, len(list))
		for i, v := range list {
			f := toFloat(v)
			if f < 0.33 {
				out[i] = 0.0
			} else if f < 0.66 {
				out[i] = 1.0
			} else {
				out[i] = 2.0
			}
		}
		return out
	}))

	e.globalEnv.SetFunction("desternarizar", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return LispList{}
		}
		val := e.eval(args[0], env)
		list, ok := val.(LispList)
		if !ok {
			t := toFloat(val)
			if t <= 0 {
				return 0.0
			}
			if t >= 2 {
				return 1.0
			}
			return 0.5
		}
		out := make(LispList, len(list))
		for i, v := range list {
			t := toFloat(v)
			if t <= 0 {
				out[i] = 0.0
			} else if t >= 2 {
				out[i] = 1.0
			} else {
				out[i] = 0.5
			}
		}
		return out
	}))

	e.globalEnv.SetFunction("embedding", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 1 {
			return LispList{}
		}
		text := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		sum := sha256.Sum256([]byte(text))
		out := make(LispList, len(sum))
		for i, b := range sum {
			out[i] = float64(b) / 255.0
		}
		return out
	}))

	e.globalEnv.SetFunction("similitud", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return 0.0
		}
		a := e.eval(args[0], env)
		b := e.eval(args[1], env)
		var la, lb LispList
		if list, ok := a.(LispList); ok {
			la = list
		} else {
			// treat as text → embedding → ternarize
			embFn, _ := e.globalEnv.LookupFunction("embedding")
			terFn, _ := e.globalEnv.LookupFunction("ternarizar")
			if embFn == nil || terFn == nil {
				return 0.0
			}
			emb := e.apply(embFn, []LispValue{a}, env)
			la, _ = e.apply(terFn, []LispValue{emb}, env).(LispList)
		}
		if list, ok := b.(LispList); ok {
			lb = list
		} else {
			embFn, _ := e.globalEnv.LookupFunction("embedding")
			terFn, _ := e.globalEnv.LookupFunction("ternarizar")
			if embFn == nil || terFn == nil {
				return 0.0
			}
			emb := e.apply(embFn, []LispValue{b}, env)
			lb, _ = e.apply(terFn, []LispValue{emb}, env).(LispList)
		}
		n := len(la)
		if len(lb) < n {
			n = len(lb)
		}
		if n == 0 {
			return 0.0
		}
		eq := 0.0
		for i := 0; i < n; i++ {
			if toFloat(la[i]) == toFloat(lb[i]) {
				eq++
			}
		}
		return eq / float64(n)
	}))

	// --- Linear layers / models (ternary topology stored on agents) ---
	e.globalEnv.SetFunction("crear-capa-lineal", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 3 {
			return "error: se requiere (nombre entrada salida)"
		}
		nombre := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		entrada := int(toFloat(e.eval(args[1], env)))
		salida := int(toFloat(e.eval(args[2], env)))
		if entrada <= 0 || salida <= 0 {
			return "error: entrada y salida deben ser positivos"
		}
		if entrada > 512 || salida > 512 {
			return "error: dimensiones demasiado grandes (max 512)"
		}

		e.host.Lock()
		if _, exists := e.host.GetAgent(nombre); !exists {
			e.host.PutAgent(&agents.Agente{
				ID:           nombre,
				RootCID:      "",
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  1000.0,
			})
		}
		e.host.Unlock()

		topo := make(LispList, salida)
		for i := 0; i < salida; i++ {
			numConns := 2 + mathrand.Intn(4)
			if numConns > entrada+1 {
				numConns = entrada + 1
			}
			conns := make(LispList, numConns)
			for j := 0; j < numConns; j++ {
				if mathrand.Float64() < 0.55 {
					idx := -(1 + mathrand.Intn(entrada))
					conns[j] = float64(idx)
				} else if salida > 1 {
					conns[j] = float64(mathrand.Intn(salida))
				} else {
					conns[j] = float64(-1)
				}
			}
			topo[i] = conns
		}

		payload, err := json.Marshal(map[string]interface{}{
			"tipo":    "capa-lineal",
			"entrada": entrada,
			"salida":  salida,
			"topo":    topo,
		})
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		cid, err := e.host.GenerarCID(payload)
		if err != nil {
			return fmt.Sprintf("error cid: %v", err)
		}
		e.host.SetAgentRoot(nombre, cid)
		e.host.AnunciarNuevoBloque(cid)
		e.host.PersistirLocamente()
		return nombre
	}))

	e.globalEnv.SetFunction("crear-modelo", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requiere (nombre lista-capas)"
		}
		nombre := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		capasVal := e.eval(args[1], env)
		capas, ok := capasVal.(LispList)
		if !ok {
			return "error: segundo argumento debe ser lista de nombres de capa"
		}
		capasStr := make([]string, 0, len(capas))
		for _, c := range capas {
			capasStr = append(capasStr, strings.Trim(fmt.Sprintf("%v", c), "\""))
		}

		e.host.Lock()
		if _, exists := e.host.GetAgent(nombre); !exists {
			e.host.PutAgent(&agents.Agente{
				ID:           nombre,
				UltimaActual: time.Now().Unix(),
				BalanceUTXO:  1000.0,
			})
		}
		e.host.Unlock()

		payload, err := json.Marshal(map[string]interface{}{
			"tipo":  "modelo",
			"capas": capasStr,
		})
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		cid, err := e.host.GenerarCID(payload)
		if err != nil {
			return fmt.Sprintf("error cid: %v", err)
		}
		e.host.SetAgentRoot(nombre, cid)
		e.host.AnunciarNuevoBloque(cid)
		e.host.PersistirLocamente()
		return nombre
	}))

	e.globalEnv.SetFunction("inferir", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: se requiere (modelo-id entrada)"
		}
		modeloID := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		entrada := e.eval(args[1], env)

		e.host.RLock()
		mod, ok := e.host.GetAgent(modeloID)
		e.host.RUnlock()
		if !ok || mod.RootCID == "" {
			return "error: modelo no encontrado"
		}
		raw, err := e.host.BuscarContenidoPorCID(mod.RootCID)
		if err != nil {
			return fmt.Sprintf("error cargando modelo: %v", err)
		}
		var meta map[string]interface{}
		if err := json.Unmarshal(raw, &meta); err != nil {
			return fmt.Sprintf("error parseando modelo: %v", err)
		}
		capasRaw, _ := meta["capas"].([]interface{})
		if len(capasRaw) == 0 {
			return "error: modelo sin capas"
		}

		zyrionNet, _ := e.globalEnv.LookupFunction("zyrion-network")
		if zyrionNet == nil {
			return "error: zyrion-network no disponible"
		}

		actual := entrada
		// Ensure continuous vectors are ternarized before first layer
		if list, ok := actual.(LispList); ok && len(list) > 0 {
			if f := toFloat(list[0]); f != 0 && f != 1 && f != 2 && !(f >= 0 && f <= 2 && float64(int(f)) == f) {
				terFn, _ := e.globalEnv.LookupFunction("ternarizar")
				if terFn != nil {
					actual = e.apply(terFn, []LispValue{actual}, env)
				}
			}
		}

		for _, cn := range capasRaw {
			capaID := fmt.Sprintf("%v", cn)
			e.host.RLock()
			capa, ok := e.host.GetAgent(capaID)
			e.host.RUnlock()
			if !ok || capa.RootCID == "" {
				return fmt.Sprintf("error: capa %s no encontrada", capaID)
			}
			craw, err := e.host.BuscarContenidoPorCID(capa.RootCID)
			if err != nil {
				return fmt.Sprintf("error capa %s: %v", capaID, err)
			}
			var cmeta map[string]interface{}
			if err := json.Unmarshal(craw, &cmeta); err != nil {
				return fmt.Sprintf("error parse capa %s", capaID)
			}
			topoIface, ok := cmeta["topo"]
			if !ok {
				return fmt.Sprintf("error: capa %s sin topologia", capaID)
			}
			topo := jsonToLispList(topoIface)
			// zyrion-network expects (topology externals-list)
			// externals: single vector as one external set → pass as list of values
			extList, ok := actual.(LispList)
			if !ok {
				return "error: entrada debe ser lista"
			}
			res := e.apply(zyrionNet, []LispValue{topo, extList}, env)
			actual = res
		}
		return actual
	}))

	e.globalEnv.SetFunction("entrenar-hebbiano", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 3 {
			return "error: (modelo-id dataset epocas [tasa])"
		}
		modeloID := strings.Trim(fmt.Sprintf("%v", e.eval(args[0], env)), "\"")
		dataset, ok := e.eval(args[1], env).(LispList)
		if !ok {
			return "error: dataset debe ser lista de pares"
		}
		epocas := int(toFloat(e.eval(args[2], env)))
		if epocas <= 0 {
			epocas = 1
		}
		if epocas > 500 {
			epocas = 500 // safety cap
		}
		tasa := 0.05
		if len(args) >= 4 {
			tasa = toFloat(e.eval(args[3], env))
		}

		inferFn, _ := e.globalEnv.LookupFunction("inferir")
		if inferFn == nil {
			return "error: inferir no disponible"
		}

		var lastLoss float64
		for epoch := 0; epoch < epocas; epoch++ {
			aciertos := 0.0
			total := 0.0
			for _, item := range dataset {
				pair, ok := item.(LispList)
				if !ok || len(pair) < 2 {
					continue
				}
				entrada := pair[0]
				esperado := pair[1]
				salida := e.apply(inferFn, []LispValue{modeloID, entrada}, env)
				outList, _ := salida.(LispList)
				expList, _ := esperado.(LispList)
				if outList == nil || expList == nil {
					continue
				}
				n := len(outList)
				if len(expList) < n {
					n = len(expList)
				}
				match := 0.0
				for i := 0; i < n; i++ {
					if toFloat(outList[i]) == toFloat(expList[i]) {
						match++
					}
				}
				if n > 0 {
					aciertos += match / float64(n)
					total++
				}
				// Light random topology nudge on mismatch (Hebbian-inspired exploration)
				if n > 0 && match/float64(n) < 0.5 && mathrand.Float64() < tasa {
					mutarFn, _ := e.globalEnv.LookupFunction("mutar-topologia")
					if mutarFn != nil {
						e.host.RLock()
						mod, okm := e.host.GetAgent(modeloID)
						e.host.RUnlock()
						if okm && mod.RootCID != "" {
							raw, _ := e.host.BuscarContenidoPorCID(mod.RootCID)
							var meta map[string]interface{}
							if json.Unmarshal(raw, &meta) == nil {
								if capas, ok := meta["capas"].([]interface{}); ok && len(capas) > 0 {
									capaID := fmt.Sprintf("%v", capas[mathrand.Intn(len(capas))])
									e.host.RLock()
									capa, okc := e.host.GetAgent(capaID)
									e.host.RUnlock()
									if okc && capa.RootCID != "" {
										craw, _ := e.host.BuscarContenidoPorCID(capa.RootCID)
										var cmeta map[string]interface{}
										if json.Unmarshal(craw, &cmeta) == nil {
											topo := jsonToLispList(cmeta["topo"])
											mutated := e.apply(mutarFn, []LispValue{topo, tasa}, env)
											cmeta["topo"] = mutated
											if nb, err := json.Marshal(cmeta); err == nil {
												if cid, err := e.host.GenerarCID(nb); err == nil {
													e.host.SetAgentRoot(capaID, cid)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
			if total > 0 {
				lastLoss = aciertos / total
			}
		}
		e.host.PersistirLocamente()
		return lastLoss
	}))

	_ = math.Abs // keep math available if needed later
}

// jsonToLispList converts JSON-decoded nested arrays into LispList of floats/lists.
func jsonToLispList(v interface{}) LispList {
	arr, ok := v.([]interface{})
	if !ok {
		// already LispList from roundtrip
		if l, ok := v.(LispList); ok {
			return l
		}
		return LispList{}
	}
	out := make(LispList, len(arr))
	for i, item := range arr {
		switch t := item.(type) {
		case []interface{}:
			out[i] = jsonToLispList(t)
		case float64:
			out[i] = t
		case int:
			out[i] = float64(t)
		default:
			out[i] = toFloat(t)
		}
	}
	return out
}
