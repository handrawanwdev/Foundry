package uikit

import "strings"

// Icons are inline stroke SVGs (24px grid, currentColor) so mockups stay
// self-contained. They mark meaning, not brand: a coding agent maps them
// to the project's own icon set.
var icons = map[string]string{
	"search":       `<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>`,
	"filter":       `<path d="M3 5h18M6 12h12M10 19h4"/>`,
	"chevron":      `<path d="m6 9 6 6 6-6"/>`,
	"left":         `<path d="m15 18-6-6 6-6"/>`,
	"right":        `<path d="m9 18 6-6-6-6"/>`,
	"plus":         `<path d="M12 5v14M5 12h14"/>`,
	"download":     `<path d="M12 4v11m-5-5 5 5 5-5M5 20h14"/>`,
	"more":         `<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>`,
	"bell":         `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9"/><path d="M10.3 21a2 2 0 0 0 3.4 0"/>`,
	"menu":         `<path d="M4 6h16M4 12h16M4 18h16"/>`,
	"grid":         `<rect x="3" y="3" width="7" height="9" rx="1"/><rect x="14" y="3" width="7" height="5" rx="1"/><rect x="14" y="12" width="7" height="9" rx="1"/><rect x="3" y="16" width="7" height="5" rx="1"/>`,
	"list":         `<path d="M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01"/>`,
	"users":        `<circle cx="9" cy="8" r="4"/><path d="M2 21a7 7 0 0 1 14 0M16 3.1a4 4 0 0 1 0 7.8M22 21a7 7 0 0 0-4-6.3"/>`,
	"settings":     `<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M4.9 4.9 7 7M17 17l2.1 2.1M2 12h3M19 12h3M4.9 19.1 7 17M17 7l2.1-2.1"/>`,
	"file":         `<path d="M14 3H6a1 1 0 0 0-1 1v16a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1V8z"/><path d="M14 3v5h5M9 13h6M9 17h6"/>`,
	"chart":        `<path d="M3 3v18h18"/><path d="M7 16v-4M12 16V8M17 16v-7"/>`,
	"card":         `<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/>`,
	"box":          `<path d="M21 8 12 3 3 8v8l9 5 9-5z"/><path d="m3 8 9 5 9-5M12 13v8"/>`,
	"pin":          `<path d="M12 21s-7-6.2-7-12a7 7 0 0 1 14 0c0 5.8-7 12-7 12z"/><circle cx="12" cy="9" r="2.5"/>`,
	"device":       `<rect x="5" y="5" width="14" height="14" rx="2"/><path d="M9 9h6v6H9zM9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>`,
	"calendar":     `<rect x="3" y="5" width="18" height="16" rx="2"/><path d="M3 10h18M8 3v4M16 3v4"/>`,
	"shield":       `<path d="M12 3 4 6v6c0 5 3.5 8 8 9 4.5-1 8-4 8-9V6z"/>`,
	"tag":          `<path d="M3 12V4a1 1 0 0 1 1-1h8l9 9-9 9z"/><circle cx="7.5" cy="7.5" r="1.5"/>`,
	"truck":        `<path d="M3 6h11v10H3zM14 9h4l3 3v4h-7"/><circle cx="7" cy="18" r="2"/><circle cx="17" cy="18" r="2"/>`,
	"cart":         `<circle cx="9" cy="20" r="1.5"/><circle cx="18" cy="20" r="1.5"/><path d="M2 3h3l2.7 12.4a1 1 0 0 0 1 .8h9.6a1 1 0 0 0 1-.8L21 7H6"/>`,
	"clock":        `<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>`,
	"enter":        `<path d="M15 3h4a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-4M10 17l5-5-5-5M15 12H3"/>`,
	"check":        `<circle cx="12" cy="12" r="9"/><path d="m8 12 3 3 5-6"/>`,
	"alert":        `<path d="M12 3 2 20h20z"/><path d="M12 10v4M12 17h.01"/>`,
	"alert-circle": `<circle cx="12" cy="12" r="9"/><path d="M12 7.5v5.5M12 16.5h.01"/>`,
	"close":        `<path d="M18 6 6 18M6 6l12 12"/>`,
	"edit":         `<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m13.5 6.5 4 4"/>`,
	"trash":        `<path d="M4 7h16M10 7V4h4v3M6 7l1 13h10l1-13M10 11v5M14 11v5"/>`,
	"user":         `<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>`,
	"key":          `<circle cx="8" cy="15" r="4"/><path d="m10.8 12.2 9.2-9.2M17 6l3 3M14 9l2 2"/>`,
	"logout":       `<path d="M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 17l-5-5 5-5M5 12h11"/>`,
	"eye":          `<path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>`,
	"upload":       `<path d="M12 16V4m-5 5 5-5 5 5M5 20h14"/>`,
	"lock":         `<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4"/>`,
	"trend":        `<path d="m3 17 6-6 4 4 8-8M15 7h6v6"/>`,
	"building":     `<path d="M4 21V5a1 1 0 0 1 1-1h9a1 1 0 0 1 1 1v16M15 9h4a1 1 0 0 1 1 1v11M8 8h3M8 12h3M8 16h3M3 21h18"/>`,
	"home":         `<path d="m3 10 9-7 9 7v10a1 1 0 0 1-1 1h-5v-7H9v7H4a1 1 0 0 1-1-1z"/>`,
	"mail":         `<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m3 7 9 6 9-6"/>`,
}

// icon is the inline SVG of an icon name.
func icon(name string) string {
	p, ok := icons[name]
	if !ok {
		p = icons["file"]
	}
	return `<svg class="fk-i" viewBox="0 0 24 24" aria-hidden="true">` + p + `</svg>`
}

// iconFor picks the icon a label is about.
func iconFor(label string) string {
	l := strings.ToLower(label)
	for _, c := range []struct{ icon, words string }{
		{"grid", "dasbor dashboard pusat beranda home ringkasan overview kendali monitor"},
		{"chart", "laporan report analitik statistik grafik"},
		{"settings", "pengaturan setting konfigurasi preferensi"},
		{"card", "bayar kasir keuangan tagihan invoice transaksi kas dompet billing"},
		{"tag", "tarif harga produk katalog promo diskon"},
		{"cart", "pembelian penjualan order pesanan belanja"},
		{"truck", "kirim pengiriman distribusi armada pemenuhan picking"},
		{"box", "persediaan stok inventori barang gudang penerimaan"},
		{"pin", "lokasi cabang area zona peta"},
		{"device", "perangkat device sensor mesin"},
		{"enter", "gerbang akses masuk login"},
		{"clock", "sesi riwayat log jejak waktu antrean antrian"},
		{"shield", "audit keamanan peran izin role"},
		{"check", "persetujuan approval verifikasi"},
		{"calendar", "jadwal kalender reservasi booking janji"},
		{"building", "organisasi tenant perusahaan platform unit"},
		{"users", "member anggota pengguna user pelanggan karyawan pasien siswa tim kontak"},
		{"bell", "notifikasi peringatan alert"},
		{"mail", "pesan email inbox"},
		{"list", "data master daftar"},
	} {
		for _, w := range strings.Fields(c.words) {
			if wordMatch(l, w) {
				return c.icon
			}
		}
	}
	return "file"
}

// wordMatch reports whether text has a word starting with w; a long w may
// also sit inside an affixed word ("bayar" in "pembayaran").
func wordMatch(text, w string) bool {
	if len(w) >= 5 {
		return strings.Contains(text, w)
	}
	for _, f := range strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == '-' || r == '/' || r == '_' }) {
		if strings.HasPrefix(f, w) {
			return true
		}
	}
	return false
}
