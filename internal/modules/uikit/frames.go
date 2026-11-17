package uikit

import (
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
)

// A page in the mockup workspace is a set of frames, as in a design tool:
// the page as it opens, each dialog and confirmation its controls open,
// the form sent incomplete, the action that fails and the one that
// succeeds, and the states of the layout spec. A frame is a query of the
// page's mockup: the page the prototype walks through, held at that
// moment. What is said in it (the success message, the failure and the
// field it marks) is the layout spec's.

// Frame is one moment of a page.
type Frame struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Name  string `json:"name"`
	Kind  string `json:"kind"` // page | form | invalid | error | success | confirm | state
	Query string `json:"query,omitempty"`
}

// formRec is a form the page drew: in the dialog view ("" on the page),
// sent with submit, with required fields.
type formRec struct {
	view, submit string
	required     int
}

// ask is a confirmation a control of the page opens: trigger is the
// control, act the action confirmed.
type ask struct{ id, trigger, act string }

// editVerbs start an action that changes a record: its form shows the
// record.
var editVerbs = []string{"ubah", "edit", "sunting", "perbarui", "update"}

func editAction(label string) bool {
	f := strings.Fields(strings.ToLower(label))
	return len(f) > 0 && slices.Contains(editVerbs, f[0])
}

// Frames are the moments of page id.
func (k *Kit) Frames(d Doc, id string) []Frame {
	d.Link, d.Sim, d.Self = nil, Sim{}, ""
	pg, err := d.page(id)
	if err != nil {
		return nil
	}
	r := &render{kit: k, doc: d, page: pg}
	r.html()
	out := []Frame{{ID: "utama", Group: "Halaman", Name: "Utama", Kind: "page"}}
	seen := map[string]bool{}
	add := func(group, name, kind string, sim Sim) {
		fid := ident(group + "-" + name)
		if seen[fid] {
			return
		}
		seen[fid] = true
		q := ""
		if u, err := url.Parse(WithSim("x", sim)); err == nil {
			q = u.RawQuery
		}
		out = append(out, Frame{ID: fid, Group: group, Name: name, Kind: kind, Query: q})
	}
	outcomes := func(group, view, act string, fill bool) {
		add(group, "Gagal", "error", Sim{View: view, Feedback: "error", Act: act, Fill: fill})
		add(group, "Berhasil", "success", Sim{Feedback: "ok", Act: act})
	}
	if f, ok := r.pageForm(); ok {
		g := capital(f.submit)
		if f.required > 0 {
			add(g, "Validasi", "invalid", Sim{Feedback: "invalid"})
		}
		outcomes(g, "", f.submit, false)
	}
	for _, dl := range r.dialogs {
		if !dl.opened || len(dl.fields) == 0 {
			continue
		}
		f := r.formIn(dl.id)
		for _, t := range dl.triggers {
			g, fill := capital(actionName(t)), editAction(t)
			add(g, "Form", "form", Sim{View: dl.id, Fill: fill})
			if !fill && f.required > 0 {
				add(g, "Validasi", "invalid", Sim{View: dl.id, Feedback: "invalid"})
			}
			outcomes(g, dl.id, f.submit, true)
		}
	}
	for _, a := range r.asks {
		g := capital(actionName(a.trigger))
		add(g, "Konfirmasi", "confirm", Sim{View: a.id})
		outcomes(g, a.id, a.act, false)
	}
	for _, st := range core.UIStates {
		if strings.TrimSpace(pg.States[st]) != "" && st != "interactive" {
			out = append(out, Frame{ID: "state-" + ident(st), Group: "State", Name: st, Kind: "state", Query: "state=" + url.QueryEscape(st)})
		}
	}
	return out
}

// pageForm is the page's own form, however many sections it spans.
func (r *render) pageForm() (formRec, bool) {
	var out formRec
	found := false
	for _, f := range r.forms {
		if f.view == "" {
			out.submit, out.required, found = f.submit, out.required+f.required, true
		}
	}
	return out, found
}

