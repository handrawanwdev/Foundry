# Rencana Implementasi: Autonomous UI/UX Intelligence & Continuous Improvement

## Status Saat Ini
- ✅ Two-phase batching untuk workflow generation (context limit handling)
- ⚠️ Visual workflow system (perlu perbaikan)
- ❌ Design intelligence context-aware
- ❌ Autonomous UI improvement workflow
- ❌ Visual quality evaluation
- ❌ Autonomous correction loop
- ❌ Continuous design learning
- ❌ Integration dengan quality gates

## Wave 2: Design Intelligence & Visual Workflow

### 2.1 Context-Aware Design Intelligence
**File baru**: `internal/modules/work/designcontext.go`

Fungsi:
- `DetectProductType(p *core.Project) ProductType` - deteksi jenis produk dari story, requirements, dan platform
- `ContextualDesignRules(productType ProductType) string` - generate design rules berdasarkan konteks produk
- `DesignIntelligencePrompt(p *core.Project) string` - prompt untuk AI designer dengan konteks lengkap

Product types:
- Dashboard (trading, admin, analytics)
- SaaS Application
- Mobile App
- E-commerce
- Content Platform
- Communication Platform
- Workflow/Process Management
- Generic

Setiap product type punya:
- Pattern recommendations (kapan pakai table vs cards vs list)
- Information density guidance
- Interaction pattern suggestions
- Platform-specific considerations

### 2.2 Autonomous Visual Verification Workflow
**File**: `internal/modules/work/visualworkflow.go` (sudah ada, perlu perbaikan)

**Masalah saat ini**:
- Commands tidak menyebutkan page IDs di instructions
- Commands hanya reference salah satu artifact (design-rules ATAU layout-spec, bukan keduanya)
- Automatic append menyebabkan test failures

**Solusi**:
1. Generate visual verification commands secara eksplisit (tidak otomatis append)
2. Setiap command harus:
   - Menyebutkan specific page IDs dari layout-spec
   - Reference KEDUA artifacts (design-rules DAN layout-spec)
   - Instructions yang spesifik dan actionable

Commands yang dibutuhkan:
```
CMD-START-APP: Start application dan verify readiness
CMD-ENUMERATE-ROUTES: Discover dan validate semua routes
CMD-CAPTURE-SCREENSHOTS: Capture screenshots di multiple viewports
CMD-SUBMIT-VISUAL-REVIEWS: Submit reviews ke Foundry
CMD-CREATE-VISUAL-FIXES: Generate correction commands dari findings
CMD-RE-VISUAL-VERIFY: Re-verify setelah fixes
```

**Dependencies**:
```
CMD-START-APP → (semua implementation commands)
CMD-ENUMERATE-ROUTES → CMD-START-APP
CMD-CAPTURE-SCREENSHOTS → CMD-ENUMERATE-ROUTES
CMD-SUBMIT-VISUAL-REVIEWS → CMD-CAPTURE-SCREENSHOTS
CMD-CREATE-VISUAL-FIXES → CMD-SUBMIT-VISUAL-REVIEWS
CMD-RE-VISUAL-VERIFY → CMD-CREATE-VISUAL-FIXES
```

### 2.3 Integration dengan Workflow Generation
**File**: `internal/modules/work/workflow.go`

Modifikasi `WorkflowPrompt`:
- Tambahkan visual verification rules ketika project punya UI
- Jangan otomatis append commands, tapi berikan instruksi ke AI untuk include them
- AI akan generate commands dengan dependencies yang benar

## Wave 3: Visual Quality Evaluation Enhancement

### 3.1 Enhanced Visual Checks
**File**: `internal/modules/uicheck/uicheck.go` (sudah ada, perlu enhancement)

Tambahkan checks:
- Information hierarchy validation
- White space and density analysis
- State coverage verification (loading, empty, error, success)
- Responsive breakpoint testing
- Interaction feedback validation
- Console error detection

### 3.2 Visual Review API Enhancements
**File**: `internal/modules/pipeline/visual.go`

Enhance `RecordVisualReview`:
- Validate screenshot evidence lebih ketat
- Track review history per page
- Detect regression (page yang sebelumnya pass jadi fail)

## Wave 4: Autonomous Correction Loop

### 4.1 Correction Command Generator
**File baru**: `internal/modules/work/visualfixes.go`

Fungsi:
- `GenerateVisualFixCommands(p *core.Project, findings []core.VisualFinding) []core.Command`
- `PrioritizeFindings(findings []core.VisualFinding) []core.VisualFinding`
- `GroupFindingsByPage(findings []core.VisualFinding) map[string][]core.VisualFinding`

Prioritas:
1. Critical: Broken layouts, inaccessible elements
2. High: Contrast issues, missing states
3. Medium: Spacing inconsistencies, minor responsive issues
4. Low: Polish items

