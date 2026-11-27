import { expect, test } from '@playwright/test';
import { box, handle, openSpike, run, termText, titlebar, win } from './helpers';

test.describe('floating skr window', () => {
	test.beforeEach(async ({ page }) => {
		await openSpike(page);
	});

	test('is visible with a title', async ({ page }) => {
		await expect(win(page)).toBeVisible();
		await expect(win(page).getByRole('heading', { name: 'skr (spike)' })).toBeVisible();
	});

	test('drag by the title bar moves the window', async ({ page }) => {
		const before = await box(win(page));
		const t = await box(titlebar(page));
		const sx = t.x + 40;
		const sy = t.y + t.height / 2;
		await page.mouse.move(sx, sy);
		await page.mouse.down();
		await page.mouse.move(sx + 120, sy + 60, { steps: 5 });
		await page.mouse.up();
		const after = await box(win(page));
		expect(after.x - before.x).toBeCloseTo(120, 0);
		expect(after.y - before.y).toBeCloseTo(60, 0);
	});

	test('dragging past any edge keeps the whole window inside the viewport', async ({ page }) => {
		const vp = page.viewportSize()!;
		const drag = async (dx: number, dy: number): Promise<void> => {
			const t = await box(titlebar(page));
			const sx = t.x + 40;
			const sy = t.y + t.height / 2;
			await page.mouse.move(sx, sy);
			await page.mouse.down();
			await page.mouse.move(sx + dx, sy + dy, { steps: 5 });
			await page.mouse.up();
		};
		for (const [dx, dy] of [
			[5000, 0],
			[0, 5000],
			[-5000, 0],
			[0, -5000]
		] as const) {
			await drag(dx, dy);
			const b = await box(win(page));
			expect(b.x).toBeGreaterThanOrEqual(0);
			expect(b.y).toBeGreaterThanOrEqual(0);
			expect(b.x + b.width).toBeLessThanOrEqual(vp.width);
			expect(b.y + b.height).toBeLessThanOrEqual(vp.height);
		}
	});

	test('resize handle changes the size within bounds', async ({ page }) => {
		const vp = page.viewportSize()!;
		const before = await box(win(page));
		const h = await box(handle(page));
		const sx = h.x + h.width / 2;
		const sy = h.y + h.height / 2;
		await page.mouse.move(sx, sy);
		await page.mouse.down();
		await page.mouse.move(sx + 100, sy + 50, { steps: 5 });
		await page.mouse.up();
		const grown = await box(win(page));
		expect(grown.width - before.width).toBeCloseTo(100, 0);
		expect(grown.height - before.height).toBeCloseTo(50, 0);

		// Shrink far below the minimum, then grow far beyond the viewport.
		const h2 = await box(handle(page));
		await page.mouse.move(h2.x + 10, h2.y + 10);
		await page.mouse.down();
		await page.mouse.move(h2.x - 3000, h2.y - 3000, { steps: 5 });
		await page.mouse.up();
		const small = await box(win(page));
		expect(small.width).toBe(280);
		expect(small.height).toBe(160);

		const h3 = await box(handle(page));
		await page.mouse.move(h3.x + 10, h3.y + 10);
		await page.mouse.down();
		await page.mouse.move(h3.x + 5000, h3.y + 5000, { steps: 5 });
		await page.mouse.up();
		const big = await box(win(page));
		expect(big.x + big.width).toBeLessThanOrEqual(vp.width);
		expect(big.y + big.height).toBeLessThanOrEqual(vp.height);
	});

	test('keyboard: arrows on the title bar move the window', async ({ page }) => {
		const before = await box(win(page));
		await titlebar(page).focus();
		await page.keyboard.press('ArrowRight');
		await page.keyboard.press('ArrowDown');
		await page.keyboard.press('ArrowDown');
		const after = await box(win(page));
		expect(after.x - before.x).toBe(16);
		expect(after.y - before.y).toBe(32);
		await page.keyboard.press('ArrowLeft');
		expect((await box(win(page))).x).toBe(before.x);
	});

	test('keyboard: arrows on the resize handle change the size', async ({ page }) => {
		const before = await box(win(page));
		await handle(page).focus();
		await page.keyboard.press('ArrowRight');
		await page.keyboard.press('ArrowDown');
		const grown = await box(win(page));
		expect(grown.width - before.width).toBe(16);
		expect(grown.height - before.height).toBe(16);
		await page.keyboard.press('ArrowLeft');
		await page.keyboard.press('ArrowUp');
		const back = await box(win(page));
		expect(back.width).toBe(before.width);
		expect(back.height).toBe(before.height);
		await expect(handle(page)).toHaveAttribute('aria-valuenow', String(before.width));
	});

	test('minimize then restore preserves the terminal content', async ({ page }) => {
		await run(page, 'show version');
		await expect.poll(() => termText(page)).toContain('SakuraOps Next technical spike');
		const before = await termText(page);
		const full = await box(win(page));

		await page.getByRole('button', { name: 'Minimize' }).click();
		const small = await box(win(page));
		expect(small.height).toBeLessThan(full.height / 2);
		await expect(page.locator('.xterm')).toBeHidden();
		await expect(page.getByRole('button', { name: 'Restore' })).toHaveAttribute(
			'aria-pressed',
			'true'
		);

		await page.getByRole('button', { name: 'Restore' }).click();
		await expect(page.locator('.xterm')).toBeVisible();
		expect((await box(win(page))).height).toBeCloseTo(full.height, 0);
		await expect.poll(() => termText(page)).toBe(before);
	});

	test('close removes the window and the page offers to reopen a fresh one', async ({ page }) => {
		await run(page, 'help');
		await page.getByRole('button', { name: 'Close' }).click();
		await expect(win(page)).toHaveCount(0);
		await page.getByRole('button', { name: 'Reopen spike terminal' }).click();
		await expect(win(page)).toBeVisible();
		// Documented spike behaviour: closing destroys the terminal, so reopening starts clean.
		await page.locator('.xterm-rows').getByText('skr >').first().waitFor();
		expect(await termText(page)).not.toContain('Available commands');
	});

	test('keyboard focus is visible on the window controls', async ({ page }) => {
		for (const l of [
			titlebar(page),
			page.getByRole('button', { name: 'Minimize' }),
			page.getByRole('button', { name: 'Close' }),
			handle(page)
		]) {
			await l.focus();
			await expect(l).toBeFocused();
			const o = await l.evaluate((el) => {
				const s = getComputedStyle(el);
				return { style: s.outlineStyle, width: parseFloat(s.outlineWidth) };
			});
			expect(o.style).not.toBe('none');
			expect(o.width).toBeGreaterThanOrEqual(2);
		}
	});
});
