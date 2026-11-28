import { defineConfig, devices } from '@playwright/test';

// Port 18181 by default so the suite never collides with Docker Compose on 8080.
// SAKURAOPS_E2E_PORT overrides it (e.g. to run two checkouts side by side).
const PORT = Number(process.env.SAKURAOPS_E2E_PORT ?? '18181');
if (!Number.isInteger(PORT) || PORT < 1024 || PORT > 65535) {
	throw new Error(
		`SAKURAOPS_E2E_PORT must be an integer in 1024-65535, got "${process.env.SAKURAOPS_E2E_PORT}"`
	);
}
const BASE = `http://127.0.0.1:${PORT}`;
const CI = !!process.env.CI;

export default defineConfig({
	testDir: './e2e',
	outputDir: './test-results',
	fullyParallel: true,
	// Few workers: each test drives a real browser; many at once causes timing flakiness.
	workers: 2,
	forbidOnly: CI,
	// Generous expect timeout: the real server and Chromium share a laptop with other work.
	expect: { timeout: 10_000 },
	timeout: 60_000, // real browser on shared CI runners and loaded laptops
	retries: 0, // a retry would hide flakiness
	reporter: CI ? [['list'], ['html', { open: 'never' }]] : [['list']],
	use: {
		baseURL: BASE,
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure'
	},
	// Chromium only for now; Firefox and WebKit are a later task (docs/dev/e2e.md).
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
	webServer: {
		// The REAL Go server with the embedded production build, not the Vite dev server.
		command: `make -C .. build && ../bin/sakuraops-server -listen 127.0.0.1:${PORT}`,
		url: `${BASE}/healthz`,
		timeout: 300_000,
		reuseExistingServer: !CI,
		stdout: 'ignore',
		stderr: 'pipe'
	}
});
