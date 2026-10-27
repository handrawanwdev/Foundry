package core

import (
	"fmt"
	"slices"
	"strings"
)

// ConsistencyIssues validates the blueprint across artifacts: features
// against scope, use cases against the data model and authorization, and
// module boundaries against themselves. The single-artifact checks
// (UseCaseIssues, AcceptanceIssues, RequirementCoverageIssues,
// DesignIssues, DecisionIssues) live where each artifact is defined; this
// file holds only what needs two or more artifacts at once. Every finding
// names the location, the consequence and a suggestion; errors block
// finalize, warnings and info are shown for review.
func ConsistencyIssues(p *Project) []Issue {
	var out []Issue
	find := func(level, where, msg, impact, suggestion string) {
		out = append(out, Issue{Level: level, Where: where, Message: msg, Impact: impact, Suggestion: suggestion})
	}
	if p == nil {
		return out
	}

	modules := bpModules(p)
	moduleIDs := map[string]bool{}
	for _, m := range modules {
		moduleIDs[strings.ToLower(strings.TrimSpace(m.ID))] = true
	}

	// Domain boundaries: dependencies must resolve, not point at the module
	// itself, and not form a cycle. A public interface is a recommendation,
	// not a requirement.
	for _, m := range modules {
		where := "module:" + m.ID
		id := strings.ToLower(strings.TrimSpace(m.ID))
		if len(m.Interface) == 0 {
			find(IssueInfo, where, fmt.Sprintf("modul %s belum mendeklarasikan antarmuka publik", m.ID),
				"coding agent tidak tahu kontrak yang bisa dipakai modul lain",
				"deklarasikan antarmuka modul (fungsi, API, atau event) yang dipakai di luar modul")
		}
		seen := map[string]bool{}
		for _, d := range m.DependsOn {
			dd := strings.ToLower(strings.TrimSpace(d))
			switch {
			case dd == "" || seen[dd]:
				continue
			case dd == id:
				find(IssueError, where, fmt.Sprintf("modul %s bergantung pada dirinya sendiri", m.ID),
					"batas modul melingkar dan tidak dapat diimplementasikan",
					"hapus dependency ke diri sendiri; pindahkan perilaku itu ke dalam modul")
			case !moduleIDs[dd]:
				find(IssueError, where, fmt.Sprintf("modul %s bergantung pada modul %q yang tidak ada", m.ID, d),
					"coding agent tidak punya kontrak untuk modul itu dan dapat mengarang batas baru",
					fmt.Sprintf("tulis modul %q atau arahkan dependency ke modul yang ada", d))
			}
			seen[dd] = true
		}
	}
	if cyc := moduleCycle(modules); len(cyc) > 0 {
		find(IssueError, "architecture", "dependency modul melingkar: "+strings.Join(cyc, " → "),
			"batas domain tidak stabil; perubahan satu modul memaksa perubahan berulang",
			"putus siklus dengan memindahkan tanggung jawab atau memakai antarmuka/event satu arah")
	}

	// Entities belong to a module that exists; data without an owner has no
	// clear boundary.
	entities := entitySet(p)
	if len(moduleIDs) > 0 {
		for _, e := range bpEntities(p) {
			if m := strings.TrimSpace(e.Module); m != "" && !moduleIDs[strings.ToLower(m)] {
				find(IssueWarning, "entity:"+e.Name, fmt.Sprintf("entity %s menyebut modul %q yang tidak ada", e.Name, m),
					"data tidak punya pemilik domain, jadi batas perubahan jadi tidak jelas",
					fmt.Sprintf("buat modul %q atau pindahkan entity ke modul yang ada", m))
			}
		}
	}

	// Features trace to a requirement; an untraced feature cannot be
	// accepted or covered by the workflow.
	reqIDs := map[string]bool{}
	for _, r := range p.Requirements {
		reqIDs[r.ID] = true
	}
	for _, f := range p.Features {
		if len(f.RequirementIDs) == 0 {
			find(IssueWarning, "feature:"+f.ID, fmt.Sprintf("fitur %s %s tidak terhubung ke requirement mana pun", f.ID, f.Name),
				"fitur tidak dapat ditelusuri ke kebutuhan, jadi acceptance criteria dan command tidak menjamin kebutuhan terpenuhi",
				"hubungkan fitur ke requirement yang mendasarinya, atau hapus bila bukan fitur")
		}
		for _, r := range f.RequirementIDs {
			if !reqIDs[r] {
				find(IssueWarning, "feature:"+f.ID, fmt.Sprintf("fitur %s merujuk requirement %q yang tidak ada", f.ID, r),
					"acuan requirement kosong memutus keterlacakan",
					"perbaiki id requirement atau hapus acuannya")
			}
		}
	}

	// Use cases: the actor, permissions and data effects must match the
	// recorded roles, the authorization model and the data model.
	roles := roleSet(p)
	perms := permSet(p)
	for _, f := range p.Flows {
		if r := strings.TrimSpace(f.Role); r != "" && len(roles) > 0 && !roles[strings.ToLower(r)] {
			find(IssueWarning, f.ID, fmt.Sprintf("aktor %q belum terdaftar sebagai peran pengguna", f.Role),
				"kebutuhan akses aktor itu tidak masuk ke desain peran dan permission",
				"daftarkan aktor sebagai role, atau perbaiki nama aktornya")
		}
		for _, perm := range f.Permissions {
			if lp := strings.ToLower(strings.TrimSpace(perm)); lp != "" && len(perms) > 0 && !perms[lp] {
				find(IssueWarning, f.ID, fmt.Sprintf("permission %q dipakai use case tetapi tidak terdaftar di otorisasi", perm),
					"coding agent tidak tahu permission itu diizinkan dan dapat membuat kontrol akses yang salah",
					"tambahkan permission ke blueprint.authz atau gunakan permission yang sudah terdaftar")
			}
		}
		for _, d := range f.Data {
			if ent, _, ok := strings.Cut(d, ":"); ok {
				if name := strings.ToLower(strings.TrimSpace(ent)); name != "" && len(entities) > 0 && !entities[name] {
					find(IssueWarning, f.ID, fmt.Sprintf("efek data %q menyebut entity %q yang tidak ada di model data", d, ent),
						"use case mengubah data yang tidak dimodelkan, jadi implementasinya tidak konsisten",
						"tambahkan entity ke model data atau perbaiki efek datanya")
				}
			}
		}
	}

	return out
}

