package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Design IR: a page is the source of truth for both the preview renderer and
// the build. Layout, component tree, responsive changes and visual intent are
// explicit and use a finite vocabulary, so the renderer only translates them
// and never picks a layout itself.

// PageLayout is the page composition. Frame is what surrounds the content:
// app (top bar and the nav slot), center (a centered panel, e.g. sign-in) or
// full (content only). Nav places the page's nav components. Width is the
// content width. Devices change the composition per device.
type PageLayout struct {
	Frame   string                  `json:"frame"`
	Nav     string                  `json:"nav,omitempty"`
	Width   string                  `json:"width,omitempty"`
	Brand   bool                    `json:"brand,omitempty"` // app name above the center panel
	Back    string                  `json:"back,omitempty"`  // page id of the back link
	Root    LayoutNode              `json:"root"`
	Devices map[string]DeviceLayout `json:"devices,omitempty"`
}

// LayoutNode is a layout primitive with children, or a leaf that places one
// component. In JSON a leaf is the component id as a plain string.
type LayoutNode struct {
	Component string       `json:"-"`
	ID        string       `json:"id,omitempty"`
	Type      string       `json:"type,omitempty"`
	Children  []LayoutNode `json:"children,omitempty"`
	Columns   int          `json:"columns,omitempty"`   // grid: 1-4
	Ratio     string       `json:"ratio,omitempty"`     // split: 1:1 (default) | 1:2 | 2:1 | 1:3 | 3:1
	Side      string       `json:"side,omitempty"`      // sidebar: start (default) | end, the narrow column
	Placement string       `json:"placement,omitempty"` // overlay: drawer (default) | modal | sheet
	Gap       string       `json:"gap,omitempty"`       // space token
	Align     string       `json:"align,omitempty"`
	Surface   string       `json:"surface,omitempty"`   // surface token
	Elevation string       `json:"elevation,omitempty"` // elevation token
}

// DeviceLayout is what changes on one device. Hide takes component or node
// ids; Nodes changes a node's type, columns or child order; Swap draws a
// component as another type (table ↔ list).
type DeviceLayout struct {
	Frame string                `json:"frame,omitempty"`
	Nav   string                `json:"nav,omitempty"`
	Width string                `json:"width,omitempty"`
	Hide  []string              `json:"hide,omitempty"`
	Nodes map[string]NodeChange `json:"nodes,omitempty"`
	Swap  map[string]string     `json:"swap,omitempty"`
}

type NodeChange struct {
	Type    string   `json:"type,omitempty"`
	Columns int      `json:"columns,omitempty"`
	Order   []string `json:"order,omitempty"` // child keys: component ids and node ids
}

func (n LayoutNode) MarshalJSON() ([]byte, error) {
	if n.Component != "" {
		return json.Marshal(n.Component)
	}
	type plain LayoutNode
	return json.Marshal(plain(n))
}

func (n *LayoutNode) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*n = LayoutNode{Component: s}
		return nil
	}
	type plain LayoutNode
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*n = LayoutNode(p)
	return nil
}

// Key is the component id of a leaf, else the node id.
func (n LayoutNode) Key() string {
	if n.Component != "" {
		return n.Component
	}
	return n.ID
}

// The finite vocabulary of the IR.
var (
	LayoutPrimitives = []string{"stack", "row", "grid", "split", "sidebar", "center", "full", "overlay"}
	Frames           = []string{"app", "center", "full"}
	NavPlacements    = []string{"side", "rail", "top", "bottom", "none"}
	Widths           = []string{"narrow", "medium", "wide", "full"}
	Devices          = []string{"desktop", "tablet", "mobile"}
	Densities        = []string{"compact", "comfortable", "spacious"}
	SpaceTokens      = []string{"space.none", "space.xs", "space.sm", "space.md", "space.lg", "space.xl"}
	SurfaceTokens    = []string{"surface.primary", "surface.secondary"}
	ElevationTokens  = []string{"elevation.none", "elevation.raised", "elevation.overlay"}
	EmphasisTokens   = []string{"action.primary", "action.secondary", "action.ghost", "action.danger"}
	ToneTokens       = []string{"text.primary", "text.secondary", "status.success", "status.warning", "status.error", "status.info"}
	Aligns           = []string{"start", "center", "end", "between", "stretch"}
	Ratios           = []string{"1:1", "1:2", "2:1", "1:3", "3:1"}
	Sides            = []string{"start", "end"}
	Placements       = []string{"drawer", "modal", "sheet"}
	// Swaps are the component substitutions a device may make.
	Swaps = map[string]string{"table": "list", "list": "table"}
)

