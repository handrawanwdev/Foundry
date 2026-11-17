package core

import (
	"regexp"
	"strings"
)

// Every app people sign in to lets them see their own account, change
// their password and sign out. The mockups always draw that account menu;
// the gate warns while the blueprint (use cases, requirements, features)
// leaves it out, since what the blueprint leaves out is not built.

// accountNeed is a part of the account standard and the words that cover
// it in the blueprint.
type accountNeed struct {
	name, use string
	covered   *regexp.Regexp
}

var accountNeeds = []accountNeed{
	{"keluar (logout)", "pengguna keluar dan sesinya diakhiri",
		regexp.MustCompile(`log\s?out|sign\s?out|keluar (dari )?(akun|aplikasi|sistem|sesi)|akhiri sesi`)},
	{"ganti kata sandi", "pengguna mengganti kata sandinya sendiri",
		regexp.MustCompile(`(ganti|ubah|perbarui|change|update)\s+(kata\s?sandi|password|sandi)`)},
	{"profil pengguna", "pengguna melihat dan memperbarui profilnya",
		regexp.MustCompile(`profil (saya|pengguna|akun|user)|akun saya|my (profile|account)|user profile|(edit|ubah|kelola|lihat) profil`)},
}

// passwordWords say people sign in with a password.
var passwordWords = regexp.MustCompile(`kata\s?sandi|password|sandi`)

// signsIn reports whether people sign in to the app: a sign-in page, or a
// page only signed-in people open.
func signsIn(l *LayoutSpec) bool {
	for _, pg := range l.Pages {
		if pg.Pattern == "auth" || pg.Access == "auth" {
			return true
		}
	}
	return false
}

// blueprintText is what the blueprint says will be built: its use cases,
// requirements and features, lower-cased.
func (p *Project) blueprintText() string {
	var b strings.Builder
	add := func(s ...string) {
		for _, x := range s {
			b.WriteString(strings.ToLower(x))
			b.WriteString(" | ")
		}
	}
	for _, fl := range p.Flows {
		add(fl.Name, fl.Trigger)
		add(fl.Steps...)
		add(fl.Alternatives...)
	}
	for _, r := range p.Requirements {
		add(r.Text)
	}
	for _, f := range p.Features {
		add(f.Name, f.Description)
		for _, s := range f.SubFeatures {
			add(s.Name)
		}
	}
	return b.String()
}

// MissingAccount are the parts of the account standard an app people sign
// in to leaves out of its blueprint. Changing the password is left out
// when nobody signs in with one (single sign-on).
func (p *Project) MissingAccount(l *LayoutSpec) []string {
	if l == nil || !signsIn(l) {
		return nil
	}
	text := p.blueprintText()
	password := passwordWords.MatchString(text)
	for _, pg := range l.Pages {
		if pg.Pattern == "auth" && passwordWords.MatchString(strings.ToLower(pageText(&pg))) {
			password = true
		}
	}
	var out []string
	for _, n := range accountNeeds {
		if n.name == "ganti kata sandi" && !password {
			continue
		}
		if !n.covered.MatchString(text) {
			out = append(out, n.name)
		}
	}
	return out
}

// accountCheck warns while the blueprint leaves out a part of the account
// standard. It never blocks: the mockups draw the account menu anyway.
func accountCheck(p *Project, l *LayoutSpec) GateCheck {
	c := check("account-standard", "Standar akun pengguna")
	for _, name := range p.MissingAccount(l) {
		for _, n := range accountNeeds {
			if n.name == name {
				c.note(GateWarn, "%s belum ada di use case, requirement atau fitur: tambahkan use case \"%s\" agar ikut dibangun", n.name, n.use)
			}
		}
	}
	return c
}
