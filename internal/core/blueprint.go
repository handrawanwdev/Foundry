package core

import "strings"

// Blueprint is the production architecture of the product, decided during
// discovery before any code is written. Stack (frameworks) is decided
// separately and deterministically; this record holds domain boundaries,
// data, auth, storage, workers, performance and the rules coding agents
// must follow.
type Blueprint struct {
	Overview     Overview     `json:"overview"`
	Platform     PlatformArch `json:"platform"`
	Frontend     FrontendArch `json:"frontend,omitempty"`
	Backend      BackendArch  `json:"backend,omitempty"`
	Database     DatabaseArch `json:"database"`
	Cache        CacheArch    `json:"cache,omitempty"`
	Architecture ArchDecision `json:"architecture"`
	Modules      []Module     `json:"modules"`
	Storage      StorageArch  `json:"storage,omitempty"`
	Auth         AuthArch     `json:"auth,omitempty"`
	Authz        AuthzArch    `json:"authz,omitempty"`
	Realtime     RealtimeArch `json:"realtime,omitempty"`
	Events       EventArch    `json:"events,omitempty"`
	Workers      []Worker     `json:"workers,omitempty"`
	Aggregation  AggArch      `json:"aggregation,omitempty"`
	Performance  PerfArch     `json:"performance"`
	Coding       CodingRules  `json:"coding"`
	Docs         []string     `json:"docs,omitempty"`
	Workflow     []string     `json:"workflow,omitempty"`
	ChangeFlow   []string     `json:"change_flow,omitempty"`
	AIGuidelines []string     `json:"ai_guidelines,omitempty"`
}

