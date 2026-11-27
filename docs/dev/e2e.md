# Browser tests (Playwright)

Status: NEXT-005/006 browser tests of the technical spike UI. Not a product test suite yet.

## Run

```
make web-install        # once: npm ci
make e2e-install        # once: Chromium + system libraries (npx playwright install --with-deps, uses apt/sudo)
make e2e                # runs all tests
cd frontend && npx playwright test e2e/window.spec.ts   # one file
```

`frontend/playwright.config.ts` starts the REAL server: `make build` (SPA embedded into the Go
binary) and then `bin/sakuraops-server -listen 127.0.0.1:18181`. Port 18181 keeps it away from
Docker Compose on 8080. There is no Vite dev server and no mocked backend in these tests. Locally an
already running server on 18181 is reused (start it yourself after `make build` to skip the build);
when `CI` is set the config refuses to reuse one. The binary must be built from the code under test.

Two workers, no retries: a retry would hide flakiness. Traces and screenshots are kept only for
failed tests (`frontend/test-results/`, HTML report in `frontend/playwright-report/` in CI). The
workflow `.github/workflows/e2e.yml` (pull_request only, `contents: read`) uploads them only on
failure with 7 days retention. They show the fictional spike page only.

## What is covered (Chromium only)

| File | Covered |
| --- | --- |
| `server.spec.ts` | root heading, no console errors, no CSP violations (`securitypolicyviolation` listener installed before page scripts), `/api/x` is 404 JSON never HTML, `/healthz`, CSP header has no `unsafe-inline`/`unsafe-eval` for scripts |
| `window.spec.ts` | floating skr window: title, drag, stays fully inside the viewport, resize within bounds, keyboard move (title bar arrows) and resize (resize handle arrows), minimize/restore keeps content, close and reopen, visible focus ring |
| `terminal.spec.ts` | `skr > ` prompt, `help`, Up history, Tab completion (unique, listing, no match), Ctrl+L, a line wider than the terminal redraws without leftovers, unknown command does not echo input, pasted ESC sequences are not interpreted (read from `.xterm-rows`), no network requests except same-origin static assets |
| `editor.spec.ts` | CodeMirror shows the YAML, is editable, keyboard typing and undo |
| `layout.spec.ts` | 360/768/1280/1920 px: no global horizontal scroll, window fits, phone (< 768 px) uses the full area; viewport shrink pulls the window back; 200% zoom |
| `keyboard.spec.ts` | Tab order (editor, title bar, Minimize, Close, terminal), Escape leaves the terminal and never closes the window, Enter/Space on controls |
| `a11y.spec.ts` | axe-core (WCAG 2.0/2.1/2.2 A and AA plus best-practice rules) on `/` and `/spike/terminal` |

Terminal output is read from the rendered DOM rows. No test hooks exist in production code.

## Findings fixed while writing the suite

The first runs failed against the spike UI and led to fixes in `FloatingWindow.svelte`,
`TerminalPane.svelte` and `YamlEditor.svelte`: the window could extend past the viewport edge
(clamp kept only 80 px visible), had a fixed 640 px width on a 360 px phone (global horizontal
overflow), Tab could not leave the terminal (keyboard trap; Escape now moves focus to the window
title bar), the YAML editor token colours failed WCAG AA contrast on the dark background,
`<header role="toolbar">` is not an allowed role, and long prompt lines left wrapped fragments
after editing. axe now reports no violations on `/` and `/spike/terminal` with no exclusions.

## 200% zoom method

Browsers expose no page-zoom API to automation. At 200% zoom a 1280x720 viewport lays out as a
640x360 CSS-pixel viewport at 2x pixel density, and that is what the zoom test uses:
`viewport: 640x360` plus `deviceScaleFactor: 2` (asserted via `devicePixelRatio`). This reproduces
what WCAG 1.4.4/1.4.10 cares about (reflow and reachability). It does not reproduce browser UI
(zoom indicator, scaled scrollbars) or text-only zoom. Note that 640 px is below the 768 px phone
breakpoint, so the window is full-screen and has no resize handle in this test, as DESIGN.md intends.

## Not covered

- Firefox and WebKit: Chromium only for now; add projects to the config in a later task.
- Real API, authentication, RBAC, SSE: none exists yet. Tests must move to seeded fictional data
  against the real API when it does (docs/17-testing.md).
- Touch input, screen readers, high-contrast mode, real browser page zoom.
- Visual regression screenshots; there are no baselines.

## Licences

`@playwright/test` is Apache-2.0. `@axe-core/playwright` is MPL-2.0 (and axe-core, which it pulls in,
too); both are development dependencies only and are not shipped in the server binary or the
SPA bundle. Versions are pinned exactly in `frontend/package.json`.
