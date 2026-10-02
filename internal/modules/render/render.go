// Package render takes screenshots of designed pages with headless Chrome.
// The page is the UI's own /render route, so the image and the outline come
// from the same Wireframe component the user looks at.
package render

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Viewports per device, as the design page shows them.
var Viewports = map[string][2]int{"desktop": {1280, 900}, "tablet": {820, 1180}, "mobile": {390, 844}}

type Chrome struct {
	Binary string
	UIURL  string // base URL of the Foundry UI, e.g. http://127.0.0.1:8080
}

// Find returns a renderer, or nil when no Chrome/Chromium is installed.
// FOUNDRY_CHROME overrides the binary.
func Find(uiURL string) *Chrome {
	if uiURL == "" {
		return nil
	}
	names := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "chrome", "msedge"}
	if b := os.Getenv("FOUNDRY_CHROME"); b != "" {
		names = []string{b}
	} else {
		names = append(names, installPaths()...)
	}
	for _, n := range names {
		if path, err := exec.LookPath(n); err == nil {
			return &Chrome{Binary: path, UIURL: strings.TrimRight(uiURL, "/")}
		}
	}
	return nil
}

// installPaths lists default install locations of Chromium-based browsers
// that are usually not on PATH (Windows, macOS).
func installPaths() []string {
	switch runtime.GOOS {
	case "windows":
		var out []string
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if base := os.Getenv(env); base != "" {
				out = append(out,
					filepath.Join(base, `Google\Chrome\Application\chrome.exe`),
					filepath.Join(base, `Chromium\Application\chrome.exe`),
					filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`))
			}
		}
		return out
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	}
	return nil
}

var outlineRe = regexp.MustCompile(`(?s)<pre id="outline" data-dir="([^"]*)"[^>]*>(.*?)</pre>`)

// Shot renders one page at the device viewport into out (PNG) and returns
// the text outline of what was drawn. projectDir guards against the UI
// having another project open.
func (c *Chrome) Shot(ctx context.Context, projectDir, pageID, device, out string) (string, error) {
	vp, ok := Viewports[device]
	if !ok {
		return "", fmt.Errorf("perangkat %q tidak dikenal", device)
	}
	u := fmt.Sprintf("%s/render?page=%s&device=%s", c.UIURL, url.QueryEscape(pageID), device)
	dom, err := c.run(ctx, vp, "--dump-dom", u)
	if err != nil {
		return "", err
	}
	m := outlineRe.FindStringSubmatch(dom)
	if m == nil {
		return "", errors.New("halaman render tidak siap (UI Foundry tidak terjangkau atau halaman tidak ada)")
	}
	if filepath.Clean(html.UnescapeString(m[1])) != filepath.Clean(projectDir) {
		return "", fmt.Errorf("UI Foundry di %s sedang membuka project lain", c.UIURL)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	if _, err := c.run(ctx, vp, "--screenshot="+out, u); err != nil {
		return "", err
	}
	return strings.TrimSpace(html.UnescapeString(m[2])), nil
}

func (c *Chrome) run(ctx context.Context, vp [2]int, args ...string) (string, error) {
	profile, err := os.MkdirTemp("", "foundry-chrome-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(profile)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	argv := append([]string{"--headless=new", "--disable-gpu", "--hide-scrollbars", "--no-first-run",
		"--user-data-dir=" + profile, "--virtual-time-budget=8000", fmt.Sprintf("--window-size=%d,%d", vp[0], vp[1])}, args...)
	if os.Geteuid() == 0 {
		argv = append([]string{"--no-sandbox"}, argv...)
	}
	cmd := exec.CommandContext(ctx, c.Binary, argv...)
	cmd.WaitDelay = 5 * time.Second // chrome helpers may outlive the browser and hold the pipes
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil && !errors.Is(err, exec.ErrWaitDelay) {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 300 {
			msg = msg[len(msg)-300:]
		}
		return "", fmt.Errorf("chrome gagal: %v %s", err, msg)
	}
	return stdout.String(), nil
}

// Capture screenshots an external page (a design reference) at the device
// viewport into out (PNG). Only the image is kept: no DOM, HTML or CSS is
// read from the page.
func (c *Chrome) Capture(ctx context.Context, pageURL, device, out string) error {
	vp, ok := Viewports[device]
	if !ok {
		return fmt.Errorf("perangkat %q tidak dikenal", device)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	_, err := c.run(ctx, vp, "--screenshot="+out, pageURL)
	return err
}
