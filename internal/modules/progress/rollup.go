package progress

import (
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/handrawanwdev/foundry/internal/core"
)

// Roll-up statuses shown for a project, module, feature, subfeature or
// workflow. They are derived: no report means not_started, never done.
const (
	RollupEmpty      = "no_commands"
	RollupNotStarted = "not_started"
	RollupRunning    = "in_progress"
	RollupBlocked    = "blocked"
	RollupUnknown    = "unknown"
	RollupDone       = "done"
)

// Rollup is the weighted execution state of a group of commands. Weight
// comes from core.CommandWeight, so a large command contributes more than a
// small one. Percent counts verified work only.
type Rollup struct {
	Percent     int        `json:"percent"`
	Weight      int        `json:"weight"`
	DoneWeight  int        `json:"done_weight"`
	Total       int        `json:"total"`
	Completed   int        `json:"completed"`   // verified
	Claimed     int        `json:"claimed"`     // agent says done, not verified
	InProgress  int        `json:"in_progress"` // started or waiting review
	NotStarted  int        `json:"not_started"` // unconfirmed or waiting for deps
	Failed      int        `json:"failed"`
	Blocked     int        `json:"blocked"`
	Skipped     int        `json:"skipped"`
	Unknown     int        `json:"unknown"`  // stale or disconnected
	Revision    int        `json:"revision"` // has to be redone
	Status      string     `json:"status"`
	LastStatus  string     `json:"last_status,omitempty"`
	LastCommand string     `json:"last_command,omitempty"`
	LastAt      *time.Time `json:"last_at,omitempty"`
}

// Dependency is one edge of the command graph with its current state.
type Dependency struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Satisfied bool   `json:"satisfied"`
	Status    string `json:"status"`
}

// WorkflowProgress is the execution state of the one command workflow.
type WorkflowProgress struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DesignStatus string `json:"design_status"` // draft | finalized
	Rollup
	Dependencies []Dependency `json:"dependencies"`
}

// ModuleProgress groups features from Feature Discovery. Its roll-up counts
// the union of their commands once.
type ModuleProgress struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	FeatureIDs []string `json:"feature_ids"`
	Rollup
}

// FeatureProgress is one feature with its subfeatures and linked commands.
type FeatureProgress struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ModuleID     string   `json:"module_id"`
	ModuleName   string   `json:"module_name"`
	FeatureMapID string   `json:"feature_map_id,omitempty"`
	Commands     []string `json:"commands"`
	Rollup
	SubFeatures []SubFeatureProgress `json:"sub_features"`
}

// SubFeatureProgress is one subfeature and the commands that name it.
type SubFeatureProgress struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	FeatureID    string   `json:"feature_id"`
	FeatureMapID string   `json:"feature_map_id,omitempty"`
	Commands     []string `json:"commands"`
	Rollup
}

// rollupCommands builds the weighted state of the given commands. Duplicate
// ids are ignored, so the same command never counts twice in one roll-up.
func rollupCommands(cmds []CommandProgress, only map[string]bool) Rollup {
	seen := map[string]bool{}
	var chosen []CommandProgress
	for _, c := range cmds {
		if seen[c.ID] || (only != nil && !only[c.ID]) {
			continue
		}
		seen[c.ID] = true
		chosen = append(chosen, c)
	}
	r := Rollup{Total: len(chosen), Status: RollupEmpty}
	for _, c := range chosen {
		w := c.Weight
		if w < 1 {
			w = 1
		}
		r.Weight += w
		switch c.Status {
		case core.RunVerified:
			r.Completed++
			r.DoneWeight += w
		case core.RunDone:
			r.Claimed++
		case core.RunStarted, core.RunReview:
			r.InProgress++
		case core.RunUnconfirmed, core.RunWaiting:
			r.NotStarted++
		case core.RunFailed:
			r.Failed++
		case core.RunBlocked:
			r.Blocked++
		case core.RunSkipped:
			r.Skipped++
		case core.RunStale, core.RunUnknown:
			r.Unknown++
		case core.RunRevision:
			r.Revision++
		default:
			r.NotStarted++
		}
		if c.UpdatedAt != nil && (r.LastAt == nil || c.UpdatedAt.After(*r.LastAt)) {
			t := c.UpdatedAt.UTC()
			r.LastAt, r.LastCommand, r.LastStatus = &t, c.ID, c.Status
		}
	}
	r.Percent = percent(r.DoneWeight, r.Weight)
	r.Status = rollupStatus(r)
	return r
}