// legacyDensity maps the density names of older designs.
var legacyDensity = map[string]string{"low": "spacious", "medium": "comfortable", "high": "compact"}

// NormDensity returns the density name, mapping older names; "" when unknown.
func NormDensity(d string) string {
	d = strings.ToLower(strings.TrimSpace(d))
	if m, ok := legacyDensity[d]; ok {
		return m
	}
	if slices.Contains(Densities, d) {
		return d
	}
	return ""
}

// BaseDevice is the device a page is designed for first.
func BaseDevice(pg Page) string {
	if pg.Platform == "android" || pg.Platform == "ios" {
		return "mobile"
	}
	return "desktop"
}

// ---------- Resolve ----------

// Resolve returns the layout as drawn on one device: frame, nav and width
// overridden, hidden components and nodes dropped, node changes applied.
// The swaps of that device are returned separately.
func (l PageLayout) Resolve(device string) (PageLayout, map[string]string) {
	d := l.Devices[device]
	out := PageLayout{Frame: l.Frame, Nav: l.Nav, Width: l.Width, Brand: l.Brand, Back: l.Back}
	if d.Frame != "" {
		out.Frame = d.Frame
	}
	if d.Nav != "" {
		out.Nav = d.Nav
	}
	if d.Width != "" {
		out.Width = d.Width
	}
	out.Root = resolveNode(l.Root, d)
	return out, d.Swap
}

func resolveNode(n LayoutNode, d DeviceLayout) LayoutNode {
	if n.Component != "" {
		return n
	}
	out := n
	out.Children = nil
	ch := d.Nodes[n.ID]
	if n.ID == "" {
		ch = NodeChange{}
	}
	if ch.Type != "" {
		out.Type = ch.Type
	}
	if ch.Columns > 0 {
		out.Columns = ch.Columns
	}
	kids := n.Children
	if len(ch.Order) > 0 {
		kids = ordered(kids, ch.Order)
	}
	for _, k := range kids {
		if slices.Contains(d.Hide, k.Key()) {
			continue
		}
		out.Children = append(out.Children, resolveNode(k, d))
	}
	return out
}

// ordered puts the children named in order first, in that order; the rest
// keep their place after them.
func ordered(kids []LayoutNode, order []string) []LayoutNode {
	var out []LayoutNode
	for _, key := range order {
		if i := slices.IndexFunc(kids, func(k LayoutNode) bool { return k.Key() == key }); i >= 0 {
			out = append(out, kids[i])
		}
	}
	for _, k := range kids {
		if !slices.Contains(order, k.Key()) {
			out = append(out, k)
		}
	}
	return out
}

// Walk calls fn for every node and leaf, parents first.
func (n LayoutNode) Walk(fn func(n LayoutNode, parent *LayoutNode)) {
	var walk func(n LayoutNode, parent *LayoutNode)
	walk = func(n LayoutNode, parent *LayoutNode) {
		fn(n, parent)
		for _, k := range n.Children {
			walk(k, &n)
		}
	}
	walk(n, nil)
}

// ---------- Validation ----------

// IRInvalid is a design that does not satisfy the IR. Nothing of it is
// applied or rendered.
type IRInvalid struct{ Problems []string }

func (e *IRInvalid) Error() string {
	return "DESIGN_IR_INVALID\n\n- " + strings.Join(e.Problems, "\n- ")
}

