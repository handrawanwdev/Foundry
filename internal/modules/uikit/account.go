package uikit

import "strings"

// Every signed-in app has an account menu in its top bar: who is signed in
// and as what, their profile, changing the password, and signing out. A
// design rarely draws it, so the mockup always does; a page of the design
// that is the profile or the password change takes its place.

const (
	accMenu       = "acc-menu"
	accProfile    = "acc-profile"
	accPassword   = "acc-password"
	actProfile    = "Simpan profil"
	actPassword   = "Simpan kata sandi"
	actSignOut    = "Keluar"
	okProfile     = "Profil diperbarui."
	okPassword    = "Kata sandi diperbarui. Gunakan kata sandi baru saat masuk berikutnya."
	okSignOut     = "Anda sudah keluar."
	failPassword  = "Kata sandi saat ini tidak cocok."
	labelCurrent  = "Kata sandi saat ini"
	labelNew      = "Kata sandi baru"
	labelRepeat   = "Ulangi kata sandi baru"
	labelName     = "Nama"
	labelEmail    = "Email"
	labelRoleName = "Peran"
)

// accountOK is what the prototype says after an account action; "" for
// another action.
func accountOK(act string) string {
	return map[string]string{actProfile: okProfile, actPassword: okPassword, actSignOut: okSignOut}[act]
}

// signedIn reports whether the app has signed-in users: a sign-in page, or
// a page only they open.
func (r *render) signedIn() bool {
	for _, pg := range r.doc.Layout.Pages {
		if pg.Pattern == "auth" || pg.Access == "auth" {
			return true
		}
	}
	return false
}

// userRole is the role the page is seen as: the simulated one, else the
// first the page names.
func (r *render) userRole() string {
	if role := strings.TrimSpace(r.doc.Sim.Role); role != "" {
		return capital(role)
	}
	first := func(roles string) string {
		f := strings.FieldsFunc(roles, func(c rune) bool { return c == ',' || c == '/' })
		if len(f) == 0 || has(strings.ToLower(f[0]), "semua", "all ", "siapa") {
			return ""
		}
		return capital(strings.TrimSpace(f[0]))
	}
	if role := first(r.page.Role); role != "" {
		return role
	}
	// The role of the page's feature's use cases.
	if p := r.doc.Project; p != nil && r.page.FeatureID != "" {
		for _, fl := range p.Flows {
			if role := first(fl.Role); fl.FeatureID == r.page.FeatureID && role != "" {
				return role
			}
		}
	}
	return "Pengguna"
}

// accountPage is the design's own page for the signed-in user's profile
// (password false) or password change (true).
func (r *render) accountPage(password bool) string {
	for _, pg := range r.doc.Layout.Pages {
		if pg.Pattern == "auth" {
			continue
		}
		id, name := strings.ToLower(pg.ID), strings.ToLower(pg.Name)
		switch {
		case password && has(name+" "+id, "kata sandi", "password", "kata-sandi"):
			return pg.ID
		case !password && (has(name, "profil saya", "profil pengguna", "akun saya", "my profile", "my account", "user profile") ||
			id == "profil" || id == "profile" || id == "akun" || id == "account"):
			return pg.ID
		}
	}
	return ""
}

// signInPage is where signing out leads: the sign-in page.
func (r *render) signInPage() string {
	first := ""
	for _, pg := range r.doc.Layout.Pages {
		if pg.Pattern != "auth" {
			continue
		}
		if has(strings.ToLower(pg.ID+" "+pg.Name), "masuk", "login", "sign in", "signin") {
			return pg.ID
		}
		if first == "" {
			first = pg.ID
		}
	}
	return first
}

// userMenu is the top bar's user button and the account menu it opens.
func (r *render) userMenu() {
	role := r.userRole()
	name, email := sample("nama", 0), sample("email", 0)
	if !r.signedIn() {
		r.f(`<div class="fk-user"><span class="fk-avatar">%s</span><span class="fk-user-text"><b>%s</b></span></div>`, esc(initials(role)), esc(role))
		return
	}
	r.account = true
	r.f(`<button class="fk-user" type="button" aria-haspopup="menu" popovertarget="%s" style="anchor-name:--%s"><span class="fk-avatar">%s</span><span class="fk-user-text"><b>%s</b><small>%s</small></span>%s</button>`,
		accMenu, accMenu, esc(initials(name)), esc(name), esc(role), icon("chevron"))
	r.f(`<div id="%s" popover class="fk-menu fk-account" role="menu" style="position-anchor:--%s">`, accMenu, accMenu)
	r.f(`<div class="fk-account-head"><span class="fk-avatar">%s</span><span><b>%s</b><small>%s</small></span><span class="fk-badge">%s</span></div>`,
		esc(initials(name)), esc(name), esc(email), esc(role))
	r.w(`<hr class="fk-menu-sep" role="separator">`)
	r.accountItem("Profil saya", "user", accProfile, r.accountPage(false))
	r.accountItem("Ganti kata sandi", "key", accPassword, r.accountPage(true))
	r.w(`<hr class="fk-menu-sep" role="separator">`)
	if pg := r.signInPage(); pg != "" {
		r.f(`<a class="fk-menu-item" role="menuitem" href="%s" data-foundry-link="%s">%sKeluar</a>`, esc(r.doc.href(pg)), esc(pg), icon("logout"))
	} else {
		sim := r.doc.Sim.Nav()
		sim.Act, sim.Feedback = actSignOut, "ok"
		r.f(`<a class="fk-menu-item" role="menuitem" href="%s">%sKeluar</a>`, esc(r.doc.hrefSim(r.page.ID, sim)), icon("logout"))
	}
	r.w(`</div>`)
}

