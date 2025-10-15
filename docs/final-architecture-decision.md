# Final Architecture Decision

After consulting 3 different AI systems with fresh perspectives, here's the consensus and final recommendation.

## The Problem (Validated by All Three)

✅ **All three AIs confirmed**: The flat structure is the issue, not the code itself.

Key quotes:
- AI #1: "No clear grouping or hierarchy"
- AI #2: "The discomfort is mainly naming and co-location"
- **Grok**: "It resembles a 'bag of tools' without clear grouping"

**You're not being neurotic.** The structure needs hierarchy.

## The Solution: Layered with Clear Boundaries

```
internal/
├── application/         # "How the app works" (workflow coordination)
│   └── orchestrator/    # Renamed from sync/ for clarity
│       ├── orchestrator.go
│       ├── orchestrator_test.go
│       └── types.go
│
├── domain/              # "What the app does" (business logic)
│   ├── categorizer/     # AI categorization of items
│   ├── matcher/         # Transaction matching logic
│   └── splitter/        # Split creation logic
│
├── adapters/            # "Who the app talks to" (external systems)
│   ├── providers/       # Retailer data sources
│   │   ├── costco/
│   │   ├── walmart/
│   │   └── types.go     # Order, OrderItem, OrderProvider interfaces
│   └── clients/         # API client builders
│       └── clients.go
│
├── infrastructure/      # "What the app needs" (technical foundations)
│   ├── config/          # Configuration loading
│   ├── storage/         # SQLite persistence (DELETE storage_v2.go)
│   └── logging/         # Optional: inline if too thin
│
└── cli/                 # "How users interact" (command-line interface)
    ├── commands.go      # Renamed from providers.go
    ├── flags.go
    └── output.go
```

## Why This Structure?

### Mental Model: 4 Clear Layers + CLI

1. **Application Layer** - Orchestrates the workflow
   - Dependencies: Domain + Adapters + Infrastructure

2. **Domain Layer** - Business rules and logic
   - Dependencies: NONE (pure logic)
   - Matcher, splitter, categorizer live here

3. **Adapters Layer** - External system integrations
   - Dependencies: Infrastructure (for config, logging)
   - Providers (Walmart, Costco) and API clients

4. **Infrastructure Layer** - Technical necessities
   - Dependencies: NONE (leaf nodes)
   - Config, storage, logging

5. **CLI Layer** - User interface
   - Dependencies: Application (entry point)

### Dependency Flow

```
CLI ──────────────┐
                  ↓
Application ──> Domain
    │           (pure logic)
    ↓
Adapters ──> Infrastructure
(external)   (utilities)
```

**Rules enforced by structure:**
- Application can use anything
- Domain uses nothing (pure logic)
- Adapters can use Infrastructure
- Infrastructure is self-contained

## What Changed from Flat Structure

| Old Location | New Location | Why |
|-------------|--------------|-----|
| `internal/sync/` | `internal/application/orchestrator/` | Clearer naming |
| `internal/matcher/` | `internal/domain/matcher/` | It's business logic |
| `internal/splitter/` | `internal/domain/splitter/` | It's business logic |
| `internal/categorizer/` | `internal/domain/categorizer/` | It's business logic |
| `internal/providers/` | `internal/adapters/providers/` | It's an external adapter |
| `internal/clients/` | `internal/adapters/clients/` | It's an external adapter |
| `internal/config/` | `internal/infrastructure/config/` | It's infrastructure |
| `internal/storage/` | `internal/infrastructure/storage/` | It's infrastructure |
| `internal/observability/` | DELETE or `internal/infrastructure/logging/` | Too thin |
| `internal/cli/` | `internal/cli/` | Unchanged |

## Key Decisions Made

### 1. **"Domain" = Business Logic, Not Models**

Unlike some recommendations, we keep domain as **operations** (matcher, splitter, categorizer), not just types.

**Why?** These packages contain the core business rules that make the app work. They're not just data structures.

### 2. **Providers Live Under `adapters/`**

One AI suggested keeping `providers/` top-level. We rejected this.

