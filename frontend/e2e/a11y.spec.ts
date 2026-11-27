import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { openSpike } from './helpers';

// @axe-core/playwright is MPL-2.0 (dev dependency only, never shipped or linked into the product).
const TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa', 'best-practice'];

test('axe: root has no violations', async ({ page }) => {
	await page.goto('/');
	await page.getByRole('heading', { level: 1 }).waitFor();
	const r = await new AxeBuilder({ page }).withTags(TAGS).analyze();
	expect(
		r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
	).toEqual([]);
});

test('axe: spike terminal page has no violations', async ({ page }) => {
	await openSpike(page);
	const r = await new AxeBuilder({ page }).withTags(TAGS).analyze();
	expect(
		r.violations.map((v) => `${v.id}: ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`)
	).toEqual([]);
});