// ValidateDesign checks the given pages; pages is every page of the design,
// for page references (back link).
func ValidateDesign(check, pages []Page) error {
	ids := map[string]bool{}
	for _, pg := range pages {
		ids[pg.ID] = true
	}
	var out []string
	for _, pg := range check {
		for _, p := range ValidatePage(pg, ids) {
			out = append(out, fmt.Sprintf("%s %q: %s", pg.ID, pg.Name, p))
		}
	}
	if len(out) > 0 {
		return &IRInvalid{Problems: out}
	}
	return nil
}

// ValidatePage lists what is wrong with one page's IR.
func ValidatePage(pg Page, pageIDs map[string]bool) []string {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	oneOf := func(what, v string, set []string) {
		if v != "" && !slices.Contains(set, v) {
			bad("%s %q tidak dikenal (pakai: %s)", what, v, strings.Join(set, ", "))
		}
	}

	oneOf("density", pg.Density, Densities)
	comps := map[string]PageComponent{}
	primary := 0
	for i, c := range pg.Components {
		switch {
		case c.ID == "":
			bad("komponen [%d] %s tanpa id", i, c.Type)
		case comps[c.ID].Type != "":
			bad("id komponen %q dipakai lebih dari sekali", c.ID)
		default:
			comps[c.ID] = c
		}
		oneOf(fmt.Sprintf("komponen %q emphasis", c.ID), c.Emphasis, EmphasisTokens)
		oneOf(fmt.Sprintf("komponen %q tone", c.ID), c.Tone, ToneTokens)
		if c.Emphasis == "action.primary" {
			primary++
		}
	}
	if primary > 1 {
		bad("%d komponen memakai action.primary; maksimal satu aksi utama per halaman", primary)
	}

	l := pg.Layout
	if l == nil {
		return append(out, "layout belum ditentukan")
	}
	oneOf("frame", l.Frame, Frames)
	if l.Frame == "" {
		bad("frame kosong (pakai: %s)", strings.Join(Frames, ", "))
	}
	oneOf("nav", l.Nav, NavPlacements)
	oneOf("width", l.Width, Widths)
	if l.Back != "" && (!pageIDs[l.Back] || l.Back == pg.ID) {
		bad("back menuju halaman %q yang tidak ada", l.Back)
	}
	if l.Root.Component != "" || l.Root.Type == "" {
		bad("root harus node layout, bukan komponen")
	}

	// Tree: every node known, every non-nav component placed exactly once.
	nodes := map[string]LayoutNode{}
	placed := map[string]int{}
	l.Root.Walk(func(n LayoutNode, parent *LayoutNode) {
		if n.Component != "" {
			c, ok := comps[n.Component]
			switch {
			case !ok:
				bad("layout menempatkan komponen %q yang tidak ada", n.Component)
			case c.Type == "nav":
				bad("komponen nav %q tidak boleh di layout; posisinya diatur \"nav\"", n.Component)
			}
			placed[n.Component]++
			return
		}
		name := n.ID
		if name == "" {
			name = n.Type
		}
		if !slices.Contains(LayoutPrimitives, n.Type) {
			bad("node %q: primitive %q tidak didukung (pakai: %s)", name, n.Type, strings.Join(LayoutPrimitives, ", "))
		}
		if n.ID != "" {
			if _, dup := nodes[n.ID]; dup || comps[n.ID].Type != "" {
				bad("id node %q dipakai lebih dari sekali", n.ID)
			}
			nodes[n.ID] = n
		}
		if len(n.Children) == 0 && parent != nil {
			bad("node %q tidak punya isi", name)
		}
		if n.Type == "grid" && (n.Columns < 1 || n.Columns > 4) {
			bad("node %q: grid butuh columns 1-4", name)
		}
		if n.Type != "grid" && n.Columns != 0 {
			bad("node %q: columns hanya untuk grid", name)
		}
		oneOf(fmt.Sprintf("node %q ratio", name), n.Ratio, Ratios)
		oneOf(fmt.Sprintf("node %q side", name), n.Side, Sides)
		oneOf(fmt.Sprintf("node %q placement", name), n.Placement, Placements)
		oneOf(fmt.Sprintf("node %q gap", name), n.Gap, SpaceTokens)
		oneOf(fmt.Sprintf("node %q align", name), n.Align, Aligns)
		oneOf(fmt.Sprintf("node %q surface", name), n.Surface, SurfaceTokens)
		oneOf(fmt.Sprintf("node %q elevation", name), n.Elevation, ElevationTokens)
	})
	var navs int
	for _, c := range pg.Components {
		if c.Type == "nav" {
			navs++
			continue
		}
		switch n := placed[c.ID]; {
		case c.ID == "":
		case n == 0:
			bad("komponen %q tidak ditempatkan di layout (orphan)", c.ID)
		case n > 1:
			bad("komponen %q ditempatkan %d kali di layout", c.ID, n)
		}
	}
	surfaceNesting(l.Root, false, func(id string) {
		bad("node %q ber-surface di dalam node ber-surface (kartu di dalam kartu)", id)
	})

	// Devices: references and the composition each device ends up with.
	for dev, d := range l.Devices {
		if !slices.Contains(Devices, dev) {
			bad("perangkat %q tidak dikenal (pakai: %s)", dev, strings.Join(Devices, ", "))
			continue
		}
		oneOf(dev+" frame", d.Frame, Frames)
		oneOf(dev+" nav", d.Nav, NavPlacements)
		oneOf(dev+" width", d.Width, Widths)
		for _, h := range d.Hide {
			if _, ok := nodes[h]; !ok && comps[h].Type == "" {
				bad("%s: hide %q tidak ada", dev, h)
			}
		}
		for id, ch := range d.Nodes {
			n, ok := nodes[id]
			if !ok {
				bad("%s: node %q tidak ada", dev, id)
				continue
			}
			oneOf(fmt.Sprintf("%s node %q type", dev, id), ch.Type, LayoutPrimitives)
			if ch.Columns < 0 || ch.Columns > 4 {
				bad("%s node %q: columns 1-4", dev, id)
			}
			if len(ch.Order) > 0 {
				var keys []string
				for _, k := range n.Children {
					keys = append(keys, k.Key())
				}
				a, b := slices.Clone(ch.Order), slices.Clone(keys)
				slices.Sort(a)
				slices.Sort(b)
				if !slices.Equal(a, b) || slices.Contains(keys, "") {
					bad("%s node %q: order harus berisi tepat anak node: %s", dev, id, strings.Join(keys, ", "))
				}
			}
		}
		for id, to := range d.Swap {
			c, ok := comps[id]
			if !ok {
				bad("%s: swap komponen %q tidak ada", dev, id)
			} else if Swaps[c.Type] != to {
				bad("%s: swap %s %q ke %q tidak didukung (hanya table ↔ list)", dev, c.Type, id, to)
			}
		}
	}
	mobileRules := pg.Platform != "desktop"
	for _, dev := range Devices {
		r, _ := l.Resolve(dev)
		if r.Nav != "none" && r.Nav != "" && navs == 0 && r.Frame == "app" {
			bad("%s: nav %q tanpa komponen nav", dev, r.Nav)
		}
		r.Root.Walk(func(n LayoutNode, _ *LayoutNode) {
			if n.Component != "" {
				return
			}
			if (n.Type == "split" || n.Type == "sidebar") && len(n.Children) != 2 {
				bad("%s: node %q (%s) butuh tepat 2 anak, ada %d", dev, n.Key(), n.Type, len(n.Children))
			}
			if n.Type == "grid" && n.Columns == 0 {
				bad("%s: node %q grid butuh columns", dev, n.Key())
			}
			if dev == "mobile" && mobileRules {
				switch {
				case n.Type == "split" || n.Type == "sidebar":
					bad("mobile: node %q masih %s; ubah lewat devices.mobile.nodes.%s.type", n.Key(), n.Type, n.Key())
				case n.Type == "grid" && n.Columns > 2:
					bad("mobile: node %q grid %d kolom; maksimal 2 lewat devices.mobile.nodes.%s.columns", n.Key(), n.Columns, n.Key())
				}
			}
		})
		if dev == "mobile" && mobileRules && r.Frame == "app" && (r.Nav == "side" || r.Nav == "rail") {
			bad("mobile: nav %q tidak muat; pakai devices.mobile.nav bottom, top atau none", r.Nav)
		}
	}
	return out
}

