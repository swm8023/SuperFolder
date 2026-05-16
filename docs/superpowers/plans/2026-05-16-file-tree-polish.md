# File Tree Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve SuperFolder directory and file presentation with real icons, clearer metadata, and a virtualized details list.

**Architecture:** Keep backend and RPC unchanged. Add a small frontend presentation helper module for deterministic display decisions, then upgrade `BrowserPane` views to use `lucide-react` icons and `@tanstack/react-virtual` for details rows.

**Tech Stack:** React, TypeScript, Vitest, `@tanstack/react-virtual`, `lucide-react`.

---

### Task 1: Presentation Helpers

**Files:**
- Create: `app/src/superfolder/presentation.ts`
- Modify: `app/src/tests/superfolder.test.ts`

- [ ] **Step 1: Write the failing test**

Add tests that import `entryPresentation`, `formatEntrySize`, and `formatEntryTime` from `../superfolder/presentation`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd app; npm test -- src/tests/superfolder.test.ts`
Expected: FAIL because `presentation.ts` does not exist.

- [ ] **Step 3: Implement minimal helper module**

Create helper functions that classify folder/file icon kind, expose `hidden` and `readonly` badges, format directory size as `--`, and format timestamps with `Intl.DateTimeFormat`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd app; npm test -- src/tests/superfolder.test.ts`
Expected: PASS.

### Task 2: Virtualized File Views and Icons

**Files:**
- Modify: `app/package.json`
- Modify: `app/package-lock.json`
- Modify: `app/src/superfolder/components.tsx`

- [ ] **Step 1: Install UI dependencies**

Run: `cd app; npm install @tanstack/react-virtual lucide-react`
Expected: package files include both dependencies.

- [ ] **Step 2: Upgrade details, tile, and tree rows**

Use `useVirtualizer` for details rows and `lucide-react` icons for folders, files, refresh, add, close, list, grid, and tree controls. Keep existing selection, double-click, context menu, and inline rename behavior.

- [ ] **Step 3: Run frontend checks**

Run: `cd app; npm run typecheck && npm test`
Expected: PASS.

### Task 3: Visual Styling Pass

**Files:**
- Modify: `app/src/styles.css`

- [ ] **Step 1: Refine list and tree styling**

Add Explorer-like row density, hover/selected states, icon colors, metadata badges, and aligned columns without changing layout structure.

- [ ] **Step 2: Run full verification**

Run: `script\test.bat`
Expected: Go tests, TypeScript typecheck, Vitest, build, and headless smoke all PASS.
