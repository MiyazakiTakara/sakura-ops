// Navigation model of the shell. DESIGN.md lists Pulpit, Serwery, Zadania, Harmonogramy (only
// after they exist) and Ustawienia; Harmonogramy is therefore absent, not a dead link.
export interface NavItem {
	href: string;
	label: string;
}

export const NAV_ITEMS: readonly NavItem[] = [
	{ href: '/', label: 'Pulpit' },
	{ href: '/servers', label: 'Serwery' },
	{ href: '/tasks', label: 'Zadania' },
	{ href: '/settings', label: 'Ustawienia' }
];

/** Exact match for the root, prefix match (on a path segment boundary) for the rest. */
export function isActive(href: string, pathname: string): boolean {
	if (href === '/') return pathname === '/';
	return pathname === href || pathname.startsWith(`${href}/`);
}
