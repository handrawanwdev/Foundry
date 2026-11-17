package uikit

import (
	"fmt"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A page often opens dialogs: the form that creates or edits a record, the
// confirmation before a change. They are not part of the page's layout, so
// the mockup draws each after the page's content as the dialog it is: its
// fields, its buttons, and the confirmations it asks.

// fieldWords start a content line that names input fields.
var fieldWords = []string{"field ", "input ", "isian ", "form ", "formulir "}

func fieldLine(c string) bool {
	l := strings.ToLower(strings.TrimSpace(c))
	for _, w := range fieldWords {
		if strings.HasPrefix(l, w) {
			return true
		}
	}
	return false
}

// pageDialogs draws the dialogs of the populated page.
func (r *render) pageDialogs() {
	if r.state != "" || len(r.dialogs) == 0 {
		return
	}
	r.w(`<div class="fk-dialogs">`)
	for _, s := range r.dialogs {
		r.dialog(s)
	}
	r.w(`</div>`)
}

func (r *render) dialog(s core.Section) {
	var fields, buttons, confirms, notes []string
	for _, c := range s.Content {
		c = strings.TrimSpace(c)
		switch b, isButton := buttonLabel(c); {
		case c == "":
		case isButton:
			buttons = append(buttons, splitList(b)...)
		case fieldLine(c):
			_, list, _ := strings.Cut(c, " ")
			for _, f := range splitList(strings.TrimPrefix(strings.TrimSpace(list), ":")) {
				fields = append(fields, capital(f))
			}
		case has(strings.ToLower(c), "konfirmasi", "confirm"):
			confirms = append(confirms, c)
		default:
			notes = append(notes, c)
		}
	}
	if len(fields) > 0 && len(buttons) == 0 {
		buttons = []string{"Simpan", "Batal"}
	}
	r.f(`<section class="fk-dialog" role="dialog" aria-label="%s" data-foundry-section="%s" data-foundry-dialog="true">`, esc(s.Name), esc(s.ID))
	r.f(`<header class="fk-dialog-head"><span class="fk-badge">Dialog</span><h2>%s</h2></header>`, esc(s.Name))
	if s.Purpose != "" {
		r.w(`<p class="fk-meta">` + esc(s.Purpose) + `</p>`)
	}
	for _, n := range notes {
		r.w(`<p class="fk-meta">` + esc(n) + `</p>`)
	}
	if len(fields) > 0 {
		r.formWith(s, fields, "")
	}
	r.dialogButtons(buttons, false)
	for _, c := range confirms {
		text := strings.TrimSpace(c)
		if l := strings.ToLower(text); strings.HasPrefix(l, "dialog ") || strings.HasPrefix(l, "modal ") {
			_, text, _ = strings.Cut(text, " ")
		}
		r.f(`<div class="fk-confirm-box" role="alertdialog" aria-label="%s"><p>%s</p>`, esc(capital(text)), esc(capital(text)))
		// The button does what is confirmed: "konfirmasi hapus …" deletes.
		label, action := "Konfirmasi", text
		if f := strings.Fields(text); len(f) > 1 && has(strings.ToLower(f[0]), "konfirmasi", "confirm") {
			label, action = capital(f[1]), strings.Join(f[1:], " ")
		}
		r.dialogButtons([]string{"Batal", label}, core.DangerAction(action))
		r.w(`</div>`)
	}
	r.w(`</section>`)
}

// dialogButtons draws a dialog's buttons: the dismissing ones first and
// plain, then the first other one primary (destructive when danger says
// the dialog's action cannot be undone).
func (r *render) dialogButtons(labels []string, danger bool) {
	if len(labels) == 0 {
		return
	}
	var dismiss, act []string
	for _, l := range labels {
		if has(strings.ToLower(l), "batal", "tutup", "kembali", "cancel", "close") {
			dismiss = append(dismiss, l)
		} else {
			act = append(act, l)
		}
	}
	r.w(`<div class="fk-actions">`)
	for _, l := range dismiss {
		r.f(`<button class="fk-btn fk-btn-ghost" type="button">%s</button>`, esc(capital(l)))
	}
	for i, l := range act {
		cls := "fk-btn"
		if i == 0 {
			cls = "fk-btn fk-btn-primary"
			if danger || core.DangerAction(l) {
				cls = "fk-btn fk-btn-danger"
			}
		}
		r.f(`<button class="%s" type="button">%s</button>`, cls, esc(capital(l)))
	}
	r.w(`</div>`)
}

// formContentIssue reports a collection whose content names form fields
// and buttons only: a form or dialog drawn as a table or list.
func formContentIssue(pg *core.LayoutPage, s core.Section, kind string) string {
	fields := 0
	for _, c := range s.Content {
		switch _, isButton := buttonLabel(strings.TrimSpace(c)); {
		case strings.TrimSpace(c) == "", isButton, core.ControlEntry(c) != "":
		case fieldLine(c):
			fields++
		default:
			return ""
		}
	}
	if fields == 0 {
		return ""
	}
	what := map[string]string{"table": "tabel", "list": "daftar", "cards": "kartu"}[kind]
	return fmt.Sprintf("halaman %s, bagian %s: tergambar sebagai %s padahal isinya field form; jadikan form, atau dialog bila dibuka dari halaman ini (nama/id memuat \"dialog\")", pg.ID, s.Name, what)
}
