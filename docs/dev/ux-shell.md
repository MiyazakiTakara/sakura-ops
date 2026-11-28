# UX shell prototype (NEXT-006)

Status: prototype of the application shell described in `DESIGN.md`. It is not a finished UI and
must not be called one. All real data comes from the two endpoints that exist today.

## What is in the prototype

- Layout (`frontend/src/routes/(app)/+layout.svelte`): skip link, `banner` (product name), a floating
  navigation capsule (`nav`, labelled "Główna nawigacja") and `main`. The capsule is about 180 px wide and
  sticky at 1200 px and above, a horizontal capsule with visible labels at 768-1199 px, and sits behind a
  visible Menu button (`aria-expanded`, `aria-controls`, Escape closes and restores focus) below 768 px.
  Touch targets in the navigation are at least 44 px high.
- Routes: Pulpit `/`, Serwery `/servers`, Zadania `/tasks`, Ustawienia `/settings`, and a not-found page
  for any other path. The route group `(app)` carries the shell; the technical spike
  `/spike/terminal` stays outside it with its own window, so its tests are unchanged.
- Real data: `GET /healthz` and `GET /readyz` through `frontend/src/lib/api/system.ts`, typed from the
  generated `schema.d.ts` and narrowed at runtime with the guards in `errors.ts`. The status panel
  (Pulpit and Ustawienia) is checked on load, every 30 s, when the tab becomes visible again and on
  "Sprawdź teraz" (GET only, so retry never repeats a mutation).
- Floating skr terminal: opened from the "Terminal" capsule action, hosted in the layout so routing
  never destroys the session. Open, close, minimize/restore, drag, resize and keyboard move/resize are the
  existing `FloatingWindow.svelte` and `TerminalPane.svelte` behaviour. It is still the local-echo spike
  interpreter (`help`, `clear`, `show version`): no backend, no command execution path. The shell
  does not add one.

## States shown

| State | Where it comes from |
| --- | --- |
| Loading ("Sprawdzanie…") | first check not finished |
| OK ("Działa" / "Gotowy", with schema version and check time) | 200 and a body matching the contract |
| Refused ("Niegotowy") | `/readyz` 503 envelope: `db_not_configured`, `db_unavailable`, `schema_outdated`, `schema_ahead`, unknown code; shows cause, next step, code and request id |
| Unavailable ("Brak odpowiedzi" / "Niedostępne") | network error, timeout (5 s), or a 200/5xx body that does not match the contract (the body is never displayed) |
| Stale ("Nieaktualne") | a success older than 60 s with no newer check; after a failed refresh the last success is mentioned only as outdated, never shown as current |
| Empty / not available | Serwery, Zadania and the not-yet-implemented parts of Ustawienia say "Niedostępne" and why; there are no buttons |
| Forbidden | not applicable yet: no authentication or RBAC exists, so there is no 403 to render |

Against the e2e server (no database) `/readyz` really answers 503 `db_not_configured`, so that is what
the prototype shows there.

## Accessibility

Landmarks, skip link ("Przejdź do treści", moves focus to `main`), focus moved to `main` after client-side
navigation, `aria-current="page"` plus bold accent text and a bar (not colour alone), 2 px accent focus
ring, one polite `role="status"` sentence for the check outcome (not logs), `prefers-reduced-motion`
(global rule in `app.css`; the shell has no transitions anyway). Layouts are tested at 360, 768, 1280 and
1920 px; the existing 200% zoom test stays on the spike page. axe-core (WCAG 2.x A/AA and best-practice)
runs on all shell routes, on the error state with the terminal open, and at 360 px with the menu open.

Contrast of the DESIGN.md tokens (computed WCAG ratios): text on bg/panel/elevated 15.7/12.7/10.7;
muted 9.7/7.8/6.6; accent on bg/panel/elevated 8.1/6.6/5.5; on-accent on accent 8.1; success, warning,
danger on panel 8.0/8.1/7.0; control-border on bg/panel/elevated 4.7/3.8/3.2 (3:1 needed for controls);
line is decorative only (2.0 on bg).

### Terminal keyboard exit and focus

xterm consumes Tab, so Tab cannot leave the terminal (WCAG 2.1.2). Two exits move focus to the window
title bar: **Escape** (existing behaviour, kept and tested) and **Ctrl+]**, the telnet/SSH escape
character. When a PTY-backed SSH tab arrives (issue #26) Escape must be forwarded to the remote program;
Ctrl+] is then the only exit and the Escape path is to be removed in that change. Opening the terminal from
the capsule focuses its input; Close returns focus to the "Terminal" action; the action restores a
minimized window. The window opens at the right (and below the capsule on narrower screens) so it does
not cover the navigation. Below 768 px it uses the whole area.

## Deviations from DESIGN.md

- Nav labels and shell text are Polish as in DESIGN.md; `document.documentElement.lang` is set to `pl`
  while the shell is mounted and the terminal host is `lang="en"` because the window chrome and skr output
  are English (the window labels are asserted by existing tests). Localisation is not done.
- Topbar has only the product name: "stan instancji w ustawieniach" is honoured (details live in Ustawienia,
  the same panel is on Pulpit as the only real content); notifications and account do not exist yet and are
  absent, not faked. Harmonogramy is absent until it exists.
- The capsule is sticky at 1200 px and above (floating, not full height). Colour tokens are the DESIGN.md
  proposals, unchanged.
- The e2e `server.spec.ts` root test mocks `/readyz` as ready, because a real 503 is logged by the browser
  as a console error.

## Known limitations (to track in #26)

1. The terminal is still the spike: local echo only, no modes, parser, Tab for real commands or API. The
   banner and `aria-label` still say "local spike".
2. No maximize/restore, and window position and size are not persisted across a page reload.
3. Escape/Ctrl+] decision for the SSH PTY (above) must be finalised with the SSH gateway.
4. No component tests: Vitest runs in the `node` environment and covers state logic only (`status.ts`,
   `nav.ts`, `system.ts`, `errors.ts`). Component behaviour is covered by Playwright. A DOM environment
   would need a new dev dependency (jsdom or happy-dom) and was not justified.
5. A real 503 from `/readyz` shows up as a browser console error (resource load), so "no console errors"
   cannot hold while the hub is genuinely not ready. Tests mock a ready hub for that assertion.
6. Status polling stops while the tab is hidden and resumes on visibility; a hidden tab can therefore
   show a stale row until it is focused again.
7. The shell does not yet link or reach the technical spike or the YAML editor; the CodeMirror editor
   has no home in the shell until Task v1 work (NEXT-014).
8. Contrast was computed for tokens and checked by axe on rendered pages; manual review in a real
   browser at 200% zoom for the shell routes, a screen reader pass and Firefox/WebKit are not done.
9. Bundle size was not split or measured (issue #26, gate for NEXT-011); the xterm/CodeMirror chunks are
   still loaded with the shell layout.
