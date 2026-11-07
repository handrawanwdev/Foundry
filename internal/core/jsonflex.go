package core

import (
	"bytes"
	"encoding/json"
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
		Criteria []TechCriterion `json:"criteria"`
		Findings json.RawMessage `json:"findings"`
		Score    int             `json:"score"`
		Outdated bool            `json:"outdated"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	e.Summary = aux.Summary
	e.Criteria = aux.Criteria
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
		Criteria []RepoCriterion `json:"criteria"`
		Findings json.RawMessage `json:"findings"`
		Score    int             `json:"score"`
		Outdated bool            `json:"outdated"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	e.Summary = aux.Summary
	e.Criteria = aux.Criteria
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