func rollupStatus(r Rollup) string {
	switch {
	case r.Total == 0:
		return RollupEmpty
	case r.Completed == r.Total:
		return RollupDone
	case r.Failed+r.Blocked > 0:
		return RollupBlocked
	case r.Completed+r.Claimed+r.InProgress+r.Revision > 0:
		return RollupRunning
	case r.Unknown >= r.Total:
		return RollupUnknown
	default:
		return RollupNotStarted
	}
}

func percent(done, total int) int {
	if total <= 0 || done <= 0 {
		return 0
	}
	if done >= total {
		return 100
	}
	p := int(math.Round(float64(done) * 100 / float64(total)))
	if p < 1 {
		p = 1
	}
	if p >= 100 {
		p = 99
	}
	return p
}

func workflowProgress(p *core.Project, cmds []CommandProgress, overall Rollup) WorkflowProgress {
	w := WorkflowProgress{ID: "WF-001", Name: "Command workflow", Dependencies: []Dependency{}, Rollup: overall}
	if p == nil || p.Workflow == nil {
		return w
	}
	if p.Workflow.ID != "" {
		w.ID = p.Workflow.ID
	}
	w.DesignStatus = p.Workflow.Status
	status := map[string]string{}
	for _, c := range cmds {
		status[c.ID] = c.Status
	}
	seen := map[string]bool{}
	for _, c := range p.Workflow.Commands {
		for _, d := range c.DependsOn {
			key := c.ID + "\x00" + d
			if seen[key] {
				continue
			}
			seen[key] = true
			st := status[d]
			if st == "" {
				st = core.RunUnconfirmed
			}
			w.Dependencies = append(w.Dependencies, Dependency{From: c.ID, To: d, Satisfied: st == core.RunVerified, Status: st})
		}
	}
	sort.SliceStable(w.Dependencies, func(i, j int) bool {
		if w.Dependencies[i].From != w.Dependencies[j].From {
			return w.Dependencies[i].From < w.Dependencies[j].From
		}
		return w.Dependencies[i].To < w.Dependencies[j].To
	})
	return w
}

