import { describe, expect, it } from 'vitest';
import { isActive, NAV_ITEMS } from './nav';

describe('nav', () => {
	it('root is active only on /', () => {
		expect(isActive('/', '/')).toBe(true);
		expect(isActive('/', '/servers')).toBe(false);
	});
	it('sections match on segment boundaries', () => {
		expect(isActive('/servers', '/servers')).toBe(true);
		expect(isActive('/servers', '/servers/abc')).toBe(true);
		expect(isActive('/servers', '/serversx')).toBe(false);
	});
	it('has unique hrefs and no scheduling entry before it exists', () => {
		const hrefs = NAV_ITEMS.map((i) => i.href);
		expect(new Set(hrefs).size).toBe(hrefs.length);
		expect(hrefs).not.toContain('/schedules');
	});
});
