import AxeBuilder from '@axe-core/playwright';
import { expect, test, type Page } from '@playwright/test';
import { termInput, termText, titlebar, watchPage } from './helpers';

const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa', 'best-practice'];

const READY = { status: 'ready', schema_version: 7 };
const apiError = (code: string) => ({ code, message: 'fictional', request_id: 'req-test-1' });

/** Test-only mock of /readyz through route interception; production code has no hooks. */
async function mockReadyz(
	page: Page,
	reply: { status: number; body: unknown } | 'abort'
): Promise<void> {
	await page.route('**/readyz', (route) =>
		reply === 'abort'
			? route.abort()
			: route.fulfill({
					status: reply.status,
					contentType: 'application/json',
					body: JSON.stringify(reply.body)
				})
	);
}

const statusRow = (page: Page, id: 'health' | 'ready') => page.getByTestId(`status-${id}`);

test.describe('shell landmarks and navigation', () => {
	test('has banner, labelled navigation, main and one h1', async ({ page }) => {
		await page.goto('/');
		await expect(page.getByRole('banner')).toBeVisible();
		await expect(page.getByRole('navigation', { name: 'Główna nawigacja' })).toBeVisible();
		await expect(page.getByRole('main')).toBeVisible();
		await expect(page.getByRole('heading', { level: 1 })).toHaveText('Pulpit');
	});

	test('every nav item leads to a page, marks itself current, and placeholders say so', async ({
		page
	}) => {
		await page.goto('/');
		const nav = page.getByRole('navigation', { name: 'Główna nawigacja' });
		for (const [name, path] of [
			['Serwery', '/servers'],
			['Zadania', '/tasks'],
			['Ustawienia', '/settings'],
			['Pulpit', '/']
		] as const) {
			await nav.getByRole('link', { name }).click();
			await expect(page).toHaveURL(path);
			await expect(page.getByRole('heading', { level: 1, name })).toBeVisible();
			await expect(nav.getByRole('link', { name })).toHaveAttribute('aria-current', 'page');
			await expect(nav.locator('[aria-current="page"]')).toHaveCount(1);
		}
		for (const path of ['/servers', '/tasks']) {
			await page.goto(path);
			await expect(page.getByRole('heading', { level: 2, name: 'Niedostępne' })).toBeVisible();
			// No fake data and no dead buttons inside the content area.
			await expect(page.getByRole('main').getByRole('button')).toHaveCount(0);
		}
		await expect(nav.getByRole('link', { name: 'Harmonogramy' })).toHaveCount(0);
	});

	test('after client-side navigation focus moves to the content', async ({ page }) => {
		await page.goto('/');
		await page.getByRole('link', { name: 'Serwery' }).click();
		await expect(page.getByRole('main')).toBeFocused();
	});

	test('unknown route shows a not-found page inside the shell', async ({ page }) => {
		await page.goto('/nope');
		await expect(page.getByRole('navigation', { name: 'Główna nawigacja' })).toBeVisible();
		await expect(
			page.getByRole('heading', { level: 1, name: 'Nie znaleziono strony' })
		).toBeVisible();
		await expect(page.getByRole('navigation').locator('[aria-current="page"]')).toHaveCount(0);
	});
});