// featureRollups builds the Feature Discovery tree with execution progress.
// A command linked to a feature counts once in that feature and once in the
// union of its module; it is never counted twice inside the same roll-up.
func featureRollups(p *core.Project, cmds []CommandProgress) ([]ModuleProgress, []FeatureProgress) {
	if p == nil {
		return []ModuleProgress{}, []FeatureProgress{}
	}
	byID := map[string]CommandProgress{}
	for _, c := range cmds {
		byID[c.ID] = c
	}
	featureOf := map[string][]string{}
	subOf := map[string][]string{}
	for _, c := range cmds {
		featureOf[c.ID] = c.FeatureIDs
		subOf[c.ID] = c.SubFeatureIDs
	}
	cmdFeature, cmdSub := commandsByValue(featureOf), commandsByValue(subOf)

	featByID := map[string]*core.Feature{}
	featBySource := map[string]*core.Feature{}
	featByName := map[string]*core.Feature{}
	subBySource := map[string]*core.SubFeature{}
	subByName := map[string]*core.SubFeature{}
	for i := range p.Features {
		f := &p.Features[i]
		featByID[normName(f.ID)] = f
		if f.SourceID != "" {
			featBySource[f.SourceID] = f
		}
		if _, ok := featByName[normName(f.Name)]; !ok {
			featByName[normName(f.Name)] = f
		}
		for j := range f.SubFeatures {
			s := &f.SubFeatures[j]
			if s.SourceID != "" {
				subBySource[s.SourceID] = s
			}
			if _, ok := subByName[normName(s.Name)]; !ok {
				subByName[normName(s.Name)] = s
			}
		}
	}

	used := map[string]bool{}
	var modules []ModuleProgress
	var features []FeatureProgress
	addFeature := func(fp FeatureProgress, moduleID, moduleName string) {
		fp.ModuleID, fp.ModuleName = moduleID, moduleName
		features = append(features, fp)
	}

	if p.FeatureMap != nil && len(p.FeatureMap.Modules) > 0 {
		for _, mod := range p.FeatureMap.Modules {
			mp := ModuleProgress{ID: mod.ID, Name: mod.Name, FeatureIDs: []string{}}
			var modCmds []string
			for i := range mod.Features {
				node := &mod.Features[i]
				f := matchFeature(node, featByID, featBySource, featByName)
				if f != nil {
					used[f.ID] = true
				}
				fp, ids := buildFeatureProgress(f, node, cmdFeature, cmdSub, byID, subBySource, subByName)
				mp.FeatureIDs = append(mp.FeatureIDs, fp.ID)
				modCmds = append(modCmds, ids...)
				addFeature(fp, mod.ID, mod.Name)
			}
			mp.Rollup = rollupCommands(cmds, idSet(modCmds))
			modules = append(modules, mp)
		}
	}

	// Features the map does not cover still have work, so they never vanish
	// from progress. Without a map they are the only source of the tree.
	var rest []core.Feature
	for _, f := range p.Features {
		if !used[f.ID] {
			rest = append(rest, f)
		}
	}
	if len(rest) > 0 || (len(modules) == 0 && len(p.Features) > 0) {
		modID, modName := "features", "Fitur"
		if p.FeatureMap != nil && len(p.FeatureMap.Modules) > 0 {
			modID, modName = "other", "Fitur lain"
		}
		mp := ModuleProgress{ID: modID, Name: modName, FeatureIDs: []string{}}
		var modCmds []string
		for i := range rest {
			f := &rest[i]
			fp, ids := buildFeatureProgress(f, nil, cmdFeature, cmdSub, byID, subBySource, subByName)
			mp.FeatureIDs = append(mp.FeatureIDs, fp.ID)
			modCmds = append(modCmds, ids...)
			addFeature(fp, modID, modName)
		}
		mp.Rollup = rollupCommands(cmds, idSet(modCmds))
		modules = append(modules, mp)
	}
	if modules == nil {
		modules = []ModuleProgress{}
	}
	if features == nil {
		features = []FeatureProgress{}
	}
	for i := range features {
		if features[i].SubFeatures == nil {
			features[i].SubFeatures = []SubFeatureProgress{}
		}
		if features[i].Commands == nil {
			features[i].Commands = []string{}
		}
	}
	return modules, features
}

