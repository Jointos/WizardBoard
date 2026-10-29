// Command arena-report renders a self-contained HTML balance report from an
// arena statistics JSON file. It does not play games and does not touch the
// rankings: statistics only.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wizardboardgame/internal/arena"
)

func main() {
	in := flag.String("in", "", "arena stats JSON (e.g. data/balance/balance_mlv2_2026-10-26_14-05-33.json)")
	out := flag.String("out", "", "HTML output path (required)")
	stamp := flag.Bool("stamp", false, "append a _YYYY-MM-DD_HH-MM-SS timestamp to -out")
	flag.Parse()

	if *in == "" {
		fatal(fmt.Errorf("-in is required"))
	}
	if *out == "" {
		fatal(fmt.Errorf("-out is required"))
	}

	sf, err := arena.LoadStats(*in)
	if err != nil {
		fatal(err)
	}
	path := stampPath(*out, *stamp)
	if err := arena.WriteHTML(path, *in, sf); err != nil {
		fatal(err)
	}
	fmt.Printf("report: %s\n", path)
}

func stampPath(path string, stamp bool) string {
	if !stamp || path == "" {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return base + "_" + time.Now().Format("2006-01-02_15-04-05") + ext
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "arena-report:", err)
	os.Exit(1)
}