test.describe('skip link and keyboard-only flow', () => {
	test('first Tab shows the skip link, Enter moves focus to main', async ({ page }) => {
		await page.goto('/');
		await page.getByRole('heading', { level: 1 }).waitFor();
		await page.keyboard.press('Tab');
		const skip = page.getByRole('link', { name: 'Przejdź do treści' });
		await expect(skip).toBeFocused();
		await expect(skip).toBeInViewport({ ratio: 1 });
		await page.keyboard.press('Enter');
		await expect(page.getByRole('main')).toBeFocused();
	});

	test('focus is visible on nav links', async ({ page }) => {
		await page.goto('/');
		const link = page.getByRole('link', { name: 'Serwery' });
		await link.focus();
		const outline = await link.evaluate((el) => getComputedStyle(el).outlineStyle);
		expect(outline).not.toBe('none');
	});

	test('terminal opens from the nav by keyboard, Ctrl+] leaves it, Close restores focus', async ({
		page
	}) => {
		await page.goto('/');
		await expect(page.getByRole('region', { name: 'skr' })).toHaveCount(0);
		const button = page.getByRole('button', { name: 'Terminal' });
		await button.focus();
		await page.keyboard.press('Enter');
		await expect(page.getByRole('region', { name: 'skr' })).toBeVisible();
		await expect(termInput(page)).toBeFocused();

		await page.keyboard.press('Control+]');
		await expect(titlebar(page)).toBeFocused();
		await expect(page.getByRole('region', { name: 'skr' })).toBeVisible();

		await page.getByRole('button', { name: 'Close' }).focus();
		await page.keyboard.press('Enter');
		await expect(page.getByRole('region', { name: 'skr' })).toHaveCount(0);
		await expect(button).toBeFocused();
	});

	test('Terminal button restores a minimized window and focuses the terminal', async ({ page }) => {
		await page.goto('/');
		await page.getByRole('button', { name: 'Terminal' }).click();
		await page.getByRole('button', { name: 'Minimize' }).click();
		await expect(page.getByRole('button', { name: 'Restore' })).toBeVisible();
		await page.getByRole('button', { name: 'Terminal' }).click();
		await expect(page.getByRole('button', { name: 'Minimize' })).toBeVisible();
		await expect(termInput(page)).toBeFocused();
	});

	test('terminal session survives routing', async ({ page }) => {
		await page.goto('/');
		await page.getByRole('button', { name: 'Terminal' }).click();
		await expect(page.locator('.xterm-rows').getByText('skr >').first()).toBeVisible();
		await page.keyboard.type('show version');
		await page.keyboard.press('Enter');
		await expect.poll(() => termText(page)).toContain('no backend');
		await page.getByRole('link', { name: 'Ustawienia' }).click();
		await expect(page.getByRole('heading', { level: 1, name: 'Ustawienia' })).toBeVisible();
		await expect(page.getByRole('region', { name: 'skr' })).toBeVisible();
		expect(await termText(page)).toContain('no backend');
	});
});

test.describe('instance status (real /healthz and /readyz)', () => {
	test('against the e2e server (no database) readiness is honestly not ready', async ({ page }) => {
		await page.goto('/');
		await expect(statusRow(page, 'health')).toContainText('Działa');
		await expect(statusRow(page, 'ready')).toContainText('Niegotowy');
		await expect(statusRow(page, 'ready')).toContainText('db_not_configured');
	});

	test('ready response shows the schema version', async ({ page }) => {
		await mockReadyz(page, { status: 200, body: READY });
		await page.goto('/');
		await expect(statusRow(page, 'ready')).toContainText('Gotowy');
		await expect(statusRow(page, 'ready')).toContainText('Wersja schematu bazy: 7');
	});

	for (const code of ['schema_ahead', 'schema_outdated', 'db_unavailable']) {
		test(`503 ${code} shows cause, next step, code and request id`, async ({ page }) => {
			await mockReadyz(page, { status: 503, body: apiError(code) });
			await page.goto('/settings');
			const row = statusRow(page, 'ready');
			await expect(row).toContainText('Niegotowy');
			await expect(row).toContainText('Następny krok');
			await expect(row).toContainText(code);
			await expect(row).toContainText('req-test-1');
		});
	}

	test('loading state is shown while the request is pending', async ({ page }) => {
		let release: () => void = () => {};
		const gate = new Promise<void>((r) => (release = r));
		await page.route('**/readyz', async (route) => {
			await gate;
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify(READY)
			});
		});
		await page.goto('/');
		await expect(statusRow(page, 'ready')).toContainText('Sprawdzanie');
		release();
		await expect(statusRow(page, 'ready')).toContainText('Gotowy');
	});

	test('network failure after a success is shown as unavailable with stale data, not as current', async ({
		page
	}) => {
		let fail = false;
		await page.route('**/readyz', (route) =>
			fail
				? route.abort()
				: route.fulfill({
						status: 200,
						contentType: 'application/json',
						body: JSON.stringify(READY)
					})
		);
		await page.goto('/');
		await expect(statusRow(page, 'ready')).toContainText('Gotowy');
		fail = true;
		await page.getByRole('button', { name: 'Sprawdź teraz' }).click();
		const row = statusRow(page, 'ready');
		await expect(row).toContainText('Niedostępne');
		await expect(row).toContainText('jest nieaktualny');
		await expect(row).not.toContainText('Gotowy');
	});

	test('off-contract answer is unavailable and its body is never shown', async ({ page }) => {
		await mockReadyz(page, { status: 200, body: { status: 'ready', secret: 'leaky-detail' } });
		await page.goto('/');
		await expect(statusRow(page, 'ready')).toContainText('Niedostępne');
		await expect(page.locator('body')).not.toContainText('leaky-detail');
	});

	test('retry sends GET requests only', async ({ page }) => {
		const methods: string[] = [];
		page.on('request', (r) => {
			if (/\/(readyz|healthz)$/.test(new URL(r.url()).pathname)) methods.push(r.method());
		});
		await page.goto('/');
		await expect(statusRow(page, 'ready')).toContainText('Niegotowy');
		await page.getByRole('button', { name: 'Sprawdź teraz' }).click();
		await expect(page.getByRole('button', { name: 'Sprawdź teraz' })).toBeEnabled();
		expect(methods.length).toBeGreaterThanOrEqual(4);
		expect(new Set(methods)).toEqual(new Set(['GET']));
	});
});