func (r *render) formIn(view string) formRec {
	for _, f := range r.forms {
		if f.view == view {
			return f
		}
	}
	return formRec{view: view, submit: "Simpan"}
}

// asked records a confirmation a control opens.
func (r *render) asked(id, trigger, act string) {
	if !slices.ContainsFunc(r.asks, func(a ask) bool { return a.id == id || strings.EqualFold(a.trigger, trigger) }) {
		r.asks = append(r.asks, ask{id: id, trigger: trigger, act: act})
	}
}

// popAttrs are the attributes of popover id. The view the URL opens is
// drawn open on its own backdrop instead (open); the caller closes the
// backdrop after the element.
func (r *render) popAttrs(id string) (attrs string, open bool) {
	switch {
	case id == "":
		return "", false
	case id == r.doc.Sim.View:
		r.w(`<div class="fk-overlay" data-foundry-view="open">`)
		return ` id="` + id + `"`, true
	}
	return ` id="` + id + `" popover`, false
}

// closeX is the close button in a dialog's header: it leads back to the
// page in the open view, closes popover pop, else only shows.
func (r *render) closeX(pop string, open bool) {
	switch {
	case open:
		r.f(`<a class="fk-modal-x" href="%s" aria-label="Tutup">%s</a>`, esc(r.doc.hrefSim(r.page.ID, r.doc.Sim.Nav())), icon("close"))
	case pop != "":
		r.f(`<button class="fk-modal-x" type="button" popovertarget="%s" popovertargetaction="hide" aria-label="Tutup">%s</button>`, pop, icon("close"))
	default:
		r.f(`<button class="fk-modal-x" type="button" aria-label="Tutup">%s</button>`, icon("close"))
	}
}

// dismiss is a control that closes popover pop; in the open view it leads
// back to the page.
func (r *render) dismiss(label, pop string, open bool) {
	switch {
	case open:
		r.f(`<a class="fk-btn" href="%s">%s</a>`, esc(r.doc.hrefSim(r.page.ID, r.doc.Sim.Nav())), esc(capital(label)))
	case pop != "":
		r.f(`<button class="fk-btn" type="button" popovertarget="%s" popovertargetaction="hide">%s</button>`, pop, esc(capital(label)))
	default:
		r.f(`<button class="fk-btn" type="button">%s</button>`, esc(capital(label)))
	}
}

// quotedText is a message the layout spec quotes ('Kategori tersimpan').
var quotedText = regexp.MustCompile(`['"‘“]([^'"’”]{3,80})['"’”]`)

// successText is what the page says when act succeeds: the message of
// the success state that names the action, else its first, else the
// state itself when short.
func (r *render) successText(act string) string {
	st := strings.TrimSpace(r.page.States["success"])
	var quotes []string
	for _, m := range quotedText.FindAllStringSubmatch(st, -1) {
		quotes = append(quotes, strings.TrimSpace(m[1]))
	}
	for _, q := range quotes {
		if act != "" && core.SharesStem(act, q) {
			return q
		}
	}
	switch {
	case len(quotes) > 0:
		return quotes[0]
	case st != "" && len([]rune(st)) <= 80:
		return st
	}
	return defaultStates["success"]
}

// success is the feedback of a succeeded action: a toast when the success
// state says so, else a dialog.
func (r *render) success() {
	if msg := accountOK(r.doc.Sim.Act); msg != "" {
		r.toast("ok", "ok", msg)
		return
	}
	msg := r.successText(r.doc.Sim.Act)
	if has(strings.ToLower(r.page.States["success"]), "toast", "snackbar") {
		r.toast("ok", "ok", msg)
		return
	}
	// The page that says nothing of success names the action instead.
	if msg == defaultStates["success"] && r.doc.Sim.Act != "" {
		r.result(true, capital(strings.ToLower(actionName(r.doc.Sim.Act)))+" berhasil", "", "", "")
		return
	}
	r.result(true, "Berhasil", msg, "", "")
}

