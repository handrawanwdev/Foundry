package core

import (
	"strings"
	"testing"
)

// An app people sign in to must plan signing out, changing the password
// and the user's profile; the gate warns until the blueprint does, and
// never blocks for it.
func TestAccountStandard(t *testing.T) {
	l := &LayoutSpec{Pages: []LayoutPage{{ID: "login", Name: "Masuk", Pattern: "auth", Sections: []Section{{ID: "f", Name: "Form", Content: []string{"Field email, kata sandi"}}}},
		{ID: "barang", Name: "Barang", Pattern: "table", Access: "auth"}}}
	p := &Project{Flows: []Flow{{ID: "FLOW-1", Name: "Catat barang keluar", Steps: []string{"Isi qty"}}}}
	if got := p.MissingAccount(l); strings.Join(got, ",") != "keluar (logout),ganti kata sandi,profil pengguna" {
		t.Errorf("kurang: %v", got)
	}
	c := accountCheck(p, l)
	if c.Status != GateWarn || len(c.Items) != 3 {
		t.Errorf("cek akun: %+v", c)
	}
	p.Features = []Feature{{ID: "F1", Name: "Identitas", Description: "Login/logout, ganti password, dan profil saya"}}
	if got := p.MissingAccount(l); len(got) != 0 {
		t.Errorf("sudah tercakup, masih kurang: %v", got)
	}
	// Single sign-on: no password to change.
	l.Pages[0].Sections = nil
	p.Features = []Feature{{ID: "F1", Name: "Identitas", Description: "Masuk dengan SSO, logout, profil saya"}}
	if got := p.MissingAccount(l); len(got) != 0 {
		t.Errorf("SSO masih diminta ganti kata sandi: %v", got)
	}
	// Nobody signs in: nothing to ask.
	if got := p.MissingAccount(&LayoutSpec{Pages: []LayoutPage{{ID: "a", Pattern: "table"}}}); got != nil {
		t.Errorf("app tanpa login: %v", got)
	}
}