test.describe('responsive', () => {
	for (const width of [360, 768, 1280, 1920]) {
		test(`${width}px: no global horizontal scroll on any route, terminal open`, async ({
			page
		}) => {
			await page.setViewportSize({ width, height: 800 });
			for (const path of ['/', '/servers', '/tasks', '/settings']) {
				await page.goto(path);
				await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
				const overflow = await page.evaluate(
					() => document.documentElement.scrollWidth - document.documentElement.clientWidth
				);
				expect(overflow, path).toBeLessThanOrEqual(0);
			}
			if (width >= 768) {
				await page.getByRole('button', { name: 'Terminal' }).click();
				await expect(page.getByRole('region', { name: 'skr' })).toBeVisible();
				const overflow = await page.evaluate(
					() => document.documentElement.scrollWidth - document.documentElement.clientWidth
				);
				expect(overflow).toBeLessThanOrEqual(0);
			}
		});
	}

	test('1280px: compact capsule (about 180px) at the left, content beside it', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await page.goto('/');
		const nav = await page.getByRole('navigation', { name: 'Główna nawigacja' }).boundingBox();
		const main = await page.getByRole('main').boundingBox();
		expect(nav && Math.round(nav.width)).toBe(180);
		expect(nav && main && main.x).toBeGreaterThanOrEqual(nav!.x + nav!.width);
		await expect(page.getByRole('button', { name: 'Menu' })).toBeHidden();
	});

	test('768px: horizontal capsule with labels, no Menu button', async ({ page }) => {
		await page.setViewportSize({ width: 768, height: 800 });
		await page.goto('/');
		await expect(page.getByRole('button', { name: 'Menu' })).toBeHidden();
		await expect(page.getByRole('link', { name: 'Zadania' })).toBeVisible();
		const nav = await page.getByRole('navigation', { name: 'Główna nawigacja' }).boundingBox();
		const main = await page.getByRole('main').boundingBox();
		expect(nav && main && main.y).toBeGreaterThanOrEqual(nav!.y + nav!.height);
	});

	test('360px: Menu button toggles the navigation, Escape closes it and restores focus', async ({
		page
	}) => {
		await page.setViewportSize({ width: 360, height: 800 });
		await page.goto('/');
		const menu = page.getByRole('button', { name: 'Menu' });
		const nav = page.getByRole('navigation', { name: 'Główna nawigacja' });
		await expect(nav).toBeHidden();
		await expect(menu).toHaveAttribute('aria-expanded', 'false');
		await menu.focus();
		await page.keyboard.press('Enter');
		await expect(menu).toHaveAttribute('aria-expanded', 'true');
		await expect(nav).toBeVisible();
		await page.keyboard.press('Tab');
		await expect(page.getByRole('link', { name: 'Pulpit' })).toBeFocused();
		await page.keyboard.press('Escape');
		await expect(nav).toBeHidden();
		await expect(menu).toBeFocused();
		await menu.click();
		await page.getByRole('link', { name: 'Serwery' }).click();
		await expect(page.getByRole('heading', { level: 1, name: 'Serwery' })).toBeVisible();
		await expect(nav).toBeHidden();
	});

	test('360px: touch targets in the navigation are at least 44px high', async ({ page }) => {
		await page.setViewportSize({ width: 360, height: 800 });
		await page.goto('/');
		await page.getByRole('button', { name: 'Menu' }).click();
		for (const loc of [
			page.getByRole('button', { name: 'Menu' }),
			page.getByRole('link', { name: 'Pulpit' }),
			page.getByRole('button', { name: 'Terminal' })
		]) {
			const b = await loc.boundingBox();
			expect(b?.height).toBeGreaterThanOrEqual(44);
		}
	});

	test('360px: the terminal takes the full area', async ({ page }) => {
		await page.setViewportSize({ width: 360, height: 640 });
		await page.goto('/');
		await page.getByRole('button', { name: 'Menu' }).click();
		await page.getByRole('button', { name: 'Terminal' }).click();
		const b = await page.getByRole('region', { name: 'skr' }).boundingBox();
		expect(b).toMatchObject({ x: 0, y: 0, width: 360, height: 640 });
	});
});

