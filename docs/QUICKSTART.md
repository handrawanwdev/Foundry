# Quick Start Guide

## 5-Step Setup (Mostly Automatic!)

### Step 1: Create Project
```bash
cd /path/to/your/project
foundry init
foundry story "Dashboard analytics untuk monitoring sales dengan real-time metrics"
```

**What happens automatically:**
- ✅ Detects product type: "Dashboard/Analytics"
- ✅ Applies contextual design rules (dense layout, tables not cards)

---

### Step 2: Generate Design
```bash
foundry design
```

**What happens automatically:**
- ✅ Uses product-specific rules
- ✅ Loads learned patterns from previous projects (if any)
- ✅ Generates contextual design (not generic)

**Output example:**
```
PRODUCT CONTEXT: Dashboard/Analytics
- Use compact tables with sortable columns
- KPI cards with trend indicators
- Dense layout acceptable for monitoring
```

---

### Step 3: Generate Workflow
```bash
foundry workflow
```

**What happens automatically:**
- ✅ Two-phase batching if blueprint > 50KB
- ✅ Auto-adds 4 visual verification commands:
  - CMD-VERIFY-DESIGN
  - CMD-CAPTURE-SCREENSHOTS
  - CMD-SUBMIT-VISUAL-REVIEWS
  - CMD-CREATE-VISUAL-FIXES

---

### Step 4: Run Coding Agent
```bash
foundry run claude-code
# or
foundry run codex
# or
foundry run opencode
```

**Agent will:**
- Implement according to workflow
- Capture screenshots automatically
- Submit visual reviews
- Fix issues autonomously (correction loop)

---

### Step 5: Check Results
```bash
# Check visual status
foundry visual report

# Expected output:
# Pages: 10
# Pass: 10 ✅
# Partial: 0
# Fail: 0

# Check learnings captured
foundry learning summary

# Finalize
foundry finalize
```

---

## Manual Commands (Optional)

### Check Progress
```bash
foundry status              # Workflow status
foundry visual report       # Visual verification status
foundry warnings            # Any warnings
```

### Manual Visual Fixes
```bash
# If auto-fix didn't work, trigger manually
foundry visual fixes
```

### View Learned Patterns
```bash
foundry learning summary
```

Output example:
```
Total Projects: 15
Common Findings:
- contrast (high): 12 occurrences
- spacing (medium): 8 occurrences

Recommended Rules:
- Ensure WCAG AA contrast (4.5:1)
- Use consistent spacing tokens
```

### Reset Learnings (Rare)
```bash
rm -rf .foundry/learnings/
```

---

## Example: Dashboard Project

```bash
# 1. Init
foundry init
foundry story "Dashboard monitoring server metrics: CPU, memory, disk usage"

# 2. Design (auto-detects: Dashboard)
foundry design

# 3. Workflow (auto-adds visual commands)
foundry workflow

# 4. Implement
foundry run claude-code

# 5. Check & Finalize
foundry visual report    # Should show all pass
foundry finalize
```

**That's it!** The system handles:
- ✅ Contextual design rules
- ✅ Visual verification
- ✅ Autonomous fixes
- ✅ Learning for next project

---

## What's Automatic vs Manual

| Task | Automatic? | Command |
|------|------------|---------|
| Detect product type | ✅ Yes | - |
| Apply contextual rules | ✅ Yes | - |
| Add visual commands | ✅ Yes | - |
| Capture screenshots | ✅ Yes | - |
| Submit reviews | ✅ Yes | - |
| Generate fixes | ✅ Yes | - |
| Correction loop | ✅ Yes | - |
| Record learnings | ✅ Yes | - |
| Check visual status | ❌ No | `foundry visual report` |
| Check learnings | ❌ No | `foundry learning summary` |
| Manual fix trigger | ❌ No | `foundry visual fixes` |

---

## Troubleshooting

### Visual verification stuck?
```bash
foundry visual report
# Check which pages are failing
```

### Need to retry visual checks?
```bash
foundry visual fixes  # Manual trigger
```

### Want to see what was learned?
```bash
foundry learning summary
```

### Reset everything?
```bash
rm -rf .foundry/
foundry init  # Start fresh
```

---

## Tips for Best Results

1. **Write detailed stories** → Better product type detection
2. **Let agent capture all screenshots** → Complete visual verification
3. **Don't delete `.foundry/learnings/`** → Next project benefits
4. **Check visual report before finalize** → Ensure quality
5. **Use `foundry status` often** → Track progress

---

## File Structure

```
your-project/
├── .foundry/
│   ├── project.json          # Project state
│   └── learnings/
│       └── learnings.json    # Learned patterns
├── docs/
│   └── foundry/
│       ├── blueprint/
│       │   ├── design-rules.md
│       │   └── layout-spec.md
│       └── workflow/
│           └── CMD-*.md
└── visual-review/            # Screenshots (auto-created)
    ├── page-1/
    │   ├── desktop.png
    │   ├── tablet.png
    │   └── mobile.png
    └── page-2/
        └── ...
```

---

**TL;DR:** Just run `foundry init` → `foundry story` → `foundry design` → `foundry workflow` → `foundry run <agent>` → `foundry finalize`. Everything else is automatic! 🚀