func surfaceNesting(n LayoutNode, inSurface bool, bad func(id string)) {
	if n.Component != "" {
		return
	}
	if n.Surface != "" {
		if inSurface {
			bad(n.Key())
		}
		inSurface = true
	}
	for _, k := range n.Children {
		surfaceNesting(k, inSurface, bad)
	}
}

// ---------- Tree edits (used by patches) ----------

// Clone returns a deep copy, so edits never reach the previous version.
func (l *PageLayout) Clone() *PageLayout {
	if l == nil {
		return nil
	}
	raw, _ := json.Marshal(l)
	var out PageLayout
	_ = json.Unmarshal(raw, &out)
	return &out
}

// Detach removes a component or node from the tree and from every device
// reference.
func (l *PageLayout) Detach(key string) {
	l.Root = detach(l.Root, key)
	for dev, d := range l.Devices {
		d.Hide = slices.DeleteFunc(slices.Clone(d.Hide), func(h string) bool { return h == key })
		if d.Swap != nil {
			delete(d.Swap, key)
		}
		for id, ch := range d.Nodes {
			ch.Order = slices.DeleteFunc(slices.Clone(ch.Order), func(o string) bool { return o == key })
			d.Nodes[id] = ch
		}
		l.Devices[dev] = d
	}
}

