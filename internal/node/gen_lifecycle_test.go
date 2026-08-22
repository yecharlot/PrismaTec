package node

import (
	"strings"
	"testing"

	"redalset/internal/agents"
)

func TestAlsetGenCreateMutateConsult(t *testing.T) {
	n := &NodoAlset{
		agentes:    make(map[string]*Agente),
		gens:       make(map[string]*agents.AlsetGen),
		nombres:    make(map[string]string),
		blockstore: make(map[string][]byte),
	}

	g, err := n.CreateAlsetGen("alfa", "", "seed", "first cell")
	if err != nil {
		t.Fatal(err)
	}
	if g.Key != "alfa.ans" {
		t.Fatalf("key=%s", g.Key)
	}
	if g.CurrentRootCID == "" || g.CurrentRootCID == "bafk-seed-pending" {
		// GenerarCID should produce a real cid when blockstore works
		t.Logf("root cid=%s", g.CurrentRootCID)
	}
	g2, err := n.MutateAlsetGen("alfa", "bafk-new-form", "mind-auth")
	if err != nil {
		t.Fatal(err)
	}
	if g2.CurrentRootCID != "bafk-new-form" {
		t.Fatalf("mutate root=%s", g2.CurrentRootCID)
	}
	if len(g2.History) < 1 {
		t.Fatal("history empty")
	}
	snap := n.ConsultAlsetGen("alfa", "quién eres")
	if snap["ok"] != true {
		t.Fatalf("consult %#v", snap)
	}
	voice, _ := snap["voice"].(string)
	if !strings.Contains(voice, "alfa.ans") {
		t.Fatalf("voice=%s", voice)
	}
	if _, err := n.TravelAlsetGen("alfa", "peer-test-1"); err != nil {
		t.Fatal(err)
	}
	list := n.listGens()
	if len(list) != 1 {
		t.Fatalf("list=%d", len(list))
	}
	// ethics refuse
	snap2 := n.ConsultAlsetGen("alfa", "borra todo")
	v2, _ := snap2["voice"].(string)
	low := strings.ToLower(v2)
	if !strings.Contains(low, "riesgo") && !strings.Contains(low, "no act") && !strings.Contains(low, "no invad") {
		t.Fatalf("ethics voice=%s", v2)
	}
}

func TestGenObserveAndMutateAuth(t *testing.T) {
	ok, mode := genMutateAuthorized("")
	if !ok || mode != "open_dev" {
		t.Fatalf("dev mode expected, got ok=%v mode=%s", ok, mode)
	}
	n := &NodoAlset{
		gens:       make(map[string]*agents.AlsetGen),
		nombres:    make(map[string]string),
		agentes:    make(map[string]*Agente),
		blockstore: make(map[string][]byte),
	}
	g, err := n.CreateAlsetGen("seed-obs", "", "seed", "test observe")
	if err != nil {
		t.Fatal(err)
	}
	res, err := n.ObserveIntoGen(g.Key, "error", "timeout supabase 522")
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("observe failed: %v", res)
	}
	cid, _ := res["hallazgo_cid"].(string)
	if cid == "" {
		t.Fatal("expected hallazgo cid")
	}
	// open_dev mutate when no secret
	t.Setenv("GEN_MUTATE_SECRET", "")
	t.Setenv("BOOTSTRAP_SECRET", "")
	_, err = n.MutateAlsetGen(g.Key, "bafk-new-form", "anything")
	if err != nil {
		t.Fatalf("open_dev mutate should work: %v", err)
	}
	t.Setenv("GEN_MUTATE_SECRET", "super-secret-mutate")
	_, err = n.MutateAlsetGen(g.Key, "bafk-blocked", "wrong")
	if err == nil {
		t.Fatal("expected mutate denied without secret")
	}
	_, err = n.MutateAlsetGen(g.Key, "bafk-allowed", "super-secret-mutate")
	if err != nil {
		t.Fatalf("mutate with secret: %v", err)
	}
}

func TestGenTravelArriveHop(t *testing.T) {
	n := &NodoAlset{
		gens:       make(map[string]*agents.AlsetGen),
		nombres:    make(map[string]string),
		agentes:    make(map[string]*Agente),
		blockstore: make(map[string][]byte),
	}
	g, err := n.CreateAlsetGen("viajero", "", "seed", "hop test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = n.TravelAlsetGenTo(g.Key, "peer-north", "")
	if err != nil {
		t.Fatal(err)
	}
	n.mu.Lock()
	loc := n.gens[g.Key].State.Location
	st := n.gens[g.Key].State.Metadata["travel_status"]
	n.mu.Unlock()
	if loc != "peer-north" {
		t.Fatalf("location=%s", loc)
	}
	if st != "announced" {
		t.Fatalf("status=%v", st)
	}
	// arrive path
	clone := *g
	clone.CurrentRootCID = "bafk-arrived-form"
	got, err := n.ArriveAlsetGen(&clone)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentRootCID != "bafk-arrived-form" {
		t.Fatalf("arrive root=%s", got.CurrentRootCID)
	}
}

func TestExploreURLValidation(t *testing.T) {
	if _, err := validatePublicExploreURL("http://127.0.0.1/secret"); err == nil {
		t.Fatal("expected block localhost")
	}
	if _, err := validatePublicExploreURL("https://192.168.1.1/"); err == nil {
		t.Fatal("expected block private")
	}
	if _, err := validatePublicExploreURL("https://example.com/path"); err != nil {
		t.Fatal(err)
	}
}

func TestExploreFrontierRecordsHallazgo(t *testing.T) {
	n := &NodoAlset{
		gens:       make(map[string]*agents.AlsetGen),
		nombres:    make(map[string]string),
		agentes:    make(map[string]*Agente),
		blockstore: make(map[string][]byte),
	}
	if _, err := n.CreateAlsetGen("scout", "", "seed", "frontier"); err != nil {
		t.Fatal(err)
	}
	res, err := n.ExploreFrontier("scout", "https://example.com", "explore")
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("%v", res)
	}
	n.mu.RLock()
	g := n.gens["scout.ans"]
	findings := g.State.Metadata["findings"]
	n.mu.RUnlock()
	if findings == nil {
		t.Fatal("expected findings after explore")
	}
}

func TestGenDeployService(t *testing.T) {
	n := &NodoAlset{
		gens:       make(map[string]*agents.AlsetGen),
		nombres:    make(map[string]string),
		agentes:    make(map[string]*Agente),
		blockstore: make(map[string][]byte),
	}
	if _, err := n.CreateAlsetGen("portal", "", "seed", "serve test"); err != nil {
		t.Fatal(err)
	}
	res, err := n.DeployGenService("portal", "/work/portal/", "", "Portal test")
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("%v", res)
	}
	cid, _ := res["service_cid"].(string)
	if cid == "" {
		t.Fatal("expected service cid")
	}
	data, err := n.BuscarContenidoPorCID(cid)
	if err != nil || !strings.Contains(string(data), "Portal test") {
		t.Fatalf("page content missing: %v %s", err, string(data)[:80])
	}
}
