import type { Locator, Page } from '@playwright/test';

/** Rendered terminal rows as text (xterm DOM renderer), trailing blanks trimmed. */
export async function termRows(page: Page): Promise<string[]> {
	const rows = await page.locator('.xterm-rows > div').allInnerTexts();
	return rows.map((r) => r.replace(/ /g, ' ').trimEnd());
}

export async function termText(page: Page): Promise<string> {
	return (await termRows(page)).join('\n');
}

/** The line the cursor is on: the last non-empty row (the terminal has many blank rows). */
export async function lastLine(page: Page): Promise<string> {
	const rows = (await termRows(page)).filter((r) => r !== '');
	return rows.at(-1) ?? '';
}

export function termInput(page: Page): Locator {
	return page.locator('.xterm-helper-textarea');
}

/** Opens the spike page and waits until the terminal shows the first prompt. */
export async function openSpike(page: Page): Promise<void> {
	await page.goto('/spike/terminal');
	await page.getByRole('region', { name: 'skr (spike)' }).waitFor();
	await page.locator('.xterm-rows').getByText('skr >').first().waitFor();
}

export function win(page: Page): Locator {
	return page.getByRole('region', { name: 'skr (spike)' });
}

export function titlebar(page: Page): Locator {
	return page.getByRole('toolbar', { name: /window title bar/ });
}

export function handle(page: Page): Locator {
	return page.getByRole('slider', { name: /Resize window/ });
}

export async function box(
	l: Locator
): Promise<{ x: number; y: number; width: number; height: number }> {
	const b = await l.boundingBox();
	if (!b) throw new Error('element has no bounding box');
	return b;
}

/** Focuses the terminal and types a line, then presses Enter. */
export async function run(page: Page, line: string): Promise<void> {
	await termInput(page).focus();
	await page.keyboard.type(line);
	await page.keyboard.press('Enter');
}

/** Records CSP violations and console problems from before any page script runs. */
export async function watchPage(page: Page): Promise<string[]> {
	const problems: string[] = [];
	await page.exposeFunction('__cspReport', (m: string) => problems.push(`csp: ${m}`));
	await page.addInitScript(() => {
		document.addEventListener('securitypolicyviolation', (e) => {
			(window as unknown as { __cspReport: (m: string) => void }).__cspReport(
				`${e.violatedDirective} ${e.blockedURI}`
			);
		});
	});
	page.on('console', (m) => {
		if (m.type() === 'error' || m.type() === 'warning') problems.push(`${m.type()}: ${m.text()}`);
	});
	page.on('pageerror', (e) => problems.push(`pageerror: ${e.message}`));
	return problems;
}
