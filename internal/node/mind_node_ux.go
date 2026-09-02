package node

import (
	"strings"
)

// P5 — comandos humanizados del nodo / red Alset / Mind / Zyrion / agentes.

func normalizeNodeUserIntent(text string) string {
	low := strings.ToLower(strings.TrimSpace(text))
	if low == "" {
		return text
	}

	// Estado / cuerpo del nodo
	if matchAny(low, []string{
		"mira el nodo", "mirar el nodo", "estado del nodo", "cómo está el nodo", "como esta el nodo",
		"qué ves en el nodo", "que ves en el nodo", "muéstrame el nodo", "muestrame el nodo",
		"dime el estado", "estado general", "snapshot", "cuerpo del nodo", "qué hay en el nodo",
		"que hay en el nodo", "revisa el nodo", "inspecciona el nodo",
	}) {
		return "dame estado del nodo"
	}

	// Red / peers
	if matchAny(low, []string{
		"cómo está la red", "como esta la red", "estado de la red", "hay peers", "cuántos peers",
		"cuantos peers", "quién está conectado", "quien esta conectado", "mira la red",
		"conexión p2p", "conexion p2p", "peers activos",
	}) {
		return "dame estado de la red"
	}

	// Agentes
	if matchAny(low, []string{
		"qué agentes", "que agentes", "cuántos agentes", "cuantos agentes", "lista de agentes",
		"agentes del nodo", "quiénes son los agentes", "quienes son los agentes",
	}) {
		return "dame agentes"
	}

	// Apps
	if matchAny(low, []string{
		"qué apps", "que apps", "qué aplicaciones", "que aplicaciones", "apps del nodo",
		"aplicaciones instaladas", "lista de apps",
	}) {
		return "dame apps"
	}

	// Zyrion / órganos / checkpoint
	if matchAny(low, []string{
		"evalúa zyrion", "evalua zyrion", "evaluación zyrion", "evaluacion zyrion",
		"muéstrame los órganos", "muestrame los organos", "estado de los órganos", "estado de los organos",
		"cómo están los órganos", "como estan los organos", "checkpoint", "demo zyrion",
	}) {
		return "evalua zyrion"
	}

	// Genoma
	if matchAny(low, []string{
		"enséñame el genoma", "ensename el genoma", "muestra el genoma", "cómo está el genoma",
		"como esta el genoma", "umbrales actuales", "dame el genoma",
	}) && !strings.Contains(low, "qué es el genoma") && !strings.Contains(low, "que es el genoma") {
		return "explica genoma" // knowledge path often; also status-adjacent
	}

	return text
}

func isHumanNodeIntent(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	n := normalizeNodeUserIntent(s)
	return n != s
}

// speakOperatorHelp: guía humana para sacar el máximo al nodo (usuario autorizado).
func speakOperatorHelp(s string) string {
	low := strings.ToLower(strings.TrimSpace(s))
	// triggers
	help := matchAny(low, []string{
		"qué puedo hacer con el nodo", "que puedo hacer con el nodo",
		"cómo uso el nodo", "como uso el nodo", "ayuda del nodo", "manual del nodo",
		"comandos del nodo", "guía de comandos", "guia de comandos",
		"cómo saco provecho", "como saco provecho", "máximo al nodo", "maximo al nodo",
		"qué sé hacer aquí", "que se hacer aqui", "operar el nodo",
	})
	if !help {
		// "ayuda" + nodo/red/alset
		if strings.Contains(low, "ayuda") && matchAny(low, []string{"nodo", "red", "alset", "operar", "comandos"}) {
			help = true
		}
	}
	if !help {
		return ""
	}
	return strings.TrimSpace(`Puedes operar este nodo en lenguaje natural. No hace falta jerga de laboratorio.

**Yo (Mind)**
· «quién eres», «de qué estás hecho», «cómo te consideras»
· «qué puedes hacer», «cómo usas la memoria»

**Nodo y red**
· «mira el nodo» / «dame el estado»
· «cómo está la red» / «cuántos peers»
· «qué agentes hay» / «qué apps hay»

**Zyrion y órganos**
· «evalúa zyrion» / «cómo están los órganos»
· «qué es el estado 2» / «alarma absorbente»

**Sondas (genes)**
· «cómo uso las sondas»
· «crea una sonda llamada X»
· «manda la sonda X al borde»
· «que explore https://…»
· «trae la sonda X» / «elimina la sonda X»

**Memoria y hechos**
· «me llamo …», «vivo en …»
· «cómo me llamo», «dónde vivo»

**Cálculo y código**
· «cuánto es 12*7»
· «crea una función hola mundo»

**Límites**
No ejecuto borrados masivos, ni acceso a cuentas ajenas. Una sonda concreta sí se puede crear, enviar, traer o eliminar.

Pide un área («sondas», «red», «órganos») y bajamos al detalle.`)
}

// expandWantStatusHuman: frases que deben abrir snapshot aunque no digan "estado".
func expandWantStatusHuman(s string) bool {
	s = strings.ToLower(s)
	return matchAny(s, []string{
		"mira el nodo", "estado del nodo", "dame estado", "cuerpo del nodo",
		"qué ves en el nodo", "que ves en el nodo", "inspecciona el nodo",
		"cómo está el nodo", "como esta el nodo", "revisa el nodo",
		"dame agentes", "qué agentes", "que agentes", "dame apps",
		"dame estado de la red", "cómo está la red", "como esta la red",
		"cuántos peers", "cuantos peers", "peers activos",
	}) || normalizeNodeUserIntent(s) != s && matchAny(normalizeNodeUserIntent(s), []string{
		"dame estado", "dame agentes", "dame apps", "dame estado de la red",
	})
}
