import { expect, test } from '@playwright/test';
import { box, handle, openSpike, titlebar, win } from './helpers';

const VIEWPORTS = [
	{ width: 360, height: 640 },
	{ width: 768, height: 1024 },
	{ width: 1280, height: 800 },
	{ width: 1920, height: 1080 }
] as const;

async function noGlobalHScroll(page: import('@playwright/test').Page): Promise<void> {
	const m = await page.evaluate(() => ({
		scroll: document.documentElement.scrollWidth,
		inner: window.innerWidth
	}));
	expect(m.scroll).toBeLessThanOrEqual(m.inner);
}

async function fitsViewport(page: import('@playwright/test').Page): Promise<void> {
	const vp = page.viewportSize()!;
	const b = await box(win(page));
	expect(b.x).toBeGreaterThanOrEqual(0);
	expect(b.y).toBeGreaterThanOrEqual(0);
	expect(b.x + b.width).toBeLessThanOrEqual(vp.width);
	expect(b.y + b.height).toBeLessThanOrEqual(vp.height);
}

test.describe('viewports', () => {
	for (const vp of VIEWPORTS) {
		test(`${vp.width}px: no global horizontal scroll and the window fits`, async ({ page }) => {
			await page.setViewportSize(vp);
			await openSpike(page);
			await noGlobalHScroll(page);
			await fitsViewport(page);
			await expect(page.getByRole('button', { name: 'Minimize' })).toBeInViewport();
			await expect(page.getByRole('button', { name: 'Close' })).toBeInViewport();
			// Below 768 px the window is full-screen and has no resize handle.
			if (vp.width < 768) await expect(handle(page)).toHaveCount(0);
			else await expect(handle(page)).toBeInViewport();
		});
	}

	// DESIGN.md: on a phone (< 768 px) the terminal uses the full available area.
	test('360px: the terminal window uses the full available area', async ({ page }) => {
		await page.setViewportSize({ width: 360, height: 640 });
		await openSpike(page);
		const b = await box(win(page));
		expect(b.x).toBe(0);
		expect(b.y).toBe(0);
		expect(b.width).toBe(360);
		expect(b.height).toBe(640);
	});

	test('shrinking the viewport pulls the window back inside', async ({ page }) => {
		await page.setViewportSize({ width: 1280, height: 800 });
		await openSpike(page);
		await page.setViewportSize({ width: 600, height: 500 });
		await fitsViewport(page);
		await noGlobalHScroll(page);
	});
});

// Browsers have no API for page zoom. 200% zoom (WCAG 1.4.4/1.4.10) is emulated the way
// it affects layout: the CSS viewport is halved and the device scale factor doubled.
// A 1280x720 desktop at 200% is a 640x360 CSS viewport at 2x pixel density.
test.describe('200% zoom (emulated)', () => {
	test.use({ viewport: { width: 640, height: 360 }, deviceScaleFactor: 2 });

	test('no horizontal overflow and all window controls stay reachable', async ({ page }) => {
		await openSpike(page);
		expect(await page.evaluate(() => window.devicePixelRatio)).toBe(2);
		await noGlobalHScroll(page);
		await fitsViewport(page);
		for (const l of [
			titlebar(page),
			page.getByRole('button', { name: 'Minimize' }),
			page.getByRole('button', { name: 'Close' })
		]) {
			await expect(l).toBeInViewport({ ratio: 1 });
		}
		// Reachable means usable: operate them.
		await page.getByRole('button', { name: 'Minimize' }).click();
		await page.getByRole('button', { name: 'Restore' }).click();
		// 640 CSS px is below the 768 px phone breakpoint: full-screen window, no handle.
		await expect(handle(page)).toHaveCount(0);
		await fitsViewport(page);
	});
});