func detach(n LayoutNode, key string) LayoutNode {
	if n.Component != "" {
		return n
	}
	var kids []LayoutNode
	for _, k := range n.Children {
		if k.Component == key {
			continue
		}
		kids = append(kids, detach(k, key))
	}
	n.Children = kids
	return n
}

// Place puts a component leaf right after anchor in anchor's parent; with no
// anchor in the tree it goes to the top (top) or the bottom of the root.
// Device child orders of that parent get it at the same place.
func (l *PageLayout) Place(key, anchor string, top bool) {
	parent := ""
	if anchor != "" && placeAfter(&l.Root, key, anchor, &parent) {
		for _, d := range l.Devices {
			if ch, ok := d.Nodes[parent]; ok && len(ch.Order) > 0 {
				i := slices.Index(ch.Order, anchor)
				ch.Order = slices.Insert(slices.Clone(ch.Order), i+1, key)
				d.Nodes[parent] = ch
			}
		}
		return
	}
	leaf := LayoutNode{Component: key}
	if top {
		l.Root.Children = slices.Insert(l.Root.Children, 0, leaf)
	} else {
		l.Root.Children = append(l.Root.Children, leaf)
	}
	for _, d := range l.Devices {
		if ch, ok := d.Nodes[l.Root.ID]; ok && len(ch.Order) > 0 && l.Root.ID != "" {
			if top {
				ch.Order = slices.Insert(slices.Clone(ch.Order), 0, key)
			} else {
				ch.Order = append(slices.Clone(ch.Order), key)
			}
			d.Nodes[l.Root.ID] = ch
		}
	}
}

func placeAfter(n *LayoutNode, key, anchor string, parent *string) bool {
	for i, k := range n.Children {
		if k.Component == anchor {
			n.Children = slices.Insert(slices.Clone(n.Children), i+1, LayoutNode{Component: key})
			*parent = n.ID
			return true
		}
	}
	for i := range n.Children {
		if n.Children[i].Component == "" && placeAfter(&n.Children[i], key, anchor, parent) {
			return true
		}
	}
	return false
}

// ---------- Upgrade of designs without IR ----------

