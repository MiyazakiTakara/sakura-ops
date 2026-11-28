// Pure state logic for the instance status panel. A probe row is derived from the last
// completed check and the last successful one, so a failed refresh never turns old data into
// "current" data (DESIGN.md: stale must not look like fresh).
import type { CallResult } from '../api/system';

/** How often the shell re-checks, and when a success is considered too old to call current. */
export const POLL_INTERVAL_MS = 30_000;
export const STALE_AFTER_MS = POLL_INTERVAL_MS * 2;

export interface Probe<T> {
	/** Latest completed check; null until the first one finishes. */
	latest: { result: CallResult<T>; at: number } | null;
	/** Latest successful check; kept across failures. */
	lastOk: { data: T; at: number } | null;
}

export function emptyProbe<T>(): Probe<T> {
	return { latest: null, lastOk: null };
}

export function recordCheck<T>(prev: Probe<T>, result: CallResult<T>, at: number): Probe<T> {
	return {
		latest: { result, at },
		lastOk: result.ok ? { data: result.data, at } : prev.lastOk
	};
}

export type RowState<T> =
	| { kind: 'loading' }
	| { kind: 'ok'; data: T; at: number }
	/** The server answered and said it is not OK (it is reachable). */
	| { kind: 'refused'; status: number; code: string; requestId: string; at: number }
	/** No usable answer. `last` is older data, shown only as stale. */
	| { kind: 'unreachable'; status?: number; at: number; last: { data: T; at: number } | null }
	/** Last success is older than STALE_AFTER_MS and no newer check has completed. */
	| { kind: 'stale'; data: T; at: number };

export function deriveRow<T>(p: Probe<T>, now: number): RowState<T> {
	if (!p.latest) return { kind: 'loading' };
	const { result, at } = p.latest;
	if (result.ok) {
		return now - at > STALE_AFTER_MS
			? { kind: 'stale', data: result.data, at }
			: { kind: 'ok', data: result.data, at };
	}
	if (result.kind === 'api') {
		return {
			kind: 'refused',
			status: result.status,
			code: result.error.code,
			requestId: result.error.request_id,
			at
		};
	}
	return { kind: 'unreachable', status: result.status, at, last: p.lastOk };
}

export interface Explanation {
	/** Short, user-facing cause. */
	summary: string;
	/** A concrete next step the operator can take. */
	next: string;
}

/** Operator text for the `code` values documented for GET /readyz 503. Unknown codes fail safe. */
export function explainReadiness(code: string): Explanation {
	switch (code) {
		case 'db_not_configured':
			return {
				summary: 'Hub działa bez skonfigurowanej bazy danych.',
				next: 'Skonfiguruj połączenie z PostgreSQL i uruchom hub ponownie (docs/dev/database.md).'
			};
		case 'db_unavailable':
			return {
				summary: 'Baza danych nie odpowiada.',
				next: 'Sprawdź, czy PostgreSQL działa i jest osiągalny, potem wybierz Sprawdź teraz.'
			};
		case 'schema_outdated':
			return {
				summary: 'Schemat bazy jest starszy niż wymaga ta wersja huba.',
				next: 'Uruchom migracje (docs/dev/database.md), potem wybierz Sprawdź teraz.'
			};
		case 'schema_ahead':
			return {
				summary: 'Schemat bazy pochodzi z nowszego wydania niż ten hub.',
				next: 'Uruchom hub w wersji zgodnej ze schematem. Nie cofaj migracji bez kopii zapasowej.'
			};
		default:
			return {
				summary: 'Hub zgłosił nieznany powód braku gotowości.',
				next: 'Sprawdź logi huba, używając identyfikatora żądania z szczegółów.'
			};
	}
}

/** Local time of day, 24 h, for "last checked" text. */
export function formatTime(at: number): string {
	const d = new Date(at);
	const p = (n: number): string => String(n).padStart(2, '0');
	return `${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}
