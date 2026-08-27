package node

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// loadLocalEnvFiles reads KEY=VALUE into process env if key not already set.
// Paths: alset_data/cloudflare.env, alset_data/local.env, .env
func loadLocalEnvFiles() {
	paths := []string{
		filepath.Join("alset_data", "cloudflare.env"),
		filepath.Join("alset_data", "local.env"),
		".env",
	}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimPrefix(line, "export ")
			i := strings.IndexByte(line, '=')
			if i <= 0 {
				continue
			}
			k := strings.TrimSpace(line[:i])
			v := strings.TrimSpace(line[i+1:])
			v = strings.Trim(v, `"'`)
			if k == "" {
				continue
			}
			if os.Getenv(k) == "" {
				_ = os.Setenv(k, v)
			}
		}
		_ = f.Close()
	}
}