func buildFeatureProgress(f *core.Feature, node *core.FeatureNode, cmdFeature, cmdSub map[string][]string,
	byID map[string]CommandProgress, subBySource, subByName map[string]*core.SubFeature) (FeatureProgress, []string) {
	fp := FeatureProgress{Commands: []string{}, SubFeatures: []SubFeatureProgress{}}
	if f != nil {
		fp.ID, fp.Name = f.ID, f.Name
	} else if node != nil {
		fp.ID, fp.Name = node.ID, node.Name
	}
	if node != nil {
		fp.FeatureMapID = node.ID
	}
	seenSub := map[string]bool{}
	featureCmds := slices.Clone(cmdFeature[fp.ID])
	if f != nil {
		for i := range f.SubFeatures {
			s := &f.SubFeatures[i]
			sp := SubFeatureProgress{ID: s.ID, Name: s.Name, FeatureID: f.ID, Commands: commandsFor(cmdSub[s.ID], byID)}
			if s.SourceID != "" && (node == nil || findNodeID(node.Children, s.SourceID) != nil) {
				sp.FeatureMapID = s.SourceID
			} else if node != nil {
				if child := matchChild(node, s, subBySource, subByName); child != nil {
					sp.FeatureMapID = child.ID
				}
			}
			if sp.FeatureMapID != "" {
				seenSub[sp.FeatureMapID] = true
			}
			sp.Rollup = rollupIDs(sp.Commands, byID)
			fp.SubFeatures = append(fp.SubFeatures, sp)
			featureCmds = append(featureCmds, sp.Commands...)
		}
	}
	// Feature-map children without a matching blueprint subfeature still
	// appear; their status is simply unknown rather than assumed.
	if node != nil {
		for i := range node.Children {
			child := &node.Children[i]
			if seenSub[child.ID] {
				continue
			}
			sp := SubFeatureProgress{ID: child.ID, Name: child.Name, FeatureID: fp.ID, FeatureMapID: child.ID,
				Commands: []string{}, Rollup: Rollup{Status: RollupEmpty}}
			fp.SubFeatures = append(fp.SubFeatures, sp)
		}
	}
	fp.Commands = commandsFor(featureCmds, byID)
	fp.Rollup = rollupIDs(fp.Commands, byID)
	return fp, fp.Commands
}

func matchFeature(node *core.FeatureNode, byID, bySource, byName map[string]*core.Feature) *core.Feature {
	if node == nil {
		return nil
	}
	if f := byID[normName(node.FeatureID)]; f != nil {
		return f
	}
	if f := bySource[node.ID]; f != nil {
		return f
	}
	return byName[normName(node.Name)]
}

func findNodeID(nodes []core.FeatureNode, id string) *core.FeatureNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

func matchChild(node *core.FeatureNode, s *core.SubFeature, bySource map[string]*core.SubFeature, byName map[string]*core.SubFeature) *core.FeatureNode {
	if node == nil {
		return nil
	}
	for i := range node.Children {
		child := &node.Children[i]
		if strings.EqualFold(strings.TrimSpace(child.SubFeatureID), strings.TrimSpace(s.ID)) ||
			child.ID == s.SourceID || bySource[child.ID] == s || byName[normName(child.Name)] == s {
			return child
		}
	}
	return nil
}

// rollupIDs rolls up the given unique command ids in id order, so a roll-up
// never depends on map order. Ids without a command are ignored.
func rollupIDs(ids []string, byID map[string]CommandProgress) Rollup {
	chosen := make([]CommandProgress, 0, len(ids))
	for _, id := range ids {
		if c, ok := byID[id]; ok {
			chosen = append(chosen, c)
		}
	}
	sort.Slice(chosen, func(i, j int) bool { return chosen[i].ID < chosen[j].ID })
	return rollupCommands(chosen, nil)
}

func commandsFor(ids []string, byID map[string]CommandProgress) []string {
	seen := map[string]bool{}
	var out []string
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return commandWaveLess(out[i], out[j], byID) })
	if out == nil {
		out = []string{}
	}
	return out
}

func commandWaveLess(a, b string, byID map[string]CommandProgress) bool {
	ca, oka := byID[a]
	cb, okb := byID[b]
	if oka && okb && ca.Wave != cb.Wave {
		return ca.Wave < cb.Wave
	}
	return a < b
}

func idSet(ids []string) map[string]bool {
	if ids == nil {
		return nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

func normName(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// commandsByValue inverts command id -> values into value -> sorted command ids.
func commandsByValue(byCommand map[string][]string) map[string][]string {
	out := map[string][]string{}
	for cmd, ids := range byCommand {
		for i, id := range ids {
			if !slices.Contains(ids[:i], id) {
				out[id] = append(out[id], cmd)
			}
		}
	}
	for _, cmds := range out {
		sort.Strings(cmds)
	}
	return out
}