// toast is a short message over the page: kind ok or reject. One the
// prototype answers with (sim set) floats and closes back to the page.
func (r *render) toast(kind, sim, msg string) {
	ic := "check"
	if kind != "ok" {
		ic = "alert-circle"
	}
	attr := ""
	if sim != "" {
		attr = ` data-foundry-sim="` + sim + `"`
	}
	r.f(`<div class="fk-toast fk-toast-%s" role="status"%s>%s<span class="fk-toast-text">%s</span>`, kind, attr, icon(ic), esc(msg))
	if sim != "" {
		r.f(`<a class="fk-toast-x" href="%s" aria-label="Tutup">%s</a>`, esc(r.doc.hrefSim(r.page.ID, r.doc.Sim.Nav())), icon("close"))
	}
	r.w(`</div>`)
}

// result is the dialog that answers an action on its own.
func (r *render) result(ok bool, title, body, code, spec string) {
	ic, kind := "check", "ok"
	if !ok {
		ic, kind = "alert-circle", "error"
	}
	r.f(`<div class="fk-overlay" data-foundry-sim="%s"><div class="fk-result fk-modal fk-result-%s" role="alertdialog" aria-label="%s">`, kind, kind, esc(title))
	r.f(`<span class="fk-result-icon">%s</span><h2>%s</h2>`, icon(ic), esc(title))
	if body != "" {
		r.f(`<p>%s</p>`, esc(body))
	}
	r.code(code, spec)
	r.f(`<div class="fk-actions"><a class="fk-btn fk-btn-primary" href="%s">Tutup</a></div></div></div>`, esc(r.doc.hrefSim(r.page.ID, r.doc.Sim.Nav())))
}

// alert is what a form, dialog or confirmation says about how it was
// sent: kind error or invalid.
func (r *render) alert(kind, title, body, code, spec string) {
	r.f(`<div class="fk-alert fk-alert-feedback" role="alert" data-foundry-sim="%s">%s<div class="fk-alert-text"><b>%s</b>`, kind, icon("alert-circle"), esc(title))
	if body != "" {
		r.f(`<span>%s</span>`, esc(body))
	}
	r.code(code, spec)
	r.w(`</div></div>`)
}

// code is the error code a failure answers with, for whoever builds the
// page; it holds the layout spec's words.
func (r *render) code(code, spec string) {
	if code != "" {
		r.f(`<code class="fk-code" title="%s">%s</code>`, esc("Spesifikasi: "+spec), esc(code))
	}
}

// failure is a failed action as the user reads it. The error state of the
// layout spec says what the API answers ("409 DUPLICATE_NAME pada field
// Nama"): a part naming a status or an error code is a note for whoever
// builds the page, so the user reads the plain words of that status and
// the code stands beside them; the words it quotes, or a part written in
// plain words, are what the user reads.
type failure struct {
	title, body, field, fieldMsg, code, spec string
}

var (
	httpStatus = regexp.MustCompile(`\b[1-5][0-9]{2}\b`)
	errorCode  = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)
)

func (r *render) failure(act string, fields []string) failure {
	spec, field := r.failText(act, fields)
	f := failure{title: failTitle(act), field: field, spec: spec}
	status := httpStatus.FindString(spec)
	var codes []string
	for _, c := range []string{status, errorCode.FindString(spec)} {
		if c != "" {
			codes = append(codes, c)
		}
	}
	f.code = strings.Join(codes, " · ")
	quote := ""
	if m := quotedText.FindStringSubmatch(spec); m != nil {
		quote = capital(strings.TrimSpace(m[1]))
	}
	switch {
	case quote != "":
		f.body = quote
	case spec != "" && f.code == "":
		f.body = spec
	default:
		f.body = statusText(status, r.page.Pattern == "auth")
	}
	if field != "" {
		f.fieldMsg = f.body
		if quote == "" && f.code != "" {
			f.fieldMsg = fieldStatusText(status, field)
		}
		f.body = "Periksa field yang ditandai lalu coba lagi."
	}
	return f
}