type Overview struct {
	Goal         string   `json:"goal"`
	Users        []string `json:"users,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	CoreDomain   string   `json:"core_domain"`
}

type PlatformArch struct {
	Reason     string   `json:"reason"`
	Devices    []string `json:"devices,omitempty"`
	Offline    string   `json:"offline,omitempty"`  // none | partial | required
	Realtime   string   `json:"realtime,omitempty"` // none | needed | critical
	Deployment string   `json:"deployment,omitempty"`
}

type FrontendArch struct {
	State       string `json:"state,omitempty"`
	ServerState string `json:"server_state,omitempty"`
	Forms       string `json:"forms,omitempty"`
	Validation  string `json:"validation,omitempty"`
	Routing     string `json:"routing,omitempty"`
	API         string `json:"api,omitempty"`
	Components  string `json:"components,omitempty"`
}

type BackendArch struct {
	Pattern    string `json:"pattern,omitempty"`
	DI         string `json:"di,omitempty"`
	Validation string `json:"validation,omitempty"`
	Auth       string `json:"auth,omitempty"`
	Background string `json:"background,omitempty"`
}

type DatabaseArch struct {
	Engine       string   `json:"engine,omitempty"`
	ORM          string   `json:"orm,omitempty"`
	Migrations   string   `json:"migrations,omitempty"`
	Naming       string   `json:"naming,omitempty"`
	PK           string   `json:"pk,omitempty"`
	SoftDelete   string   `json:"soft_delete,omitempty"`
	Timestamps   string   `json:"timestamps,omitempty"`
	Audit        string   `json:"audit,omitempty"`
	Entities     []Entity `json:"entities,omitempty"`
	Transactions string   `json:"transactions,omitempty"`
	Indexing     []string `json:"indexing,omitempty"`
}

type Entity struct {
	Name    string   `json:"name"`
	Fields  []string `json:"fields,omitempty"`
	Module  string   `json:"module,omitempty"`
	Relates []string `json:"relates,omitempty"`
}

type CacheArch struct {
	Use          string   `json:"use,omitempty"`
	Store        []string `json:"store,omitempty"`
	Never        []string `json:"never,omitempty"`
	Invalidation string   `json:"invalidation,omitempty"`
}

type ArchDecision struct {
	Pattern    string   `json:"pattern"`
	Reason     string   `json:"reason"`
	Tradeoff   string   `json:"tradeoff,omitempty"`
	EvolveWhen []string `json:"evolve_when,omitempty"`
	Scale      string   `json:"scale,omitempty"`
}

// Module is one domain boundary. Coding agents put the module's code,
// data and public interface here and nowhere else.
type Module struct {
	ID             string   `json:"id"`
	Responsibility string   `json:"responsibility"`
	Data           []string `json:"data,omitempty"`
	Interface      []string `json:"interface,omitempty"`
	DependsOn      []string `json:"depends_on,omitempty"`
}

type StorageArch struct {
	Strategy   string   `json:"strategy,omitempty"` // none | local | object | hybrid
	Provider   string   `json:"provider,omitempty"`
	Upload     string   `json:"upload,omitempty"`
	Types      []string `json:"types,omitempty"`
	Security   []string `json:"security,omitempty"`
	Processing []string `json:"processing,omitempty"`
	Lifecycle  string   `json:"lifecycle,omitempty"`
	CDN        string   `json:"cdn,omitempty"`
}

type AuthArch struct {
	Method  string `json:"method,omitempty"`
	Hashing string `json:"hashing,omitempty"`
	Tokens  string `json:"tokens,omitempty"`
	Session string `json:"session,omitempty"`
	Refresh string `json:"refresh,omitempty"`
}

type AuthzArch struct {
	Model       string   `json:"model,omitempty"`
	Convention  string   `json:"convention,omitempty"`
	Permissions []string `json:"permissions,omitempty"`
}

type RealtimeArch struct {
	Needed bool     `json:"needed,omitempty"`
	Broker string   `json:"broker,omitempty"`
	Events []string `json:"events,omitempty"`
	Retry  string   `json:"retry,omitempty"`
}

// EventArch is FOUNDRY.md bagian 5: event antar-modul berformat seragam.
type EventArch struct {
	Durable   string      `json:"durable,omitempty"`   // Redis Streams + consumer group
	Ephemeral string      `json:"ephemeral,omitempty"` // Redis Pub/Sub
	Outbox    string      `json:"outbox,omitempty"`
	Envelope  string      `json:"envelope,omitempty"`
	Events    []EventSpec `json:"events,omitempty"`
}

type EventSpec struct {
	Type      string   `json:"type"`
	Producer  string   `json:"producer,omitempty"`
	Consumers []string `json:"consumers,omitempty"`
	Durable   bool     `json:"durable,omitempty"`
}

type Worker struct {
	Name    string `json:"name"`
	Trigger string `json:"trigger"` // event | cron | queue
	Job     string `json:"job"`
}

type AggArch struct {
	Needed   bool     `json:"needed,omitempty"`
	Approach string   `json:"approach,omitempty"`
	Reads    []string `json:"reads,omitempty"`
}

type PerfArch struct {
	Budget     string   `json:"budget,omitempty"` // p95 from Survey #5
	Health     string   `json:"health,omitempty"` // /health and /ready
	Pagination string   `json:"pagination,omitempty"`
	Caching    string   `json:"caching,omitempty"`
	Avoid      []string `json:"avoid,omitempty"`
	Indexing   string   `json:"indexing,omitempty"`
}

type CodingRules struct {
	Backend  []string `json:"backend,omitempty"`
	Frontend []string `json:"frontend,omitempty"`
	API      APIStd   `json:"api,omitempty"`
}

type APIStd struct {
	Style      string `json:"style,omitempty"`
	Versioning string `json:"versioning,omitempty"`
	Errors     string `json:"errors,omitempty"`
	Pagination string `json:"pagination,omitempty"`
}

// Module returns the module with this id, or nil.
func (b *Blueprint) Module(id string) *Module {
	if b == nil {
		return nil
	}
	id = strings.ToLower(strings.TrimSpace(id))
	for i := range b.Modules {
		if strings.ToLower(b.Modules[i].ID) == id {
			return &b.Modules[i]
		}
	}
	return nil
}
