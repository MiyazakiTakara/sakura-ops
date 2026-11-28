import { describe, expect, it } from 'vitest';
import type { Health, Readiness } from '../api/errors';
import type { CallResult } from '../api/system';
import { deriveRow, emptyProbe, explainReadiness, recordCheck, STALE_AFTER_MS } from './status';

const ok: CallResult<Readiness> = { ok: true, data: { status: 'ready', schema_version: 4 } };
const refused = (code: string): CallResult<Readiness> => ({
	ok: false,
	kind: 'api',
	status: 503,
	error: { code, message: 'x', request_id: 'req-1' }
});
const down: CallResult<Readiness> = { ok: false, kind: 'unreachable' };

describe('deriveRow', () => {
	it('is loading before the first check', () => {
		expect(deriveRow(emptyProbe<Health>(), 1000)).toEqual({ kind: 'loading' });
	});

	it('is ok right after a success', () => {
		const p = recordCheck(emptyProbe<Readiness>(), ok, 1000);
		expect(deriveRow(p, 1000 + 5)).toMatchObject({ kind: 'ok', at: 1000 });
	});

	it('ages a success into stale when no newer check completes', () => {
		const p = recordCheck(emptyProbe<Readiness>(), ok, 1000);
		expect(deriveRow(p, 1000 + STALE_AFTER_MS + 1).kind).toBe('stale');
	});

	it('a refusal from the server is reported as refused with code and request id', () => {
		const p = recordCheck(emptyProbe<Readiness>(), refused('schema_ahead'), 5);
		expect(deriveRow(p, 6)).toEqual({
			kind: 'refused',
			status: 503,
			code: 'schema_ahead',
			requestId: 'req-1',
			at: 5
		});
	});

	it('a failed refresh keeps the last success only as stale data, never as current', () => {
		let p = recordCheck(emptyProbe<Readiness>(), ok, 100);
		p = recordCheck(p, down, 200);
		const row = deriveRow(p, 201);
		expect(row.kind).toBe('unreachable');
		if (row.kind === 'unreachable') expect(row.last).toEqual({ data: ok.ok && ok.data, at: 100 });
	});

	it('unreachable without history has no last data', () => {
		const p = recordCheck(emptyProbe<Readiness>(), down, 1);
		const row = deriveRow(p, 2);
		expect(row.kind === 'unreachable' && row.last).toBeNull();
	});
});

describe('explainReadiness', () => {
	it.each(['db_not_configured', 'db_unavailable', 'schema_outdated', 'schema_ahead'])(
		'%s has a summary and a next step',
		(code) => {
			const e = explainReadiness(code);
			expect(e.summary.length).toBeGreaterThan(5);
			expect(e.next.length).toBeGreaterThan(5);
		}
	);

	it('unknown codes fail safe without echoing the code into the text', () => {
		const e = explainReadiness('<script>alert(1)</script>');
		expect(e.summary).not.toContain('<script>');
	});
});