// failTitle heads the failure of act: "Simpan gagal".
func failTitle(act string) string {
	if a := strings.ToLower(actionName(act)); a != "" {
		return capital(a) + " gagal"
	}
	return "Aksi gagal"
}

// statusText is what the user reads when the API answers status; auth
// when the page signs in.
func statusText(status string, auth bool) string {
	switch status {
	case "401":
		if auth {
			return "Akun atau kata sandi tidak cocok."
		}
		return "Sesi Anda berakhir. Masuk lagi untuk melanjutkan."
	case "403":
		return "Anda tidak punya akses untuk aksi ini."
	case "404":
		return "Data tidak ditemukan. Mungkin sudah dihapus."
	case "409":
		return "Tidak dapat diproses: data sudah ada atau masih dipakai data lain."
	case "400", "422":
		return "Data belum valid. Periksa lalu coba lagi."
	case "429":
		return "Terlalu banyak percobaan. Tunggu sebentar lalu coba lagi."
	}
	return "Terjadi kesalahan. Coba lagi beberapa saat lagi."
}

// fieldStatusText is what a field says when the API rejects it.
func fieldStatusText(status, field string) string {
	if status == "409" {
		return field + " sudah dipakai."
	}
	return field + " tidak valid."
}

// failText is the failure of act as the error state describes it: the
// part that names one of fields (returned as field), else the part that
// names the action. A page that is its form takes the first part; else
// nothing.
func (r *render) failText(act string, fields []string) (msg, field string) {
	var parts []string
	for _, p := range strings.Split(r.page.States["error"], ";") {
		if p = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), ".")); p != "" {
			parts = append(parts, p)
		}
	}
	for _, p := range parts {
		for _, f := range fields {
			if core.SharesStem(f, p) {
				return capital(p), f
			}
		}
	}
	for _, p := range parts {
		if act != "" && core.SharesStem(act, p) {
			return capital(p), ""
		}
	}
	if len(parts) > 0 && (formLike(r.page.Pattern) || r.page.Pattern == "auth") {
		return capital(parts[0]), ""
	}
	return "", ""
}

// fieldSpec reads a written field: its label, a hint, and whether it may
// be left empty. "Kategori wajib dipilih" is Kategori, required;
// "Catatan (opsional)" is Catatan, optional; "Nama (unik)" hints unik;
// "Stok awal dengan helper 'dicatat …'" hints the quote.
func fieldSpec(it string) (label, hint string, optional bool) {
	it = strings.TrimSpace(it)
	l := strings.ToLower(it)
	optional = has(l, "opsional", "optional")
	if m := quotedText.FindStringSubmatch(it); m != nil {
		hint = strings.TrimSpace(m[1])
	}
	if a := strings.Index(it, "("); hint == "" && a > 0 {
		if b := strings.Index(it[a:], ")"); b > 1 {
			if h := strings.TrimSpace(it[a+1 : a+b]); !has(strings.ToLower(h), "opsional", "optional", "wajib") {
				hint = h
			}
		}
	}
	cut := len(it)
	for _, m := range []string{" wajib", " opsional", " optional", " (", " dengan ", " default ", " sebagai ", " bila ", " jika ", " terkunci"} {
		if i := strings.Index(l, m); i > 0 && i < cut {
			cut = i
		}
	}
	// "Kata sandi bila akun baru" asks for it only then.
	for _, m := range []string{" bila ", " jika "} {
		if i := strings.Index(l, m); i > 0 && i == cut && hint == "" {
			hint = "Hanya " + strings.TrimSpace(it[i:]) + "."
		}
	}
	if label = capital(strings.TrimSpace(it[:cut])); label == "" {
		label = capital(it)
	}
	return label, hint, optional
}
