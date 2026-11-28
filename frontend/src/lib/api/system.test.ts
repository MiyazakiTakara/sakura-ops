import { describe, expect, it } from 'vitest';
import { getHealth, getReadiness, type Fetcher } from './system';

const json =
	(status: number, body: unknown): Fetcher =>
	async () =>
		new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });

describe('system client', () => {
	it('getHealth accepts the contract body', async () => {
		expect(await getHealth(json(200, { status: 'ok' }))).toEqual({
			ok: true,
			data: { status: 'ok' }
		});
	});

	it('getReadiness returns the error envelope for 503', async () => {
		const env = { code: 'db_unavailable', message: 'm', request_id: 'r' };
		expect(await getReadiness(json(503, env))).toEqual({
			ok: false,
			kind: 'api',
			status: 503,
			error: env
		});
	});

	it('a 200 with an off-contract body is unreachable, not ok', async () => {
		const r = await getReadiness(json(200, { status: 'ready' }));
		expect(r).toEqual({ ok: false, kind: 'unreachable', status: 200 });
	});

	it('a non-JSON 502 is unreachable and its body is never exposed', async () => {
		const f: Fetcher = async () => new Response('<html>proxy secret</html>', { status: 502 });
		const r = await getHealth(f);
		expect(r).toEqual({ ok: false, kind: 'unreachable', status: 502 });
	});

	it('network failure is unreachable', async () => {
		const f: Fetcher = async () => {
			throw new TypeError('failed');
		};
		expect(await getHealth(f)).toEqual({ ok: false, kind: 'unreachable' });
	});

	it('sends GET only, no credentials in the call', async () => {
		let seen: RequestInit | undefined;
		const f: Fetcher = async (_u, init) => {
			seen = init;
			return new Response('{"status":"ok"}');
		};
		await getHealth(f);
		expect(seen?.method).toBe('GET');
	});
});
