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
	pkgPath := flag.String("package", "", "ruta al JSON FrontierPackage")
	pkgURL := flag.String("package-url", "", "URL del paquete (by-cid o gateway IPFS)")
	httpAddr := flag.String("http", ":9090", "dirección HTTP del daemon")
	dataDir := flag.String("data", "gen_data", "directorio de datos (clave libp2p, names)")
	p2p := flag.Bool("p2p", false, "activar host libp2p propio")
	announce := flag.String("announce", "", "URL base del nodo Alset/Mind (ej. https://prismatec.onrender.com)")
	publicURL := flag.String("public-url", "", "URL pública con la que Mind te alcanza (ngrok, IP:puerto, etc.)")
	udpPort := flag.Int("udp", 0, "puerto UDP Pulse (ej. 9091); 0 = desactivado")
	flag.Parse()

	if *pkgPath == "" && *pkgURL == "" {
		fmt.Fprintln(os.Stderr, "uso: alset-gen -package <file.json> | -package-url <url> [-http :9090] [-udp 9091] [-announce ...]")
		os.Exit(2)
	}
	var pkg *gennode.FrontierPackage
	var err error
	if *pkgURL != "" {
		pkg, err = gennode.LoadPackageURL(*pkgURL)
	} else {
		pkg, err = gennode.LoadPackageFile(*pkgPath)
	}
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
		Pkg:         pkg,
		DataDir:     *dataDir,
		HTTPAddr:    *httpAddr,
		EnableP2P:   *p2p,
		AnnounceURL: *announce,
		PublicURL:   *publicURL,
	}
	log.Printf("Semilla %s · root %s · modo autónomo", pkg.Key, short(pkg.CurrentRootCID))
	if *udpPort > 0 {
		if err := d.StartUDP(*udpPort); err != nil {
			log.Printf("⚠️ UDP: %v", err)
		}
	}
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