// accountItem is an item of the account menu: a link to the design's own
// page, else the dialog the mockup draws.
func (r *render) accountItem(label, ic, dialog, page string) {
	if page != "" {
		r.f(`<a class="fk-menu-item" role="menuitem" href="%s" data-foundry-link="%s">%s%s</a>`, esc(r.doc.href(page)), esc(page), icon(ic), esc(label))
		return
	}
	r.f(`<button class="fk-menu-item" type="button" role="menuitem" popovertarget="%s" popovertargetaction="show">%s%s</button>`, dialog, icon(ic), esc(label))
}

// accountItemIn is an account entry of another list (the global search).
func (r *render) accountItemIn(cls, label, ic, dialog, page string) {
	if page != "" {
		r.f(`<a class="%s" href="%s" data-foundry-link="%s">%s<span>%s</span></a>`, cls, esc(r.doc.href(page)), esc(page), icon(ic), esc(label))
		return
	}
	r.f(`<button class="%s" type="button" popovertarget="%s" popovertargetaction="show">%s<span>%s</span></button>`, cls, dialog, icon(ic), esc(label))
}

// accountDialogs are the profile and password dialogs of the account menu
// the design has no page for. They sit outside the top bar: an open one
// covers the page.
func (r *render) accountDialogs() {
	if !r.account || r.state != "" {
		return
	}
	if r.accountPage(false) == "" {
		r.accountDialog(accProfile, "Profil saya", "Data akun yang Anda pakai untuk masuk.", actProfile, func() {
			name := sample("nama", 0)
			r.f(`<div class="fk-account-id"><span class="fk-avatar">%s</span><span><b>%s</b><small>%s</small></span></div>`, esc(initials(name)), esc(name), esc(r.userRole()))
			r.accountField(accProfile, labelName, "text", name, false)
			r.accountField(accProfile, labelEmail, "email", sample("email", 0), true)
			r.accountField(accProfile, labelRoleName, "text", r.userRole(), true)
		})
	}
	if r.accountPage(true) == "" {
		r.accountDialog(accPassword, "Ganti kata sandi", "Setelah diganti, gunakan kata sandi baru saat masuk berikutnya.", actPassword, func() {
			r.accountField(accPassword, labelCurrent, "password", "", false)
			r.accountField(accPassword, labelNew, "password", "", false)
			r.accountField(accPassword, labelRepeat, "password", "", false)
		})
	}
}

// accountDialog draws dialog id: its title, note, fields and buttons. In
// the live mockup saving answers with the action's outcome.
func (r *render) accountDialog(id, title, note, act string, fields func()) {
	attrs, open := r.popAttrs(id)
	r.f(`<section class="fk-dialog fk-modal"%s role="dialog" aria-label="%s" data-foundry-dialog="account">`, attrs, esc(title))
	r.f(`<header class="fk-modal-head"><div class="fk-modal-titles"><h2>%s</h2><p class="fk-modal-desc">%s</p></div>`, esc(title), esc(note))
	r.closeX(id, open)
	r.w(`</header>`)
	action := r.doc.Self
	if action == "" {
		action = "#"
	}
	r.f(`<form class="fk-form" method="get" action="%s" data-foundry-sim="form">`, esc(action))
	if r.doc.live() {
		r.hidden(r.outcome(act, id))
	}
	if r.doc.Sim.View == id && r.doc.Sim.Feedback == "error" && id == accPassword {
		r.alert("error", "Kata sandi belum diganti", failPassword, "", "")
	}
	fields()
	r.w(`<div class="fk-actions">`)
	r.dismiss("Batal", id, open)
	if r.doc.live() {
		r.f(`<button class="fk-btn fk-btn-primary" type="submit">%s</button>`, esc(act))
	} else {
		r.f(`<button class="fk-btn fk-btn-primary" type="button" popovertarget="%s" popovertargetaction="hide">%s</button>`, id, esc(act))
	}
	r.w(`</div></form></section>`)
	if open {
		r.w(`</div>`)
	}
}

// accountField is a field of an account dialog; a read-only one shows what
// only an admin changes.
func (r *render) accountField(dialog, label, typ, value string, readonly bool) {
	id := dialog + "-" + ident(label)
	attrs := ` required`
	if readonly {
		attrs = ` readonly aria-readonly="true"`
	}
	cls, msg := "fk-field", label+" wajib diisi."
	if dialog == accPassword && label == labelCurrent && r.doc.Sim.View == dialog && r.doc.Sim.Feedback == "error" {
		cls, msg = cls+" fk-invalid", failPassword
		attrs += ` aria-invalid="true"`
	}
	if typ == "password" {
		attrs += ` autocomplete="` + map[bool]string{true: "current-password", false: "new-password"}[label == labelCurrent] + `"`
	}
	r.f(`<div class="%s"><label for="%s">%s</label><input id="%s" name="%s" type="%s" class="fk-input" value="%s" aria-describedby="%s-m"%s>`,
		cls, id, esc(label), id, id, typ, esc(value), id, attrs)
	if readonly {
		r.w(`<p class="fk-help">Diatur oleh admin.</p>`)
	}
	r.f(`<p class="fk-err" id="%s-m">%s%s</p></div>`, id, icon("alert-circle"), esc(msg))
}