**Why?** Providers ARE adapters to external systems (Walmart API, Costco API). Nesting them makes this explicit.

### 3. **Rename `sync/` → `orchestrator/`**

**Why?** "sync" is vague. "orchestrator" explicitly says "this coordinates everything."

### 4. **Delete or Inline `observability/`**

If it's just a logging wrapper, either:
- Delete it and use `log/slog` directly
- Keep as `infrastructure/logging/` if it has real logic

### 5. **Consolidate Storage**

Delete either `storage.go` or `storage_v2.go` - keep only the current version.

## Benefits of New Structure

### Before: Flat Structure
```
internal/
├── categorizer/    ← What is this?
├── cli/            ← What is this?
├── clients/        ← What is this?
├── config/         ← What is this?
├── matcher/        ← What is this?
├── observability/  ← What is this?
├── providers/      ← What is this?
├── splitter/       ← What is this?
├── storage/        ← What is this?
└── sync/           ← What is this?
```

**Mental overhead:** Can't quickly tell what depends on what or where to add features.

### After: Layered Structure
```
internal/
├── application/orchestrator/  ← The brain (coordinates everything)
├── domain/                    ← Business logic (pure)
│   ├── categorizer/
│   ├── matcher/
│   └── splitter/
├── adapters/                  ← External world
│   ├── providers/
│   └── clients/
├── infrastructure/            ← Technical needs
│   ├── config/
│   └── storage/
└── cli/                       ← User interface
```

**Clear answers:**
- Where's the entry point? `application/orchestrator/`
- Where's business logic? `domain/`
- Where do providers go? `adapters/providers/`
- Can domain use adapters? No (visual hierarchy)
- Where does order validation go? `domain/validation/` (new package)

## Migration Plan

See [architecture-migration-plan.md](./architecture-migration-plan.md) for detailed steps.

**Summary:**
1. Create new directories
2. `git mv` packages to new locations
3. Update import paths (find/replace)
4. Delete `storage_v2.go` and `observability/`
5. Rename `cli/providers.go` → `cli/commands.go`
6. Verify build and tests
7. Commit

**Estimated time:** 1 hour

## Rollback Plan

```bash
git reset --hard HEAD  # Before commit
git revert HEAD        # After commit
```

## Success Criteria

After migration:
- ✅ New developers can understand structure in 30 seconds
- ✅ "Where does X go?" has obvious answer
- ✅ Visual hierarchy shows dependencies
- ✅ All tests pass
- ✅ No import cycles
- ✅ Documentation updated

## Alternative: Document Only (Not Recommended)

**If you don't want to reorganize**, you could just:
1. Add `docs/architecture-overview.md` explaining the layers
2. Add ASCII diagram to README
3. Keep flat structure

**Why not recommended:** Documentation gets stale. Structure is self-documenting.

## Consensus from All Three AIs

✅ **Unanimous agreement:**
1. Flat structure is the problem
2. Need 4-5 top-level groups
3. Orchestrator is special (needs clear location)
4. Config/storage are infrastructure
5. You're not overthinking it

✅ **Strong consensus (2/3 agree):**
- Providers should nest under adapters
- Matcher/splitter are business logic (not just data)
- Rename sync to be clearer

## Next Steps

1. **Review this decision doc**
2. **Choose approach:**
   - Option A: Run migration (recommended)
   - Option B: Document only (not recommended)
3. **If migrating:** Follow [architecture-migration-plan.md](./architecture-migration-plan.md)
4. **Update README** with new structure
5. **Commit with message:** `refactor: reorganize into application/domain/adapters/infrastructure layers`

## Questions?

- **Will this break anything?** No, just moving files and updating imports
- **How long does it take?** ~1 hour with the automated script
- **Can I do it incrementally?** Yes, but all-at-once is cleaner
- **What if I don't like it?** Rollback with `git revert`

---

**Decision:** Proceed with layered reorganization as described above.

**Date:** 2025-10-13

**Reasoning:** Validated by 3 independent AI reviews. All confirmed the flat structure is non-intuitive and recommended layering.
