// alset-gen: daemon autónomo de una célula Alset-Gen (fuera del monolito PrismaTec).
//
//	go run ./cmd/alset-gen -package ./demo-cell.package.json -http :9090
//	go run ./cmd/alset-gen -package ./demo-cell.package.json -http :9090 -p2p
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"redalset/internal/gennode"
)

func main() {
	pkgPath := flag.String("package", "", "ruta al JSON FrontierPackage (obligatorio)")
	httpAddr := flag.String("http", ":9090", "dirección HTTP del daemon")
	dataDir := flag.String("data", "gen_data", "directorio de datos (clave libp2p, names)")
	p2p := flag.Bool("p2p", false, "activar host libp2p propio")
	flag.Parse()

	if *pkgPath == "" {
		fmt.Fprintln(os.Stderr, "uso: alset-gen -package <archivo.json> [-http :9090] [-p2p]")
		fmt.Fprintln(os.Stderr, "El archivo es un FrontierPackage (type alset_gen_frontier_package).")
		os.Exit(2)
	}

	pkg, err := gennode.LoadPackageFile(*pkgPath)
	if err != nil {
		log.Fatalf("paquete: %v", err)
	}

	_ = os.MkdirAll(*dataDir, 0o755)
	if b, err := json.MarshalIndent(pkg, "", "  "); err == nil {
		_ = os.WriteFile(*dataDir+"/package.json", b, 0o644)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	d := &gennode.Daemon{
		Pkg:       pkg,
		DataDir:   *dataDir,
		HTTPAddr:  *httpAddr,
		EnableP2P: *p2p,
	}
	log.Printf("Semilla %s · root %s · modo autónomo", pkg.Key, short(pkg.CurrentRootCID))
	err = d.Start(ctx)
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func short(s string) string {
	if len(s) > 16 {
		return s[:16] + "…"
	}
	return s
}
