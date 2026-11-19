# SakuraOps visual design and terminal window (approved direction)

Status: design specification, not implemented in the running app.

## Visual identity
- Keep the **first accepted mockup** as the design baseline. Do not redesign the app around the terminal.
- Background: `#15191F` graphite.
- Panels: `#252B34`. Console body: `#101820`.
- Primary Sakura accent: `#D9A0B7`, used sparingly for selection and active controls.
- Console success/prompt accent: `#98CFA8`.
- Favor compact, readable desktop admin UI, purposeful spacing and clear keyboard focus.

## Application shell
- Top bar: SakuraOps brand at left; notifications and user account at right.
- Left navigation: a **floating capsule with content-driven height** (fit-content), not a full-height sidebar. Only as tall as its menu options plus padding.
- Navigation: Dashboard, Servers, Terminal, Tasks, Schedules, Settings. Unimplemented destinations must not appear as functional controls.
- Main pages use normal routing, not draggable windows.
- On narrow screens, navigation collapses to an accessible menu.

## Terminal window: the only movable application window
- Clicking Terminal in navigation opens/focuses its window above the current page; switching pages must not close it.
- Separate title bar with SakuraOps Terminal label and clear affordances:
  - minimize (keeps session state; restore from Terminal entry);
  - maximize/restore (fills application workspace, not browser chrome);
  - close (removes visible window; confirm if a running command would be interrupted).
- Drag by title bar, not from inputs or buttons.
- Resize by edges/corners using a clear visible corner grip and minimum dimensions.
- Bring to front on focus. Constrain geometry so at least the title bar remains on-screen; respond to browser viewport changes.
- Multiple tabs may be supported after the basic single-window behavior is reliable; do not add inactive tab UI.
- Prompt `skr >`; semantic output, command history up/down, Ctrl+L to clear; Tab completion when implemented.
- Console is a **restricted SakuraOps command interpreter**, not unrestricted Bash/PTY; every command is parsed, authorized, audited and executed by backend services.
- Keyboard-accessible alternatives for drag/resize and maximization, visible focus, appropriately labeled controls, and resize/move status where feasible.
- Phone/narrow viewport: window occupies available application area; no dragging or inaccessible off-screen edges.

## Scope / acceptance
1. Visual shell matches approved proportions, compact left capsule and colors.
2. Terminal starts as a single movable/resizable window, not a routed standalone page.
3. Minimize, restore, maximize and close work, with stable layering and state.
4. Input continues to support existing `help`, `status`, `version` commands via authenticated API and CSRF protection.
5. No server-side shell access, no mock metrics presented as live production data.
6. Test with mouse, keyboard and smaller viewport; add tests for window actions and authenticated API calls.

## Implementation sequence
- Preserve existing FastAPI/local auth prototype.
- Implement app shell and a window manager for the terminal, then bind existing console API.
- Add optional Keycloak SSO and hardened auth as scheduled in ROADMAP.md.
- Implement CLI/API service commands and Git-backed inventory before production actions.
