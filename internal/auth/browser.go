package auth

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

const openerGraceWindow = 2 * time.Second

func openBrowser(ctx context.Context, rawURL string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		name = "xdg-open"
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, name, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return waitForOpener(cmd, openerGraceWindow)
}

// waitForOpener reports an opener that exits non-zero inside the grace window
// and assumes a hand-off otherwise: xdg-open on some setups execs the browser
// in the foreground and only returns when the browser does, so waiting for the
// process to finish would block the login harder than the hang this bounds.
func waitForOpener(cmd *exec.Cmd, grace time.Duration) error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timer := time.NewTimer(grace)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-timer.C:
		return nil
	}
}

func writeBrowserMessage(w http.ResponseWriter, title, body string, ok bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	status := http.StatusOK
	accent, glyph := "#61BD73", "✓"
	if !ok {
		status = http.StatusBadRequest
		accent, glyph = "#E5484D", "✕"
	}
	w.WriteHeader(status)
	fmt.Fprintf(
		w,
		`<!doctype html><html lang="en"><head><meta charset="utf-8">`+
			`<meta name="viewport" content="width=device-width,initial-scale=1"><title>%s</title>`+
			`<style>:root{color-scheme:dark}html,body{height:100%%;margin:0}`+
			`body{display:flex;align-items:center;justify-content:center;background:#141414;color:#fff;`+
			`font-family:Inter,system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}`+
			`main{max-width:420px;padding:0 24px;text-align:center}`+
			`.badge{width:48px;height:48px;border-radius:50%%;margin:0 auto 24px;display:flex;`+
			`align-items:center;justify-content:center;background:%s;color:#fff;font-size:24px}`+
			`h1{font-size:20px;font-weight:600;margin:0 0 8px}`+
			`p{margin:0;color:rgba(255,255,255,.6);font-size:14px;line-height:1.5}`+
			`footer{margin-top:32px;color:rgba(255,255,255,.35);font-size:12px;letter-spacing:.02em}</style></head>`+
			`<body><main><div class="badge">%s</div><h1>%s</h1><p>%s</p>`+
			`<footer>Weaviate Cloud CLI</footer></main></body></html>`,
		html.EscapeString(title),
		accent,
		glyph,
		html.EscapeString(title),
		html.EscapeString(body),
	)
}
