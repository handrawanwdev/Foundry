package core

// DesignTemplate is a proven UX pattern the designer starts from: how a
// kind of product screen is structured (layout pattern, information
// hierarchy, component composition, responsive strategy) and what a good
// one does and avoids. It is design reasoning, never markup or a fixed
// screen: the designer adapts it to the product's features and composes
// the Design IR itself. The theme decides the look; the template decides
// the structure.
type DesignTemplate struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Category        string             `json:"category"`
	Intent          string             `json:"intent"`
	Characteristics []string           `json:"characteristics"`
	SuitableFor     TemplateFit        `json:"suitable_for"`
	Layout          TemplateLayout     `json:"layout_pattern"`
	Hierarchy       []string           `json:"information_hierarchy"`
	Composition     []TemplateRegion   `json:"component_composition"`
	Responsive      TemplateResponsive `json:"responsive_strategy"`
	Quality         TemplateQuality    `json:"quality_rules"`
	Examples        []TemplateExample  `json:"examples,omitempty"`
}

// TemplateFit is what template selection matches: the domains it is made
// for (the first is its home domain), the user roles, the primary tasks,
// how often and how dense the work is, the page kinds it structures and
// the words in a requirement that point to it.
type TemplateFit struct {
	Domains   []string `json:"domains,omitempty"`
	Roles     []string `json:"roles,omitempty"`
	Tasks     []string `json:"tasks,omitempty"`
	Frequency string   `json:"frequency,omitempty"` // low | medium | high
	Density   string   `json:"density,omitempty"`   // low | medium | high
	PageKinds []string `json:"page_kinds"`
	Keywords  []string `json:"keywords,omitempty"`
}

// TemplateLayout is the structural pattern: the layout strategies the
// template's pages use (the first is the main one), the navigation and
// density it needs and the structure in one line.
type TemplateLayout struct {
	Strategies []string `json:"strategies"`
	Navigation string   `json:"navigation"` // side | rail | top | bottom | none
	Density    string   `json:"density"`    // compact | comfortable | spacious
	Structure  string   `json:"structure"`
}

// TemplateRegion is one part of the composition. Role: primary |
// secondary | supporting | action | navigation. Area places it in the
// preview: top | side | main | aside.
type TemplateRegion struct {
	Region  string `json:"region"`
	Role    string `json:"role"`
	Type    string `json:"type"` // a component type
	Area    string `json:"area"`
	Purpose string `json:"purpose"`
}

type TemplateResponsive struct {
	Strategy string `json:"strategy"` // a responsive strategy
	Desktop  string `json:"desktop"`
	Mobile   string `json:"mobile"`
}

// TemplateQuality: what a page made from the template must do, what it
// avoids (codes from TemplateAvoid, checked on the IR) and the checks in
// words for the critic.
type TemplateQuality struct {
	Expected []string `json:"expected_behavior"`
	Avoid    []string `json:"avoid"`
	Checks   []string `json:"quality_checks,omitempty"`
}

// TemplateExample is the template adapted to one domain: what each region
// holds there.
type TemplateExample struct {
	Domain  string              `json:"domain"`
	Product string              `json:"product"`
	Regions map[string]string   `json:"regions"`
	Items   map[string][]string `json:"items,omitempty"` // sample rows, columns or nav items per region
}

// TemplateAvoid are the anti-patterns a template can rule out.
var TemplateAvoid = []string{"hero_section", "marketing_section", "large_cards", "excessive_stats", "decorative_chart", "competing_actions"}

// TemplateChoice is the template a design follows and why. Source: ai
// (the designer chose it), auto (Foundry's recommendation, used when the
// designer chose none or an unknown one).
type TemplateChoice struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Secondary string   `json:"secondary,omitempty"`
	Source    string   `json:"source"`
	Reasons   []string `json:"reasons,omitempty"`
}