func bpModules(p *Project) []Module {
	if p.Blueprint == nil {
		return nil
	}
	return p.Blueprint.Modules
}

func bpEntities(p *Project) []Entity {
	if p.Blueprint == nil {
		return nil
	}
	return p.Blueprint.Database.Entities
}

func entitySet(p *Project) map[string]bool {
	out := map[string]bool{}
	for _, e := range bpEntities(p) {
		if n := strings.ToLower(strings.TrimSpace(e.Name)); n != "" {
			out[n] = true
		}
	}
	return out
}

func roleSet(p *Project) map[string]bool {
	out := map[string]bool{}
	if d := p.Discovery; d != nil {
		for _, r := range d.Roles {
			if n := strings.ToLower(strings.TrimSpace(r.Name)); n != "" {
				out[n] = true
			}
		}
	}
	if p.UI != nil && p.UI.Layout != nil {
		for _, u := range p.UI.Layout.Users {
			if n := strings.ToLower(strings.TrimSpace(u.Role)); n != "" {
				out[n] = true
			}
		}
	}
	return out
}

func permSet(p *Project) map[string]bool {
	out := map[string]bool{}
	if p.Blueprint != nil {
		for _, pm := range p.Blueprint.Authz.Permissions {
			if n := strings.ToLower(strings.TrimSpace(pm)); n != "" {
				out[n] = true
			}
		}
	}
	return out
}

// moduleCycle returns one dependency cycle among modules (first id repeated
// at the end), or nil.
func moduleCycle(modules []Module) []string {
	deps := map[string][]string{}
	ids := map[string]bool{}
	for _, m := range modules {
		ids[strings.ToLower(strings.TrimSpace(m.ID))] = true
	}
	for _, m := range modules {
		id := strings.ToLower(strings.TrimSpace(m.ID))
		for _, d := range m.DependsOn {
			dd := strings.ToLower(strings.TrimSpace(d))
			if dd != id && ids[dd] && !slices.Contains(deps[id], dd) {
				deps[id] = append(deps[id], dd)
			}
		}
	}
	const (
		unseen = iota
		open
		done
	)
	state := map[string]int{}
	var stack []string
	var found []string
	var visit func(id string) bool
	visit = func(id string) bool {
		state[id] = open
		stack = append(stack, id)
		for _, d := range deps[id] {
			switch state[d] {
			case open:
				i := slices.Index(stack, d)
				found = append(append([]string{}, stack[i:]...), d)
				return true
			case unseen:
				if visit(d) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = done
		return false
	}
	for _, m := range modules {
		id := strings.ToLower(strings.TrimSpace(m.ID))
		if state[id] == unseen && visit(id) {
			return found
		}
	}
	return nil
}
