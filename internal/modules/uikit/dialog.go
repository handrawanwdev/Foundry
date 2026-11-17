package uikit

import (
	"fmt"
	"slices"
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

// dialog is a dialog section as the mockup draws it. A control of the page
// that names it opens it (opened); the confirmations it asks open on
// their own, from the action they confirm.
type dialog struct {
	s                      core.Section
	id                     string
	fields, buttons, notes []string
	confirms               []*confirm
	opened                 bool
	// triggers are the controls that open it; pop is its popover id while
	// drawn, open when it is the view drawn open.
	triggers []string
	pop      string
	open     bool
}

type confirm struct {
	text, id string
	opened   bool
}

func parseDialog(s core.Section) *dialog {
	d := &dialog{s: s, id: "dlg-" + ident(s.ID)}
	for _, c := range s.Content {
		c = strings.TrimSpace(c)
		switch b, isButton := buttonLabel(c); {
		case c == "":
		case isButton:
			d.buttons = append(d.buttons, splitList(b)...)
		case fieldLine(c):
			_, list, _ := strings.Cut(c, " ")
			for _, f := range splitList(strings.TrimPrefix(strings.TrimSpace(list), ":")) {
				d.fields = append(d.fields, capital(f))
			}
		case has(strings.ToLower(c), "konfirmasi", "confirm"):
			d.confirms = append(d.confirms, &confirm{text: c, id: fmt.Sprintf("%s-c%d", d.id, len(d.confirms)+1)})
		default:
			d.notes = append(d.notes, c)
		}
	}
	if len(d.fields) > 0 && len(d.buttons) == 0 {
		d.buttons = []string{"Simpan", "Batal"}
	}
	return d
}

// formVerbs start an action that fills a form: it opens the page's form
// dialog when no dialog is named by it.
var formVerbs = append([]string{"tambah", "buat", "create", "add", "new"}, editVerbs...)

// openedBy marks d opened by the control label.
func (d *dialog) openedBy(label string) string {
	d.opened = true
	if !slices.ContainsFunc(d.triggers, func(t string) bool { return strings.EqualFold(t, label) }) {
		d.triggers = append(d.triggers, label)
	}
	return d.id
}

// dialogFor is the popover a control labelled label opens: the
// confirmation it names when the action cannot be undone, else the dialog
// it names, else the page's one form dialog when the action is a bare
// verb that fills a form.
// "" when it opens none.
func (r *render) dialogFor(label string) string {
	l := strings.TrimSpace(label)
	if l == "" || len(r.dialogs) == 0 {
		return ""
	}
	if core.DangerAction(l) {
		for _, d := range r.dialogs {
			for _, c := range d.confirms {
				if core.SharesStem(l, c.text) {
					c.opened = true
					act, _ := confirmLabel(c.text)
					r.asked(c.id, l, act)
					return c.id
				}
			}
		}
	}
	var forms []*dialog
	for _, d := range r.dialogs {
		if core.SharesStem(l, d.s.Name+" "+d.s.ID) {
			return d.openedBy(l)
		}
		if len(d.fields) > 0 {
			forms = append(forms, d)
		}
	}
	// A bare verb ("Ubah") fills the page's form; "Buat pesanan" names
	// what it makes, which no dialog above did.
	if f := strings.Fields(strings.ToLower(l)); len(forms) == 1 && len(f) == 1 && (slices.Contains(formVerbs, f[0]) || core.CreateStep(l)) {
		return forms[0].openedBy(l)
	}
	return ""
}

// pageDialogs draws the dialogs of the populated page: those a control
// opens as popovers, the rest after the page's content, where the quality
// gate points at them.
func (r *render) pageDialogs() {
	if r.state != "" || len(r.dialogs) == 0 {
		return
	}
	var static []*dialog
	for _, d := range r.dialogs {
		if d.opened {
			r.dialog(d, true)
		} else {
			static = append(static, d)
			if len(d.fields) > 0 || !slices.ContainsFunc(d.confirms, func(c *confirm) bool { return c.opened }) {
				r.contentIssues = append(r.contentIssues, fmt.Sprintf("halaman %s, bagian %s: tidak ada kontrol di halaman ini yang membuka dialognya; sebut pemicunya, mis. \"Aksi baris: Ubah\" atau \"Tombol Tambah ...\"", r.page.ID, d.s.Name))
			}
		}
		for _, c := range d.confirms {
			if c.opened {
				r.confirmBoxOf(c.text, c.id)
			}
		}
	}
	if len(static) == 0 {
		return
	}
	r.w(`<div class="fk-dialogs">`)
	for _, d := range static {
		r.dialog(d, false)
	}
	r.w(`</div>`)
}

// dialog draws a dialog: as a popover when a control opens it, else in
// place. Its form holds its buttons, so the browser checks the required
// fields before it is sent.
func (r *render) dialog(d *dialog, popover bool) {
	attrs := ""
	d.pop, d.open = "", false
	if popover {
		d.pop = d.id
		attrs, d.open = r.popAttrs(d.id)
	}
	title := dialogTitle(d.s.Name)
	r.f(`<section class="fk-dialog fk-modal"%s role="dialog" aria-label="%s" data-foundry-section="%s" data-foundry-dialog="true">`, attrs, esc(title), esc(d.s.ID))
	r.w(`<header class="fk-modal-head"><div class="fk-modal-titles">`)
	if !popover {
		r.w(`<span class="fk-badge">Dialog</span>`)
	}
	r.f(`<h2>%s</h2>`, esc(title))
	if d.s.Purpose != "" {
		r.w(`<p class="fk-modal-desc">` + esc(d.s.Purpose) + `</p>`)
	}
	r.w(`</div>`)
	r.closeX(d.pop, d.open)
	r.w(`</header>`)
	for _, n := range d.notes {
		r.w(`<p class="fk-meta">` + esc(n) + `</p>`)
	}
	if len(d.fields) > 0 {
		r.formWith(d.s, r.lineFields(d.fields, d.s), d)
	} else {
		r.dialogButtons(d.buttons, false, d.pop, d.open, false)
	}
	for _, c := range d.confirms {
		if !c.opened {
			r.confirmBoxOf(c.text, "")
		}
	}
	r.w(`</section>`)
	if d.open {
		r.w(`</div>`)
	}
}

// dialogTitle is the heading of dialog name: its subject without the
// word that says it is a dialog ("Dialog kategori" is "Kategori").
func dialogTitle(name string) string {
	if f := strings.Fields(name); len(f) > 1 && has(strings.ToLower(f[0]), "dialog", "modal") {
		return capital(strings.Join(f[1:], " "))
	}
	return capital(strings.TrimSpace(name))
}

// confirmTitle asks whether to do action ("Hapus pengguna?"); an action
// without an object asks about the data at hand.
func confirmTitle(action string) string {
	a := strings.TrimSpace(actionName(action))
	if a == "" {
		return "Lanjutkan?"
	}
	if len(strings.Fields(a)) == 1 {
		a += " data ini"
	}
	return capital(strings.ToLower(a)) + "?"
}

// confirmHead opens confirmation box attrs with an icon toned by whether
// action is destructive, and its question.
func (r *render) confirmHead(attrs, action, extra string) {
	cls, ic := "fk-confirm-box fk-modal", "alert-circle"
	if core.DangerAction(action) {
		cls, ic = cls+" fk-confirm-danger", "alert"
	}
	title := confirmTitle(action)
	r.f(`<div class="%s"%s role="alertdialog" aria-label="%s"%s>`, cls, attrs, esc(title), extra)
	r.f(`<span class="fk-confirm-icon">%s</span><div class="fk-confirm-text"><p class="fk-confirm-title">%s</p>`, icon(ic), esc(title))
}

// confirmLabel is the button of a confirmation and the action it does:
// "Dialog konfirmasi hapus" deletes, with Hapus.
func confirmLabel(c string) (label, action string) {
	text := strings.TrimSpace(c)
	if l := strings.ToLower(text); strings.HasPrefix(l, "dialog ") || strings.HasPrefix(l, "modal ") {
		_, text, _ = strings.Cut(text, " ")
	}
	label, action = "Konfirmasi", text
	if f := strings.Fields(text); len(f) > 1 && has(strings.ToLower(f[0]), "konfirmasi", "confirm") {
		label, action = capital(f[1]), strings.Join(f[1:], " ")
	}
	return label, action
}

// confirmBoxOf draws a confirmation a dialog asks: a popover of its own
// when pop is set.
func (r *render) confirmBoxOf(c, pop string) {
	attrs, open := r.popAttrs(pop)
	label, action := confirmLabel(c)
	r.confirmHead(attrs, action, "")
	r.w(`</div>`)
	r.failAlert(pop, label)
	r.dialogButtons([]string{"Batal", label}, core.DangerAction(action), pop, open, false)
	r.w(`</div>`)
	if open {
		r.w(`</div>`)
	}
}

// failAlert says, in the view drawn open after act failed, why it did.
func (r *render) failAlert(view, act string) {
	if view == "" || r.doc.Sim.View != view || r.doc.Sim.Feedback != "error" {
		return
	}
	f := r.failure(act, nil)
	r.alert("error", f.title, f.body, f.code, f.spec)
}

// dialogButtons draws a dialog's buttons: the dismissing ones first and
// plain, then the first other one primary (destructive when danger says
// the dialog's action cannot be undone). In the popover pop, dismissing
// closes it (leaves the open view) and acting simulates the action: the
// live mockup answers with its outcome, sending the form it ends when
// form is set; a static one closes the popover.
func (r *render) dialogButtons(labels []string, danger bool, pop string, open, form bool) {
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
		r.dismiss(l, pop, open)
	}
	for i, l := range act {
		cls := "fk-btn"
		if i == 0 {
			cls = "fk-btn fk-btn-primary"
			if danger || core.DangerAction(l) {
				cls = "fk-btn fk-btn-danger"
			}
		}
		switch {
		case pop != "" && i == 0 && r.doc.live() && form:
			r.f(`<button class="%s" type="submit">%s</button>`, cls, esc(capital(l)))
		case pop != "" && i == 0 && r.doc.live():
			sim := r.outcome(capital(l), pop)
			r.f(`<a class="%s" href="%s" data-foundry-sim="%s">%s</a>`, cls, esc(r.doc.hrefSim(r.page.ID, sim)), sim.Feedback, esc(capital(l)))
		case pop != "" && !open:
			r.f(`<button class="%s" type="button" popovertarget="%s" popovertargetaction="hide">%s</button>`, cls, pop, esc(capital(l)))
		default:
			r.f(`<button class="%s" type="button">%s</button>`, cls, esc(capital(l)))
		}
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
