package agents

import "sync"

type Registry struct {
	MuModulos   sync.RWMutex
	MuEntidades sync.RWMutex
	MuTokens    sync.RWMutex
	Modulos     map[string]*Modulo
	Entidades   map[string]*EntidadProgramatica
	Relaciones  map[string]*RelacionEntidad
	Tokens      map[string]*TokenAlset
	Roles       map[string][]string
}

func NewRegistry() *Registry {
	return &Registry{
		Modulos:    make(map[string]*Modulo),
		Entidades:  make(map[string]*EntidadProgramatica),
		Relaciones: make(map[string]*RelacionEntidad),
		Tokens:     make(map[string]*TokenAlset),
		Roles:      make(map[string][]string),
	}
}

var Global = NewRegistry()
