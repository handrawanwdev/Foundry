package core

import (
	"bytes"
	"encoding/json"
	"math"
	"slices"
	"strconv"
	"strings"
)

// coerceText accepts a JSON string or an array of strings.
func coerceText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		return strings.Join(ss, "; ")
	}
	return ""
}

func coerceFacts(raw json.RawMessage) []TechFact {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	var facts []TechFact
	if json.Unmarshal(raw, &facts) == nil {
		return facts
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		out := make([]TechFact, 0, len(ss))
		for _, s := range ss {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, TechFact{Statement: s})
		}
		return out
	}
	var one TechFact
	if json.Unmarshal(raw, &one) == nil && one.Statement != "" {
		return []TechFact{one}
	}
	return nil
}

func coerceParts(raw json.RawMessage) []StructurePart {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	var parts []StructurePart
	if json.Unmarshal(raw, &parts) == nil {
		return parts
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		out := make([]StructurePart, 0, len(ss))
		for _, s := range ss {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, StructurePart{Name: s})
		}
		return out
	}
	return nil
}

func (c *TechContext) UnmarshalJSON(b []byte) error {
	var aux struct {
		Summary     string          `json:"summary"`
		Facts       json.RawMessage `json:"facts"`
		Unknowns    []string        `json:"unknowns"`
		Assumptions []string        `json:"assumptions"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	c.Summary = aux.Summary
	c.Facts = coerceFacts(aux.Facts)
	c.Unknowns = aux.Unknowns
	c.Assumptions = aux.Assumptions
	return nil
}

func (ir *StructureIR) UnmarshalJSON(b []byte) error {
	var aux struct {
		Type     string          `json:"type"`
		Apps     json.RawMessage `json:"apps"`
		Services json.RawMessage `json:"services"`
		Workers  json.RawMessage `json:"workers"`
		Packages json.RawMessage `json:"packages"`
		Docs     string          `json:"docs"`
		Rules    []string        `json:"rules"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	ir.Type = aux.Type
	ir.Apps = coerceParts(aux.Apps)
	ir.Services = coerceParts(aux.Services)
	ir.Workers = coerceParts(aux.Workers)
	ir.Packages = coerceParts(aux.Packages)
	ir.Docs = aux.Docs
	ir.Rules = aux.Rules
	return nil
}

func coerceByString[T any](raw json.RawMessage, fromString func(string) T) []T {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	var objs []T
	if json.Unmarshal(raw, &objs) == nil {
		return objs
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		out := make([]T, 0, len(ss))
		for _, s := range ss {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			out = append(out, fromString(s))
		}
		return out
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
			return []T{fromString(s)}
		}
	}
	var one T
	if json.Unmarshal(raw, &one) == nil {
		return []T{one}
	}
	return nil
}

