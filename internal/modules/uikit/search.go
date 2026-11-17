package uikit

import "github.com/handrawanwdev/foundry/internal/core"

// Every app people sign in to has a global search in its top bar (⌘K): it
// opens a command palette that jumps to any page of the menu, starts the
// records the lists create, and reaches the account. The mockup draws it
// without the search itself: what it lists is what the design offers.

// searchButton is the top bar's search: a field-like button that opens the
// command palette.
func (r *render) searchButton(items []navItem) {
	r.cmdkItems = items
	r.w(`<button class="fk-search" type="button" aria-haspopup="dialog" aria-keyshortcuts="Meta+K Control+K" popovertarget="fk-cmdk">` + icon("search") +
		`<span>Cari halaman, data, atau aksi…</span><span class="fk-kbd">⌘K</span></button>`)
}

// commandPalette is the dialog the global search opens.
func (r *render) commandPalette() {
	if r.cmdkItems == nil || r.state != "" {
		return
	}
	r.w(`<div id="fk-cmdk" popover class="fk-cmdk" role="dialog" aria-label="Pencarian global">`)
	r.w(`<div class="fk-cmdk-input">` + icon("search") + `<input type="search" class="fk-cmdk-field" placeholder="Cari halaman, data, atau aksi…" aria-label="Cari" autofocus>` +
		`<button class="fk-kbd" type="button" popovertarget="fk-cmdk" popovertargetaction="hide" aria-label="Tutup">Esc</button></div><div class="fk-cmdk-list">`)
	r.w(`<p class="fk-cmdk-group">Halaman</p>`)
	for _, it := range r.cmdkItems {
		cur := ""
		if it.page == r.page.ID {
			cur = ` aria-current="page"`
		}
		r.f(`<a class="fk-cmdk-item" href="%s" data-foundry-link="%s"%s>%s<span>%s</span><small>%s</small></a>`,
			esc(r.doc.href(it.page)), esc(it.page), cur, icon(iconFor(it.label+" "+it.page)), esc(it.label), esc(it.group))
	}
	// The records the menu's lists create, each with its form.
	var acts []string
	for _, it := range r.cmdkItems {
		if form, label := core.CreateForm(r.doc.Layout, it.page); form != "" {
			acts = append(acts, form, label)
		}
	}
	if len(acts) > 0 {
		r.w(`<p class="fk-cmdk-group">Aksi cepat</p>`)
		for i := 0; i < len(acts); i += 2 {
			r.f(`<a class="fk-cmdk-item" href="%s" data-foundry-link="%s">%s<span>%s</span></a>`, esc(r.doc.href(acts[i])), esc(acts[i]), icon("plus"), esc(acts[i+1]))
		}
	}
	if r.account {
		r.w(`<p class="fk-cmdk-group">Akun</p>`)
		r.accountItemIn("fk-cmdk-item", "Profil saya", "user", accProfile, r.accountPage(false))
		r.accountItemIn("fk-cmdk-item", "Ganti kata sandi", "key", accPassword, r.accountPage(true))
	}
	r.w(`</div><footer class="fk-cmdk-foot"><span><span class="fk-kbd">↑</span><span class="fk-kbd">↓</span> pilih</span><span><span class="fk-kbd">↵</span> buka</span><span><span class="fk-kbd">Esc</span> tutup</span></footer></div>`)
}
