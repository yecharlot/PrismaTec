package lisp

import (
	"fmt"
	"strings"
)

// registerEvaluarZyrion adds the high-level ternary decision DSL:
//
//	(evaluar-zyrion
//	  '(NODO :entradas (A B) :salidas ((0 ACCION0) (1 ACCION1) (2 SUB-O-ACCION)))
//	  '(A 0.9 B 0.2 Valor_Continuous_Neural 0.5))
//
// Continuous env values are mapped to ternary: <0.33→0, <0.66→1, else→2.
// A salida of 2 may be a nested node list (same shape) or the symbol INVOCAR_RED_NEURONAL.
func (e *Evaluator) registerEvaluarZyrion() {
	e.globalEnv.SetFunction("evaluar-zyrion", LispFunction(func(args []LispValue, env *LispEnvironment) LispValue {
		if len(args) < 2 {
			return "error: evaluar-zyrion requiere topologia y entorno"
		}
		// Args are already evaluated by the interpreter.
		topo := args[0]
		entorno := args[1]
		envMap := parseEntorno(entorno)
		return e.evalZyrionNode(topo, envMap, env, 0)
	}))
}

func parseEntorno(v LispValue) map[string]float64 {
	out := make(map[string]float64)
	list, ok := v.(LispList)
	if !ok {
		return out
	}
	// Support (K1 V1 K2 V2 ...) and ((K1 V1) (K2 V2) ...)
	i := 0
	for i < len(list) {
		item := list[i]
		if pair, ok := item.(LispList); ok && len(pair) >= 2 {
			key := strings.Trim(fmt.Sprintf("%v", pair[0]), "\"")
			out[key] = toFloat(pair[1])
			i++
			continue
		}
		if i+1 >= len(list) {
			break
		}
		key := strings.Trim(fmt.Sprintf("%v", item), "\"")
		out[key] = toFloat(list[i+1])
		i += 2
	}
	return out
}

func continuousToTernary(f float64) float64 {
	// Already ternary
	if f == 0 || f == 1 || f == 2 {
		return f
	}
	if f < 0.33 {
		return 0
	}
	if f < 0.66 {
		return 1
	}
	return 2
}

func keywordName(v LispValue) string {
	s := strings.Trim(fmt.Sprintf("%v", v), "\"")
	s = strings.TrimPrefix(s, ":")
	return strings.ToLower(s)
}

// extractNodeFields reads (NAME :entradas (...) :salidas (...)) or without name.
func extractNodeFields(node LispValue) (name string, entradas LispList, salidas LispList, err string) {
	list, ok := node.(LispList)
	if !ok || len(list) == 0 {
		return "", nil, nil, "nodo inválido"
	}
	start := 0
	// Optional leading name symbol (not a keyword)
	if kw := keywordName(list[0]); kw != "entradas" && kw != "salidas" {
		name = fmt.Sprintf("%v", list[0])
		start = 1
	}
	for i := start; i < len(list); i++ {
		kw := keywordName(list[i])
		if kw == "entradas" && i+1 < len(list) {
			if el, ok := list[i+1].(LispList); ok {
				entradas = el
			}
			i++
			continue
		}
		if kw == "salidas" && i+1 < len(list) {
			if el, ok := list[i+1].(LispList); ok {
				salidas = el
			}
			i++
			continue
		}
	}
	if len(entradas) == 0 || len(salidas) == 0 {
		return name, entradas, salidas, "faltan :entradas o :salidas"
	}
	return name, entradas, salidas, ""
}

func (e *Evaluator) evalZyrionNode(node LispValue, envMap map[string]float64, env *LispEnvironment, depth int) LispValue {
	if depth > 32 {
		return "error: profundidad máxima de subnodos"
	}
	_, entradas, salidas, errMsg := extractNodeFields(node)
	if errMsg != "" {
		return "error: " + errMsg
	}

	// Collect ternary inputs for this node
	vals := make([]float64, 0, len(entradas))
	for _, ent := range entradas {
		key := strings.Trim(fmt.Sprintf("%v", ent), "\"")
		f, ok := envMap[key]
		if !ok {
			f = 0
		}
		vals = append(vals, continuousToTernary(f))
	}

	zyrionFn, _ := e.globalEnv.LookupFunction("zyrion")
	if zyrionFn == nil {
		return "error: zyrion no disponible"
	}
	inputList := make(LispList, len(vals))
	for i, v := range vals {
		inputList[i] = v
	}
	estado := toFloat(e.apply(zyrionFn, []LispValue{inputList}, env))
	// estado is 0, 1 or 2

	// Match salida: ((0 ACCION) (1 ACCION) (2 ...))
	var chosen LispValue
	found := false
	for _, s := range salidas {
		pair, ok := s.(LispList)
		if !ok || len(pair) < 2 {
			// Support dotted style was ((0 . SYM)) which may parse differently;
			// also (0 ACCION) flat pairs already handled.
			continue
		}
		key := toFloat(pair[0])
		if key == estado {
			chosen = pair[1]
			// If more elements after key, treat rest as list action
			if len(pair) > 2 {
				chosen = pair[1:]
			}
			found = true
			break
		}
	}
	if !found {
		return fmt.Sprintf("error: sin salida para estado %.0f", estado)
	}

	return e.resolveZyrionAction(chosen, envMap, env, depth)
}

func (e *Evaluator) resolveZyrionAction(action LispValue, envMap map[string]float64, env *LispEnvironment, depth int) LispValue {
	// Nested node: list starting with symbol and containing :entradas
	if list, ok := action.(LispList); ok && len(list) > 0 {
		for _, item := range list {
			if keywordName(item) == "entradas" {
				return e.evalZyrionNode(list, envMap, env, depth+1)
			}
		}
		// List without :entradas — join as string label
		parts := make([]string, 0, len(list))
		for _, item := range list {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, " ")
	}

	label := strings.Trim(fmt.Sprintf("%v", action), "\"")
	upper := strings.ToUpper(label)
	if upper == "INVOCAR_RED_NEURONAL" || strings.Contains(upper, "INVOCAR_RED_NEURONAL") {
		return e.invokeNeuralFallback(envMap, env)
	}
	return label
}

func (e *Evaluator) invokeNeuralFallback(envMap map[string]float64, env *LispEnvironment) LispValue {
	// Prefer explicit continuous neural value if provided
	if v, ok := envMap["Valor_Continuous_Neural"]; ok {
		t := continuousToTernary(v)
		return fmt.Sprintf("INVOCAR_RED_NEURONAL:%.2f->%.0f", v, t)
	}
	inferFn, _ := e.globalEnv.LookupFunction("local-inference")
	if inferFn != nil {
		// Build a small vector from env values
		vals := make(LispList, 0, len(envMap))
		for _, v := range envMap {
			vals = append(vals, v)
		}
		if len(vals) == 0 {
			vals = LispList{0.5}
		}
		res := e.apply(inferFn, []LispValue{vals}, env)
		return fmt.Sprintf("INVOCAR_RED_NEURONAL:%v", res)
	}
	return "INVOCAR_RED_NEURONAL"
}
