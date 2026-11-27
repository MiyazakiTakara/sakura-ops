# Browser tests (Playwright, Chromium only for now). Included by the root Makefile.
.PHONY: e2e-install e2e

## e2e-install: download Chromium and its system libraries (uses apt, needs sudo).
e2e-install:
	cd frontend && npx playwright install --with-deps chromium

## e2e: run the browser tests against the real embedded server (builds it first).
e2e:
	cd frontend && npm run e2e
