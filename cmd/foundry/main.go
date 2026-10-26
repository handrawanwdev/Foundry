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

	web "github.com/handrawanwdev/foundry/apps/web"
	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/pipeline"
	"github.com/handrawanwdev/foundry/internal/modules/work"
	"github.com/handrawanwdev/foundry/internal/transport/httpapi"
	"github.com/handrawanwdev/foundry/internal/transport/mcp"
)

var version = "dev"

const usage = `Foundry — dari prompt ke rancangan aplikasi dan spesifikasi final

  foundry serve [--addr 127.0.0.1:8080] [folder-project]
      Buka UI: Hubungkan AI → Prompt → Rancang (preview + revisi) → Finalisasi
      Tanpa folder: halaman Start untuk Open/New/Close Project

  foundry mcp [--allow READ,WRITE,MODIFY|ALL] [--project folder] [--print-config]
      Server MCP (stdio) untuk AI Host: Cursor, Claude, Codex, OpenCode, Windsurf, dll.

  foundry version
      Cetak versi Foundry

  foundry visual-verify [--addr 127.0.0.1:8080] [folder-project]
      Tambahkan visual verification commands ke workflow dan jalankan.
      Otomatis capture screenshots dan submit visual reviews untuk semua halaman.
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
	case "visual-verify":
		visualVerify(os.Args[2:])
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
	fs.String("ui-url", "", "tidak dipakai lagi; diterima untuk perintah lama")
	fs.Parse(args)
	dir := fs.Arg(0)
	bin, _ := os.Executable()
	srv := httpapi.New(web.FS(), bin)
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
	allow := fs.String("allow", "READ", "READ, WRITE, MODIFY atau ALL")
	dir := fs.String("project", "", "folder project (default: folder saat ini)")
	printCfg := fs.Bool("print-config", false, "cetak konfigurasi MCP client")
	fs.String("ui-url", "", "tidak dipakai lagi; diterima untuk config MCP lama")
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

func visualVerify(args []string) {
	fs := flag.NewFlagSet("visual-verify", flag.ExitOnError)
	dir := fs.String("project", "", "folder project (default: folder saat ini)")
	fs.Parse(args)
	
	if *dir == "" {
		*dir, _ = os.Getwd()
	}

	st, err := core.NewStore(*dir)
	if err != nil {
		log.Fatalf("buka project: %v", err)
	}

	if _, err := st.Init(""); err != nil {
		log.Fatalf("init project: %v", err)
	}

	// Check if visual verification commands already exist
	p, err := st.Load()
	if err != nil {
		log.Fatalf("load project: %v", err)
	}

	if p == nil || p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		log.Fatal("workflow belum ada; jalankan workflow generation terlebih dahulu")
	}

	// Check if visual commands already exist
	hasVisualCommands := false
	for _, cmd := range p.Workflow.Commands {
		if strings.HasPrefix(cmd.ID, "CMD-VISUAL-") {
			hasVisualCommands = true
			break
		}
	}

	if hasVisualCommands {
		fmt.Println("✓ Visual verification commands sudah ada di workflow")
	} else {
		// Add visual verification commands using Update
		_, err := st.Update(func(p *core.Project) error {
			count, err := work.AddVisualVerificationCommands(p, nil)
			if err != nil {
				return fmt.Errorf("gagal menambah visual commands: %w", err)
			}
			fmt.Printf("✓ Berhasil menambah %d visual verification commands\n", count)
			return nil
		})
		if err != nil {
			log.Fatalf("update project: %v", err)
		}
	}

	// Show visual status
	p, err = st.Load()
	if err != nil {
		log.Fatalf("load project: %v", err)
	}

	var verified, pending, failed int
	if p.Visual != nil {
		for _, audit := range p.Visual.Pages {
			switch audit.Status {
			case "pass":
				verified++
			case "fail":
				failed++
			default:
				pending++
			}
		}
	}

	totalPages := len(p.UI.Layout.Pages)
	
	fmt.Printf("\n📊 Visual Verification Status:\n")
	fmt.Printf("  Total pages: %d\n", totalPages)
	fmt.Printf("  Verified: %d\n", verified)
	fmt.Printf("  Pending: %d\n", pending)
	fmt.Printf("  Failed: %d\n", failed)

	if pending > 0 {
		fmt.Printf("\n⚠️  %d halaman belum diverifikasi\n", pending)
		fmt.Println("\n📋 Next steps:")
		fmt.Println("  1. Jalankan Foundry MCP server:")
		fmt.Println("     foundry mcp --allow ALL --project", *dir)
		fmt.Println("\n  2. Di AI agent (Claude/OpenCode/Cursor), jalankan workflow:")
		fmt.Println("     - Execute visual verification commands")
		fmt.Println("     - Capture screenshots untuk semua halaman")
		fmt.Println("     - Submit visual reviews")
		fmt.Println("\n  3. Atau gunakan UI Foundry:")
		fmt.Println("     foundry serve --project", *dir)
	} else if failed > 0 {
		fmt.Printf("\n❌ %d halaman gagal verifikasi\n", failed)
		fmt.Println("\n📋 Next steps:")
		fmt.Println("  1. Review visual findings di UI Foundry")
		fmt.Println("  2. Generate visual fix commands:")
		fmt.Println("     foundry visual-fixes --project", *dir)
		fmt.Println("  3. Execute fix commands di AI agent")
		fmt.Println("  4. Re-submit visual reviews")
	} else {
		fmt.Println("\n✅ Semua halaman sudah diverifikasi!")
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