func (e *TechEvaluation) UnmarshalJSON(b []byte) error {
	var aux struct {
		Summary  string          `json:"summary"`
		Criteria json.RawMessage `json:"criteria"`
		Findings json.RawMessage `json:"findings"`
		Score    int             `json:"score"`
		Outdated bool            `json:"outdated"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	crit, err := coerceCriteria[TechCriterion](aux.Criteria, func(c *TechCriterion, name string) {
		if c.Name == "" {
			c.Name = name
		}
	}, func(name string, score int) TechCriterion { return TechCriterion{Name: name, Score: score} })
	if err != nil {
		return err
	}
	e.Summary = aux.Summary
	e.Criteria = crit
	e.Findings = coerceByString(aux.Findings, func(s string) TechFinding { return TechFinding{Message: s} })
	e.Score = aux.Score
	e.Outdated = aux.Outdated
	return nil
}

func (d *RepoDependency) UnmarshalJSON(b []byte) error {
	var aux struct {
		Direction     string          `json:"direction"`
		Allowed       json.RawMessage `json:"allowed"`
		Forbidden     json.RawMessage `json:"forbidden"`
		Communication json.RawMessage `json:"communication"`
		Interfaces    json.RawMessage `json:"interfaces"`
		SharedRules   []string        `json:"shared_rules"`
		CycleGuard    string          `json:"cycle_guard"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	from := func(s string) RepoDepRule { return RepoDepRule{Reason: s} }
	d.Direction = aux.Direction
	d.Allowed = coerceByString(aux.Allowed, from)
	d.Forbidden = coerceByString(aux.Forbidden, from)
	d.Communication = coerceByString(aux.Communication, from)
	d.Interfaces = coerceByString(aux.Interfaces, func(s string) RepoInterface { return RepoInterface{Reason: s} })
	d.SharedRules = aux.SharedRules
	d.CycleGuard = aux.CycleGuard
	return nil
}

func (e *RepoEvaluation) UnmarshalJSON(b []byte) error {
	var aux struct {
		Summary  string          `json:"summary"`
		Criteria json.RawMessage `json:"criteria"`
		Findings json.RawMessage `json:"findings"`
		Score    int             `json:"score"`
		Outdated bool            `json:"outdated"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	crit, err := coerceCriteria[RepoCriterion](aux.Criteria, func(c *RepoCriterion, name string) {
		if c.Name == "" {
			c.Name = name
		}
	}, func(name string, score int) RepoCriterion { return RepoCriterion{Name: name, Score: score} })
	if err != nil {
		return err
	}
	e.Summary = aux.Summary
	e.Criteria = crit
	e.Findings = coerceByString(aux.Findings, func(s string) RepoFinding { return RepoFinding{Message: s} })
	e.Score = aux.Score
	e.Outdated = aux.Outdated
	return nil
}

func (n *RepoNode) UnmarshalJSON(b []byte) error {
	var aux struct {
		Path           string          `json:"path"`
		Name           string          `json:"name"`
		Type           string          `json:"type"`
		Role           string          `json:"role"`
		Responsibility string          `json:"responsibility"`
		Owner          string          `json:"owner"`
		DependsOn      []string        `json:"depends_on"`
		Reason         string          `json:"reason"`
		Part           string          `json:"part"`
		SharedBy       []string        `json:"shared_by"`
		Allowed        []string        `json:"allowed"`
		Forbidden      []string        `json:"forbidden"`
		Relations      json.RawMessage `json:"relations"`
		Boundary       string          `json:"boundary"`
		Implied        bool            `json:"implied"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	n.Path = aux.Path
	n.Name = aux.Name
	n.Type = aux.Type
	n.Role = aux.Role
	n.Responsibility = aux.Responsibility
	n.Owner = aux.Owner
	n.DependsOn = aux.DependsOn
	n.Reason = aux.Reason
	n.Part = aux.Part
	n.SharedBy = aux.SharedBy
	n.Allowed = aux.Allowed
	n.Forbidden = aux.Forbidden
	n.Relations = coerceText(aux.Relations)
	n.Boundary = aux.Boundary
	n.Implied = aux.Implied
	return nil
}

// coerceCriteria reads evaluation criteria as a list, or as an object keyed
// by criterion name whose values are the criterion or only its score.
func coerceCriteria[T any](raw json.RawMessage, named func(*T, string), scored func(string, int) T) ([]T, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, nil
	}
	if raw[0] != '{' {
		var list []T
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, err
		}
		return list, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	slices.Sort(names)
	out := make([]T, 0, len(m))
	for _, name := range names {
		v := bytes.TrimSpace(m[name])
		if len(v) > 0 && v[0] == '{' {
			var c T
			if err := json.Unmarshal(v, &c); err != nil {
				return nil, err
			}
			named(&c, name)
			out = append(out, c)
			continue
		}
		score, ok := coerceInt(v)
		if !ok {
			return nil, &json.UnmarshalTypeError{Value: string(v), Field: "criteria." + name}
		}
		out = append(out, scored(name, score))
	}
	return out, nil
}

// coerceInt reads a number written as an integer, a fraction (rounded) or
// a string.
func coerceInt(raw json.RawMessage) (int, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, true
	}
	var s string
	if raw[0] == '"' {
		if json.Unmarshal(raw, &s) != nil {
			return 0, false
		}
	} else {
		s = string(raw)
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return int(math.Round(f)), true
}

// coerceList accepts a list of strings or one string.
func coerceList(raw json.RawMessage) []string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil
	}
	var ss []string
	if json.Unmarshal(raw, &ss) == nil {
		return ss
	}
	var s string
	if json.Unmarshal(raw, &s) == nil && strings.TrimSpace(s) != "" {
		return []string{s}
	}
	return nil
}

func firstText(texts ...string) string {
	for _, t := range texts {
		if t = strings.TrimSpace(t); t != "" {
			return t
		}
	}
	return ""
}

// criterionJSON is a criterion as models write it: the name under "name"
// or a synonym, the score as a number or string, text as a string or list.
type criterionJSON struct {
	Name      string          `json:"name"`
	Criterion string          `json:"criterion"`
	ID        string          `json:"id"`
	Key       string          `json:"key"`
	Score     json.RawMessage `json:"score"`
	Reason    json.RawMessage `json:"reason"`
	Evidence  json.RawMessage `json:"evidence"`
}

func (c *criterionJSON) decode(b []byte) (name string, score int, err error) {
	if err := json.Unmarshal(b, c); err != nil {
		return "", 0, err
	}
	score, ok := coerceInt(c.Score)
	if !ok {
		return "", 0, &json.UnmarshalTypeError{Value: string(c.Score), Field: "score"}
	}
	return firstText(c.Name, c.Criterion, c.ID, c.Key), score, nil
}

func (k *TechCriterion) UnmarshalJSON(b []byte) error {
	var aux criterionJSON
	name, score, err := aux.decode(b)
	if err != nil {
		return err
	}
	*k = TechCriterion{Name: name, Score: score, Reason: coerceText(aux.Reason), Evidence: coerceList(aux.Evidence)}
	return nil
}

func (k *RepoCriterion) UnmarshalJSON(b []byte) error {
	var aux criterionJSON
	name, score, err := aux.decode(b)
	if err != nil {
		return err
	}
	*k = RepoCriterion{Name: name, Score: score, Reason: coerceText(aux.Reason)}
	return nil
}

// checkJSON is a consistency check whose note may come as "detail" or
// "reason".
type checkJSON struct {
	Area   string          `json:"area"`
	Status string          `json:"status"`
	Note   json.RawMessage `json:"note"`
	Detail json.RawMessage `json:"detail"`
	Reason json.RawMessage `json:"reason"`
	Source string          `json:"source"`
}

func (c checkJSON) note() string {
	return firstText(coerceText(c.Note), coerceText(c.Detail), coerceText(c.Reason))
}

func (k *TechCheck) UnmarshalJSON(b []byte) error {
	var aux checkJSON
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*k = TechCheck{Area: aux.Area, Status: aux.Status, Note: aux.note(), Source: aux.Source}
	return nil
}

func (k *RepoCheck) UnmarshalJSON(b []byte) error {
	var aux checkJSON
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*k = RepoCheck{Area: aux.Area, Status: aux.Status, Note: aux.note(), Source: aux.Source}
	return nil
}