### 4.2 Correction Loop Controller
**File baru**: `internal/modules/pipeline/visualloop.go`

Fungsi:
- `RunCorrectionLoop(ctx context.Context, maxIterations int) error`
- Track iteration count untuk mencegah infinite loop
- Stop criteria:
  - No high/critical findings
  - Max iterations reached
  - No progress in last iteration

## Wave 5: Continuous Design Learning

### 5.1 Learning Data Structure
**File baru**: `internal/core/learning.go`

Struktur data:
```go
type DesignPattern struct {
    ID          string
    ProductType ProductType
    Pattern     string
    Context     string
    SuccessRate float64
    Examples    []string
}

type VisualFinding struct {
    Pattern     string
    Severity    string
    Frequency   int
    Resolution  string
    RootCause   string
}

type LearningRecord struct {
    ProjectID      string
    ProductType    ProductType
    Findings       []VisualFinding
    PatternsLearned []DesignPattern
    Timestamp      time.Time
}
```

### 5.2 Learning Storage
**File baru**: `internal/modules/learning/learning.go`

Fungsi:
- `RecordLearning(project *core.Project) error`
- `GetPatterns(productType ProductType) []DesignPattern`
- `SuggestImprovements(findings []VisualFinding) []string`

Storage:
- File-based: `.foundry/learnings.json`
- Aggregate across projects
- Track success rates

### 5.3 Learning Integration
**File**: `internal/modules/work/designcontext.go`

Modifikasi:
- Load learned patterns saat generate design rules
- Suggest improvements berdasarkan historical findings
- Track which patterns work best untuk setiap product type

## Wave 6: Quality Gates Integration

### 6.1 Visual Quality Gate
**File**: `internal/modules/pipeline/progress.go`

Enhance progress tracking:
- Track visual verification status per page
- Block finalize jika ada untested pages
- Warn jika ada high severity findings

### 6.2 Definition of Done Enhancement
**File**: `internal/modules/pipeline/finalize.go`

Tambahkan checks:
- All pages have visual reviews
- No critical/high findings unresolved
- All states tested (loading, empty, error)
- Responsive tested di minimal 2 breakpoints

## Implementation Steps

### Step 1: Fix Visual Workflow System
1. Perbaiki `visualworkflow.go` - generate commands yang valid
2. Update `workflow.go` - integrate visual verification rules
3. Write tests untuk visual workflow generation
4. Ensure semua existing tests pass

### Step 2: Implement Design Intelligence
1. Create `designcontext.go` dengan product type detection
2. Implement contextual design rules
3. Integrate dengan `DesignPrompt`
4. Write tests

### Step 3: Enhance Visual Quality Evaluation
1. Add lebih banyak checks di `uicheck.go`
2. Enhance visual review API
3. Write tests

### Step 4: Implement Correction Loop
1. Create `visualfixes.go` untuk generate fix commands
2. Create `visualloop.go` untuk control loop
3. Integrate dengan pipeline
4. Write tests

### Step 5: Implement Continuous Learning
1. Create learning data structures
2. Implement learning storage
3. Integrate dengan design intelligence
4. Write tests

### Step 6: Enhance Quality Gates
1. Update progress tracking
2. Enhance finalize checks
3. Write tests

## Testing Strategy

### Unit Tests
- Setiap function punya test
- Edge cases covered
- Mock AI responses

### Integration Tests
- End-to-end workflow generation
- Visual verification flow
- Correction loop execution
- Learning recording dan retrieval

### Regression Tests
- Pastikan existing functionality tidak broken
- All existing tests harus tetap pass

## Acceptance Criteria

✅ Blueprint generates context-aware design specifications
✅ Workflow UI memiliki visual verification steps dengan dependencies yang benar
✅ Setiap halaman dapat dilacak status visual verification dan bukti
✅ Implementasi dan visual verification memiliki status terpisah
✅ Visual findings dapat generate correction commands
✅ Screenshots di-capture setelah fixes dan dievaluasi ulang
✅ Kegagalan dan limitations dilaporkan dengan jujur
✅ Learnings dari satu project dapat digunakan di project lain
✅ No regression pada existing functionality
✅ All tests pass (unit, integration, regression)

## Timeline

- Step 1-2: 2-3 hari (foundation)
- Step 3-4: 2-3 hari (visual evaluation & correction)
- Step 5-6: 2-3 hari (learning & quality gates)
- Testing & refinement: 1-2 hari

**Total**: 7-11 hari

## Risks & Mitigation

**Risk 1**: AI tidak generate visual verification commands
**Mitigation**: Provide explicit rules dan examples di prompt

**Risk 2**: Correction loop infinite
**Mitigation**: Max iterations limit dan progress tracking

**Risk 3**: Learning data terlalu besar
**Mitigation**: Aggregate dan summarize, archive old data

**Risk 4**: Breaking existing functionality
**Mitigation**: Comprehensive testing, incremental changes
