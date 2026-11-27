import { expect, test } from '@playwright/test';
import { lastLine, openSpike, run, termInput, termRows, termText, watchPage } from './helpers';

test.describe('skr terminal input', () => {
	test.beforeEach(async ({ page }) => {
		await openSpike(page);
	});

	test('shows the skr > prompt', async ({ page }) => {
		const rows = await termRows(page);
		expect(rows.some((r) => r === 'skr >')).toBe(true);
	});

	test('help lists the commands', async ({ page }) => {
		await run(page, 'help');
		await expect.poll(() => termText(page)).toContain('Available commands (local spike):');
		const t = await termText(page);
		for (const c of ['help', 'clear', 'show version']) expect(t).toContain(`  ${c}`);
	});

	test('Up recalls the previous command', async ({ page }) => {
		await run(page, 'show version');
		await expect
			.poll(() => termText(page))
			.toContain('SakuraOps Next technical spike (no backend)');
		await page.keyboard.press('ArrowUp');
		await expect.poll(() => lastLine(page)).toMatch(/^skr > show version$/);
	});

	test('Tab completes a unique prefix', async ({ page }) => {
		await termInput(page).focus();
		await page.keyboard.type('sh');
		await page.keyboard.press('Tab');
		await expect.poll(() => lastLine(page)).toBe('skr > show version');
	});

	test('Tab on an empty or ambiguous line lists candidates; no match changes nothing', async ({
		page
	}) => {
		await termInput(page).focus();
		await page.keyboard.press('Tab');
		await expect.poll(() => termText(page)).toContain('help  clear  show version');

		await page.keyboard.type('x');
		await page.keyboard.press('Tab');
		await expect.poll(() => lastLine(page)).toBe('skr > x');
	});

	test('Ctrl+L clears the screen', async ({ page }) => {
		await run(page, 'help');
		await expect.poll(() => termText(page)).toContain('Available commands');
		await termInput(page).focus();
		await page.keyboard.press('Control+L');
		await expect.poll(() => termText(page)).not.toContain('Available commands');
		expect(await termText(page)).not.toContain('Technical spike');
		expect(await termText(page)).toContain('skr >');
	});

	test('unknown command prints the no-backend message and never echoes the input', async ({
		page
	}) => {
		await run(page, 'zzqq-secret-marker');
		await expect.poll(() => termText(page)).toContain('Unknown command (spike: no backend)');
		const out = (await termRows(page)).filter((r) => !r.startsWith('skr >')).join('\n');
		expect(out).not.toContain('zzqq-secret-marker');
	});

	test('pasted ESC sequences are not interpreted as colour or control', async ({ page }) => {
		await termInput(page).focus();
		await termInput(page).evaluate((el) => {
			const dt = new DataTransfer();
			dt.setData('text/plain', '\x1b[31mRED\x1b[0m\x1b[2J');
			el.dispatchEvent(
				new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true })
			);
		});
		await expect.poll(() => lastLine(page)).toBe('skr > RED');
		// No red foreground (palette colour 1) was applied to the pasted text, and the screen
		// was not erased by the pasted 2J: the banner is still there.
		await expect(page.locator('.xterm-rows .xterm-fg-1')).toHaveCount(0);
		expect(await termText(page)).toContain('Technical spike');
	});
});

test('a line longer than the terminal width redraws without leftovers', async ({ page }) => {
	await openSpike(page);
	await termInput(page).focus();
	const long = 'a'.repeat(120);
	await page.keyboard.type(long);
	await expect.poll(() => termText(page)).toContain('skr > aaaa');
	await page.keyboard.press('Control+U');
	// After clearing the line only the prompt may remain: no wrapped fragments of 'a's.
	await expect.poll(async () => (await termText(page)).includes('aaaa')).toBe(false);
	expect(await lastLine(page)).toBe('skr >');
});

test('terminal makes no network requests besides same-origin static assets', async ({ page }) => {
	const urls: string[] = [];
	page.on('request', (r) => urls.push(r.url()));
	const origin = 'http://127.0.0.1:18181';
	await openSpike(page);
	await run(page, 'help');
	await run(page, 'show version');
	await run(page, 'whatever');
	await expect.poll(() => termText(page)).toContain('Unknown command');
	for (const u of urls) {
		expect(u.startsWith(`${origin}/`), u).toBe(true);
		const p = new URL(u).pathname;
		expect(
			p === '/spike/terminal' ||
				p.startsWith('/_app/') ||
				p === '/favicon.png' ||
				p === '/favicon.ico',
			u
		).toBe(true);
	}
	expect(urls.some((u) => u.includes('/api'))).toBe(false);
});

test('spike page has no console errors and no CSP violations', async ({ page }) => {
	const problems = await watchPage(page);
	await openSpike(page);
	await run(page, 'help');
	await page.waitForLoadState('networkidle');
	expect(problems).toEqual([]);
});
