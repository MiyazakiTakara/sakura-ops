import { expect, test } from '@playwright/test';
import { handle, openSpike, termInput, titlebar, win } from './helpers';

test.describe('keyboard only', () => {
	test.beforeEach(async ({ page }) => {
		await openSpike(page);
	});

	test('Tab reaches the editor, the window controls, the terminal and the resize handle', async ({
		page
	}) => {
		const editor = page.getByRole('textbox', { name: 'Spike task YAML editor' });
		await page.locator('body').focus();
		await page.keyboard.press('Tab');
		await expect(editor).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(titlebar(page)).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(page.getByRole('button', { name: 'Minimize' })).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(page.getByRole('button', { name: 'Close' })).toBeFocused();
		await page.keyboard.press('Tab');
		await expect(termInput(page)).toBeFocused();
	});

	test('the terminal does not trap keyboard focus: Escape leaves it', async ({ page }) => {
		// xterm sends Tab to the line editor for completion, so Escape is the way out (WCAG 2.1.2):
		// focus goes to the window title bar, from where Tab and Shift+Tab continue normally.
		await termInput(page).focus();
		await page.keyboard.press('Escape');
		await expect(titlebar(page)).toBeFocused();
		await page.keyboard.press('Shift+Tab');
		await expect(page.getByRole('textbox', { name: 'Spike task YAML editor' })).toBeFocused();
		await titlebar(page).focus();
		await page.keyboard.press('Tab');
		await page.keyboard.press('Tab');
		await page.keyboard.press('Tab');
		await expect(termInput(page)).toBeFocused();
	});

	test('Escape does not close or minimize the window', async ({ page }) => {
		await titlebar(page).focus();
		await page.keyboard.press('Escape');
		await expect(titlebar(page)).toBeFocused();
		await termInput(page).focus();
		await page.keyboard.press('Escape');
		await expect(win(page)).toBeVisible();
		await expect(page.locator('.xterm')).toBeVisible();
	});

	test('window controls work with Enter and Space', async ({ page }) => {
		await page.getByRole('button', { name: 'Minimize' }).focus();
		await page.keyboard.press('Enter');
		await expect(page.getByRole('button', { name: 'Restore' })).toBeFocused();
		await page.keyboard.press('Space');
		await expect(page.getByRole('button', { name: 'Minimize' })).toBeFocused();
		await expect(handle(page)).toBeVisible();
	});
});
