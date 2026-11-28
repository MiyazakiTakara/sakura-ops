import { expect, test } from '@playwright/test';
import { watchPage } from './helpers';

test.describe('root and server behaviour', () => {
	test('root renders the shell heading with no console errors and no CSP violations', async ({
		page
	}) => {
		const problems = await watchPage(page);
		// The e2e server has no database, so /readyz really answers 503 and the browser logs that
		// failed load as a console error. The shell is checked here in its ready state (mocked).
		await page.route('**/readyz', (route) =>
			route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ status: 'ready', schema_version: 1 })
			})
		);
		await page.goto('/');
		await expect(page.getByRole('heading', { level: 1, name: 'Pulpit' })).toBeVisible();
		await page.waitForLoadState('networkidle');
		expect(problems).toEqual([]);
	});

	test('unknown /api path returns the 404 JSON envelope, never HTML', async ({ request }) => {
		const res = await request.get('/api/x');
		expect(res.status()).toBe(404);
		expect(res.headers()['content-type']).toContain('application/json');
		const body = await res.json();
		expect(Object.keys(body).sort()).toEqual(['code', 'message', 'request_id']);
	});

	test('/healthz returns ok', async ({ request }) => {
		const res = await request.get('/healthz');
		expect(res.status()).toBe(200);
		expect(await res.json()).toEqual({ status: 'ok' });
	});

	test('the page is served with a CSP without unsafe-inline for scripts', async ({ request }) => {
		const res = await request.get('/');
		const csp = res.headers()['content-security-policy'] ?? '';
		expect(csp).not.toBe('');
		const script = csp.split(';').find((d) => d.trim().startsWith('script-src')) ?? '';
		expect(script).not.toContain('unsafe-inline');
		expect(script).not.toContain('unsafe-eval');
	});
});
