// Command foundry runs Foundry: `foundry serve` for the UI + API, `foundry mcp`
// for AI hosts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	web "github.com/handrawanw/foundry/apps/web"
	"github.com/handrawanw/foundry/internal/core"
	"github.com/handrawanw/foundry/internal/modules/pipeline"
	"github.com/handrawanw/foundry/internal/modules/render"
	"github.com/handrawanw/foundry/internal/transport/httpapi"
	"github.com/handrawanw/foundry/internal/transport/mcp"
)

var version = "dev"

const usage = `Foundry — ide sampai aplikasi siap diperiksa

  foundry serve [--addr 127.0.0.1:8080] [folder-project]
      Buka UI: Hubungkan AI → Ceritakan → Rancang → Bangun → Siap
      Tanpa folder: halaman Start untuk Open/New/Close Project

  foundry mcp [--allow READ,WRITE,MODIFY,EXECUTE|ALL] [--project folder] [--print-config]
      Server MCP (stdio) untuk AI Host: Cursor, Claude, Codex, OpenCode, Windsurf, dll.

  foundry metrics [folder-project]
      Ukuran prompt (baseline vs terfilter) dan metrik run agent. Hanya baca.

  foundry version
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	switch os.Args[1] {
	case "serve":
		serve(os.Args[2:])
	case "mcp":
		runMCP(os.Args[2:])
	case "metrics":
		runMetrics(os.Args[2:])
	case "version":
		fmt.Println(version)
	default:
		fmt.Print(usage)
		os.Exit(2)
	}
}

func serve(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	addr := fs.String("addr", "127.0.0.1:8080", "alamat listen")
	uiURL := fs.String("ui-url", "", "URL UI Foundry untuk render pemeriksaan visual (default: http://<addr>)")
	fs.Parse(args)
	dir := fs.Arg(0)
	bin, _ := os.Executable()
	srv := httpapi.New(web.FS(), bin)
	if *uiURL == "" && web.FS() != nil {
		// ":8081" listens on all interfaces; Chrome needs a concrete host.
		host := *addr
		if strings.HasPrefix(host, ":") {
			host = "127.0.0.1" + host
		}
		*uiURL = "http://" + host
	}
	if r := render.Find(*uiURL); r != nil {
		srv.Render = r
	} else {
		log.Printf("Pemeriksaan visual nonaktif: butuh Chrome/Chromium dan UI (--ui-url saat dev)")
	}
	// Without a folder argument the UI starts on the Start page (Open / New).
	if dir != "" {
		if _, err := srv.Open(dir, "", true); err != nil {
			log.Fatalf("buka project: %v", err)
		}
	}
	hs := &http.Server{Addr: *addr, Handler: srv.Handler()}
	onStop(func() {
		if err := srv.Shutdown(shutdownTimeout); err != nil {
			log.Printf("shutdown: %v", err)
		}
		hs.Close()
	})
	log.Printf("Foundry: http://%s", *addr)
	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func runMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	allow := fs.String("allow", "READ", "READ, WRITE, MODIFY, EXECUTE atau ALL")
	dir := fs.String("project", "", "folder project (default: folder saat ini)")
	printCfg := fs.Bool("print-config", false, "cetak konfigurasi MCP client")
	uiURL := fs.String("ui-url", "http://127.0.0.1:8080", "URL UI Foundry (foundry serve) untuk render pemeriksaan visual")
	fs.Parse(args)
	if *dir == "" {
		*dir, _ = os.Getwd()
	}
	st, err := core.NewStore(*dir)
	if err != nil {
		log.Fatal(err)
	}
	if *printCfg {
		bin, _ := os.Executable()
		b, _ := json.MarshalIndent(mcp.ClientConfig(bin, st.Dir), "", "  ")
		fmt.Println(string(b))
		return
	}
	perms, err := mcp.ParseAllow(*allow)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := st.Init(""); err != nil {
		log.Fatal(err)
	}
	// stdout carries the protocol; logs go to stderr.
	log.SetOutput(os.Stderr)
	svc := pipeline.New(st, pipeline.ConfigAI{})
	if r := render.Find(*uiURL); r != nil {
		svc.Render = r
	}
	s := &mcp.Server{Svc: svc, Allow: perms, Version: version}
	stop := func() {
		if err := svc.Shutdown(shutdownTimeout); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}
	onStop(func() { stop(); os.Exit(1) })
	err = s.Serve(os.Stdin, os.Stdout)
	stop() // the host closed stdin: nothing it started may keep running
	if err != nil {
		log.Fatal(err)
	}
}

// shutdownTimeout bounds how long stopping waits for jobs to end.
const shutdownTimeout = 15 * time.Second

// onStop runs fn once on the first interrupt, terminate or hangup signal.
// Commands run in their own process group, so a terminal's Ctrl+C or
// hangup reaches only Foundry, and Foundry must stop them itself.
func onStop(fn func()) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-sig
		fn()
	}()
}