test.describe('motion, console and CSP', () => {
	test('prefers-reduced-motion: no transitions or animations on shell controls', async ({
		page
	}) => {
		await page.emulateMedia({ reducedMotion: 'reduce' });
		await page.goto('/');
		const d = await page
			.getByRole('link', { name: 'Serwery' })
			.evaluate((el) => getComputedStyle(el).transitionDuration);
		expect(d === '0s' || d === '').toBe(true);
	});

	test('all routes and the open terminal: no console errors and no CSP violations', async ({
		page
	}) => {
		const problems = await watchPage(page);
		for (const path of ['/', '/servers', '/tasks', '/settings']) {
			await page.goto(path);
			await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
		}
		await page.getByRole('button', { name: 'Terminal' }).click();
		await expect(page.locator('.xterm-rows').getByText('skr >').first()).toBeVisible();
		await page.keyboard.type('help');
		await page.keyboard.press('Enter');
		await page.waitForLoadState('networkidle');
		// /readyz answers 503 on the e2e server: the browser logs that failed fetch as a console
		// error. That is the real, expected state; anything else is a problem.
		expect(problems.filter((p) => !/503|Failed to load resource/.test(p))).toEqual([]);
	});

	test('the shell only talks to /healthz and /readyz and static assets', async ({ page }) => {
		const hosts: string[] = [];
		page.on('request', (r) => {
			const u = new URL(r.url());
			hosts.push(u.origin);
			if (u.pathname.startsWith('/api')) throw new Error(`unexpected API call ${u.pathname}`);
		});
		await page.goto('/');
		await page.getByRole('button', { name: 'Terminal' }).click();
		await page.waitForLoadState('networkidle');
		expect(new Set(hosts).size).toBe(1);
	});
});

test.describe('axe', () => {
	for (const path of ['/', '/servers', '/tasks', '/settings']) {
		test(`${path} has no violations`, async ({ page }) => {
			await page.goto(path);
			await page.getByRole('heading', { level: 1 }).waitFor();
			await expect(
				statusRow(page, 'ready')
					.or(page.getByRole('heading', { level: 2 }).first())
					.first()
			).toBeVisible();
			const r = await new AxeBuilder({ page }).withTags(TAGS).analyze();
			expect(
				r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
			).toEqual([]);
		});
	}

	test('error state and open terminal have no violations', async ({ page }) => {
		await mockReadyz(page, { status: 503, body: apiError('schema_ahead') });
		await page.goto('/settings');
		await expect(statusRow(page, 'ready')).toContainText('Niegotowy');
		await page.getByRole('button', { name: 'Terminal' }).click();
		await page.locator('.xterm-rows').getByText('skr >').first().waitFor();
		const r = await new AxeBuilder({ page }).withTags(TAGS).analyze();
		expect(
			r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
		).toEqual([]);
	});

	test('360px with the menu open has no violations', async ({ page }) => {
		await page.setViewportSize({ width: 360, height: 800 });
		await page.goto('/');
		await page.getByRole('button', { name: 'Menu' }).click();
		const r = await new AxeBuilder({ page }).withTags(TAGS).analyze();
		expect(
			r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
		).toEqual([]);
	});
});
