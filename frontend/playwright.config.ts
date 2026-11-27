import { defineConfig, devices } from '@playwright/test';

// Port 18181 so the suite never collides with Docker Compose on 8080.
const PORT = 18181;
const BASE = `http://127.0.0.1:${PORT}`;
const CI = !!process.env.CI;

export default defineConfig({
	testDir: './e2e',
	outputDir: './test-results',
	fullyParallel: true,
	// Few workers: each test drives a real browser; many at once causes timing flakiness.
	workers: 2,
	forbidOnly: CI,
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