var nameKinds = []struct {
	re   *regexp.Regexp
	kind string
}{
	{regexp.MustCompile(`login|masuk|sign ?in|sign ?up|registrasi|daftar akun|lupa (kata sandi|password)`), "auth"},
	{regexp.MustCompile(`pengaturan|setting|konfigurasi`), "settings"},
	{regexp.MustCompile(`profil|profile|akun saya`), "profile"},
	{regexp.MustCompile(`bayar|pembayaran|payment|checkout`), "payment"},
	{regexp.MustCompile(`cari|pencarian|search`), "search"},
	{regexp.MustCompile(`laporan|analitik|analytics|statistik`), "analytics"},
	{regexp.MustCompile(`berhasil|sukses|gagal|error`), "status"},
	{regexp.MustCompile(`dashboard|beranda|home`), "dashboard"},
}

var adminRole = regexp.MustCompile(`(?i)admin`)

// PageKind is the kind set by the designer, else the one older designs
// were drawn with (from the name and the components).
func PageKind(pg Page) string {
	if pg.Kind != "" {
		return pg.Kind
	}
	name := strings.ToLower(pg.Name)
	for _, nk := range nameKinds {
		if nk.re.MatchString(name) {
			return nk.kind
		}
	}
	count := func(t string) int {
		n := 0
		for _, c := range pg.Components {
			if c.Type == t {
				n++
			}
		}
		return n
	}
	switch {
	case count("steps") > 0:
		return "wizard"
	case count("table") > 0 && adminRole.MatchString(pg.Role):
		return "admin"
	case count("table") > 0:
		return "list"
	case count("stat") >= 2:
		return "dashboard"
	case count("form") > 0 || count("input") >= 2:
		return "form"
	case count("list") > 0:
		return "list"
	}
	return "detail"
}

var kindWidth = map[string]string{
	"auth": "narrow", "payment": "narrow", "status": "narrow",
	"form": "medium", "profile": "medium", "wizard": "medium",
	"detail": "wide", "search": "wide",
}

// panelKinds drew their form on a panel.
var panelKinds = map[string]bool{"form": true, "wizard": true, "payment": true, "profile": true}

// EnsureIR gives every page what the IR needs: component ids, the current
// density names and, for pages without a layout, the layout older designs
// were drawn with. Pages that already have a layout keep it.
func EnsureIR(pages []Page, app string) {
	for i := range pages {
		pg := &pages[i]
		pg.Density = NormDensity(pg.Density)
		AssignIDs(pg.Components)
		if pg.Layout == nil {
			pg.Layout = defaultLayout(pg, pages, app)
		}
	}
}

// AssignIDs gives components without an id a stable one: type and position.
func AssignIDs(comps []PageComponent) {
	used := map[string]bool{}
	for _, c := range comps {
		used[c.ID] = true
	}
	for i := range comps {
		if comps[i].ID != "" {
			continue
		}
		base := fmt.Sprintf("%s-%d", comps[i].Type, i+1)
		id := base
		for n := 2; used[id]; n++ {
			id = fmt.Sprintf("%s-%d", base, n)
		}
		comps[i].ID, used[id] = id, true
	}
}

