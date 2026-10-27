package work

// The discovery prompt's reasoning framework. The seven domains are lenses
// the analyst thinks through, not artifacts or files: a finding may land in
// several artifacts but each is analyzed once, in the domain that owns it.
// The analyst skips the domains that do not apply to the product's kind and
// complexity.
const analysisDomains = `ANALYZE ACROSS THESE SEVEN DOMAINS. A domain is a reasoning lens, not an artifact or file. Use only the domains that apply to this product's kind and complexity.

01 Product & Business — the problem, goal and success indicators; scope: in-scope, out-of-scope and deferred; users, actors and roles; use cases and scenarios; business workflows and their transitions; business rules, validation and exceptions; features and sub-features. The output must say what is built, who uses it, which problem it solves, and the expected product behavior.

02 Domain & Data — domain modules and their responsibility boundaries; entities, attributes, relations and ownership; data validation and invariants; status, lifecycle and transitions; transaction consistency; file storage when needed; retention, deletion and migration when relevant. No entity, table or module without a traceable need.

03 Security & Access — authentication; authorization and permissions; access by user, role, resource and context; session and tokens when relevant; audit log for actions that need tracing; secrets and credential handling; threats and mitigations. Keep authentication, authorization and general application security distinct. Do not assume OAuth, OIDC, RBAC or JWT unless the need supports it.

04 Experience & Platform — for products with an interface: UI/UX needs, the pages and their purpose, navigation, layout, components, interaction, loading/empty/error/success/disabled states, form validation and feedback, responsiveness and accessibility, supported platforms and devices. For CLI, library, backend API or UI-less systems use the equivalent experience contract (CLI grammar, API ergonomics, library API), never forced screens. Foundry writes the screen-level specification in a separate design stage; here record only the experience needs and platform targets.

05 Architecture & Infrastructure — project structure and component responsibilities; stack and why; API and integration contracts; external systems and dependencies; workers, schedulers, queues and background processing when needed; database, cache and file storage when needed; performance and capacity; deployment and environment config; backup, recovery, logging, metrics and tracing as operationally needed. Choose the simplest solution that meets the need; no Redis, workers or microservices just because they are common.

06 Quality & Delivery — verifiable acceptance criteria per feature or use case; the test strategy that applies; measurable non-functional targets when needed; implementation dependencies; risks and mitigations; unconfirmed assumptions; open decisions; implementation order. Every important feature needs a clear way to verify it. Do not use "production-ready" or "high-performance" as a substitute for a measurable criterion.

07 Planning Validation — the adaptive survey; detecting incomplete needs; separating fact, assumption, decision and open question; detecting conflicts; checking cross-artifact consistency; tracing requirements to acceptance criteria and workflow. This domain checks the other six and never duplicates their content.
`

// analysisRules are the reasoning constraints the analyst must follow.
const analysisRules = `REASONING RULES:
1. Understand the user's goal and problem before deciding any technical solution.
2. Separate what the user gave (fact), what the repository inspection found (fact), what you assume (assumption), what you recommend (recommendation), and what is still undecided (open question).
3. Do not invent requirements that were not given.
4. Derive use cases, features, business rules, the data model and acceptance criteria so they stay consistent with each other.
5. Choose technology and architecture from the real needs, not from a default preference.
6. Use the simplest solution that meets the needs.
7. Analyze only what is relevant to this product's kind and complexity; skip the rest.
8. Keep the user's explicit requirements; a technical recommendation must never silently replace one.
9. When missing information would change an important decision, name the uncertainty and raise it through the adaptive survey instead of guessing.
10. Never state the blueprint is valid or ready to run while a critical conflict is unresolved.
`

// adaptiveSurveyRules is the section-4 guidance: the survey is adaptive, not
// a checklist that asks every category in order.
const adaptiveSurveyRules = `ADAPTIVE SURVEY:
- Ask only for information that is missing AND would change the design. Never re-ask what the story, the repository or the survey answers already establish.
- Put questions to the user in plain business language; keep technical questions for when the answer really changes an architecture or implementation decision.
- A decision that can be deferred may be left unknown: record it as a decision kind question, do not invent an answer.
- Distinguish what must be confirmed from what is safe to decide automatically; a safe default may be applied and stated, never presented as the user's answer.
- Never turn your own assumption into a survey answer, and never present an assumption as the user's decision.
`

// traceabilityRules ties every important requirement to the artifacts that
// realize and verify it, without forcing relations that do not exist.
const traceabilityRules = `REQUIREMENT TRACEABILITY:
- every requirement is covered by at least one feature; every feature has 2-6 concrete sub-features.
- connect each requirement to the use case(s) and feature(s) it serves, and (when relevant) to its business rules, the data model, the UI/API, and its acceptance criteria. Do not force a relation that does not exist to fill a checklist.
- keep the ids you assign consistent across the whole answer (requirements, features, flows, acceptance, decisions), so the links survive landing.
`

// consistencyRules is the section-7 cross-artifact check the model runs
// before answering; deterministic checks are run by Foundry after landing.
const consistencyRules = `CONSISTENCY AND CONFLICTS:
- before answering, check conflicts between: goal/scope and features; workflows and business rules; business rules and entities/transactions; roles/permissions and allowed operations; UI needs and system capability; integrations and API contracts; technology and deployment limits; requirements, acceptance criteria and workflow.
- for every conflict return a decision kind conflict naming both sides, the impact, the severity, and the resolution options. Separate what blocks finalizing (critical) from warnings and non-blocking recommendations. A conflict that needs the user is a decision kind question with critical true, never resolved silently.
`

// outputValidationRules is the self-check before the answer is handed to the
// next stage. It complements, never replaces, Foundry's deterministic checks.
const outputValidationRules = `OUTPUT VALIDATION (before you reply, check): the main requirements are present; no critical conflict is left unresolved; artifacts are consistent with each other; acceptance criteria are verifiable; technology decisions have a reasonable basis; the output matches the JSON schema below. Foundry runs the deterministic checks (id shapes, references, cycles, coverage) after you reply, so keep ids and references exact.
`
