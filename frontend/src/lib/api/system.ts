// Typed calls to the only endpoints that exist today: GET /healthz and GET /readyz
// (operations getHealth and getReadiness in api/openapi/sakuraops.v1.yaml). Responses are
// narrowed at runtime with the guards in errors.ts; nothing unvalidated reaches the UI.
import {
	isHealth,
	isReadiness,
	readApiError,
	type ApiError,
	type Health,
	type Readiness
} from './errors';

export type CallResult<T> =
	| { ok: true; data: T }
	/** The server answered with a contract error envelope (e.g. 503 from /readyz). */
	| { ok: false; kind: 'api'; status: number; error: ApiError }
	/** Network failure, timeout, or an answer that does not match the contract. */
	| { ok: false; kind: 'unreachable'; status?: number };

export type Fetcher = (input: string, init?: RequestInit) => Promise<Response>;

export const REQUEST_TIMEOUT_MS = 5000;

async function get<T>(
	path: string,
	guard: (v: unknown) => v is T,
	fetcher: Fetcher
): Promise<CallResult<T>> {
	let res: Response;
	try {
		res = await fetcher(path, {
			method: 'GET',
			headers: { Accept: 'application/json' },
			cache: 'no-store',
			signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS)
		});
	} catch {
		return { ok: false, kind: 'unreachable' };
	}
	if (res.ok) {
		try {
			const body: unknown = await res.json();
			return guard(body)
				? { ok: true, data: body }
				: { ok: false, kind: 'unreachable', status: res.status };
		} catch {
			return { ok: false, kind: 'unreachable', status: res.status };
		}
	}
	const error = await readApiError(res);
	return error
		? { ok: false, kind: 'api', status: res.status, error }
		: { ok: false, kind: 'unreachable', status: res.status };
}

export function getHealth(fetcher: Fetcher = fetch): Promise<CallResult<Health>> {
	return get('/healthz', isHealth, fetcher);
}

export function getReadiness(fetcher: Fetcher = fetch): Promise<CallResult<Readiness>> {
	return get('/readyz', isReadiness, fetcher);
}