// defaultLayout builds the layout a page of this kind was drawn with before
// the IR: auth in a centered panel, a toolbar row on list pages, stats side
// by side, forms on a panel, nav in the sidebar and a bottom tab bar on
// mobile. It also marks
// the primary button, which was picked by label before.
func defaultLayout(pg *Page, pages []Page, app string) *PageLayout {
	kind := PageKind(*pg)
	mobile := BaseDevice(*pg) == "mobile"
	l := &PageLayout{Frame: "app", Nav: "none", Width: kindWidth[kind], Root: LayoutNode{ID: "main", Type: "stack"}}
	if l.Width == "" {
		l.Width = "full"
	}
	mob := DeviceLayout{Nodes: map[string]NodeChange{}}
	hasNav := slices.ContainsFunc(pg.Components, func(c PageComponent) bool { return c.Type == "nav" })
	switch {
	case kind == "auth":
		l.Frame = "center"
		l.Brand = !slices.ContainsFunc(pg.Components, func(c PageComponent) bool {
			return c.Type == "header" && strings.EqualFold(strings.TrimSpace(c.Label), strings.TrimSpace(app))
		})
	case hasNav && mobile:
		l.Nav = "bottom"
	case hasNav:
		l.Nav = "side"
		mob.Nav = "bottom"
	}
	if kind == "detail" {
		for _, from := range pages {
			if from.ID != pg.ID && linksTo(from, pg.ID, pages) {
				l.Back = from.ID
				break
			}
		}
	}

	var body []PageComponent
	for _, c := range pg.Components {
		if c.Type != "nav" {
			body = append(body, c)
		}
	}
	if (kind == "list" || kind == "admin") && !mobile {
		bar := LayoutNode{ID: "toolbar", Type: "row", Align: "between"}
		var rest []PageComponent
		for _, c := range body {
			if c.Type == "header" || c.Type == "button" {
				bar.Children = append(bar.Children, LayoutNode{Component: c.ID})
			} else {
				rest = append(rest, c)
			}
		}
		if len(bar.Children) > 0 {
			l.Root.Children = append(l.Root.Children, bar)
			mob.Nodes["toolbar"] = NodeChange{Type: "stack"}
			body = rest
		}
	}
	for k := 0; k < len(body); k++ {
		if body[k].Type == "form" && panelKinds[kind] {
			l.Root.Children = append(l.Root.Children, LayoutNode{ID: "panel-" + body[k].ID, Type: "stack", Surface: "surface.primary",
				Children: []LayoutNode{{Component: body[k].ID}}})
			continue
		}
		if body[k].Type != "stat" {
			l.Root.Children = append(l.Root.Children, LayoutNode{Component: body[k].ID})
			continue
		}
		grid := LayoutNode{ID: "stats-" + body[k].ID, Type: "grid"}
		for ; k < len(body) && body[k].Type == "stat"; k++ {
			grid.Children = append(grid.Children, LayoutNode{Component: body[k].ID})
		}
		k--
		grid.Columns = min(len(grid.Children), 4)
		if mobile {
			grid.Columns = min(grid.Columns, 2)
		} else if grid.Columns > 2 {
			mob.Nodes[grid.ID] = NodeChange{Columns: 2}
		}
		l.Root.Children = append(l.Root.Children, grid)
	}
	if len(l.Root.Children) == 0 {
		l.Root.Children = nil
	}
	if mob.Nav != "" || len(mob.Nodes) > 0 {
		if len(mob.Nodes) == 0 {
			mob.Nodes = nil
		}
		l.Devices = map[string]DeviceLayout{"mobile": mob}
	}

	if !slices.ContainsFunc(pg.Components, func(c PageComponent) bool { return c.Emphasis != "" }) {
		if i := legacyPrimary(*pg); i >= 0 {
			pg.Components[i].Emphasis = "action.primary"
		}
	}
	return l
}

// legacyPrimary is the button older designs drew as primary: the one with
// the primary action label, else the first button that opens a page.
func legacyPrimary(pg Page) int {
	if pg.PrimaryAction == nil {
		return slices.IndexFunc(pg.Components, func(c PageComponent) bool { return c.Type == "button" && c.Target != "" })
	}
	label := strings.ToLower(strings.TrimSpace(pg.PrimaryAction.Label))
	if label == "" {
		return -1
	}
	return slices.IndexFunc(pg.Components, func(c PageComponent) bool {
		return c.Type == "button" && strings.ToLower(strings.TrimSpace(c.Label)) == label
	})
}

// linksTo reports whether from opens page id through a target or a nav item
// (a nav item opens the page of the same platform with the same name).
func linksTo(from Page, id string, pages []Page) bool {
	for _, c := range from.Components {
		if c.Target == id {
			return true
		}
		if c.Type != "nav" {
			continue
		}
		for _, it := range c.Items {
			for _, to := range pages {
				if to.ID == id && to.Platform == from.Platform && strings.EqualFold(strings.TrimSpace(to.Name), strings.TrimSpace(it)) {
					return true
				}
			}
		}
	}
	return false
}
