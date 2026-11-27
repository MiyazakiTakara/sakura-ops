import { expect, test } from '@playwright/test';
import { openSpike } from './helpers';

test.describe('CodeMirror panel', () => {
	test('shows the YAML and accepts keyboard typing', async ({ page }) => {
		await openSpike(page);
		const ed = page.getByRole('textbox', { name: 'Spike task YAML editor' });
		await expect(ed).toBeVisible();
		await expect(ed).toHaveAttribute('contenteditable', 'true');
		await expect(ed).toContainText('apiVersion: "sakuraops/v1"');
		await expect(ed).toContainText('Fictional example task');

		// The floating window overlays the page by design; minimize it to reach the editor.
		await page.getByRole('button', { name: 'Minimize' }).click();
		await ed.click();
		await page.keyboard.press('Control+End');
		await page.keyboard.type('# typed by test');
		await expect(ed).toContainText('# typed by test');
		await page.keyboard.press('Control+Z');
		await expect(ed).not.toContainText('# typed by test');
	});
});
