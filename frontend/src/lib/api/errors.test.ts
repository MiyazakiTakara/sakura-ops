import { describe, expect, it } from 'vitest';
import { isApiError, isHealth, isReadiness, parseApiError, readApiError } from './errors';

// Shared with the Go tests (internal/api/contract_test.go): same files, same expectations.
const raw = import.meta.glob<string>('../../../../api/fixtures/*.json', {
	eager: true,
	query: '?raw',
	import: 'default'
});
const fixtures = Object.entries(raw).map(([path, text]) => ({
	name: path.slice(path.lastIndexOf('/') + 1),
	text
}));

const validators: Record<string, (v: unknown) => boolean> = {
	error: isApiError,
	health: isHealth
};

describe('shared contract fixtures', () => {
	it('finds valid and invalid fixtures', () => {
		expect(fixtures.some((f) => f.name.includes('.valid.'))).toBe(true);
		expect(fixtures.some((f) => f.name.includes('.invalid.'))).toBe(true);
	});

	for (const f of fixtures) {
		const [schema, verdict] = f.name.split('.');
		it(`${f.name} is ${verdict}`, () => {
			const check = validators[schema ?? ''];
			expect(check, `no validator for ${f.name}`).toBeDefined();
			expect(['valid', 'invalid']).toContain(verdict);
			const value: unknown = JSON.parse(f.text);
			expect(check?.(value)).toBe(verdict === 'valid');
		});
	}
});

describe('parseApiError', () => {
	it('returns the envelope unchanged when valid', () => {
		const v = { code: 'not_found', message: 'No.', request_id: 'r' };
		expect(parseApiError(v)).toEqual(v);
	});

	it.each([null, undefined, 'x', 3, [], {}, { code: 'c' }])('rejects %j', (v) => {
		expect(parseApiError(v)).toBeNull();
	});
});

describe('readApiError', () => {
	it('parses a JSON error response', async () => {
		const res = new Response(JSON.stringify({ code: 'c', message: 'm', request_id: '' }), {
			status: 404
		});
		expect(await readApiError(res)).toEqual({ code: 'c', message: 'm', request_id: '' });
	});

	it('returns null for HTML and for malformed JSON', async () => {
		expect(await readApiError(new Response('<html></html>', { status: 502 }))).toBeNull();
		expect(await readApiError(new Response('{', { status: 500 }))).toBeNull();
	});

	it('returns null for JSON that does not match the contract', async () => {
		expect(await readApiError(new Response('{"error":"x"}', { status: 500 }))).toBeNull();
	});
});

describe('isReadiness', () => {
	it('accepts the contract shape', () => {
		expect(isReadiness({ status: 'ready', schema_version: 3 })).toBe(true);
	});
	it('rejects other shapes', () => {
		expect(isReadiness({ status: 'ready' })).toBe(false);
		expect(isReadiness({ status: 'ok', schema_version: 3 })).toBe(false);
		expect(isReadiness({ status: 'ready', schema_version: 1.5 })).toBe(false);
		expect(isReadiness({ status: 'ready', schema_version: 1, extra: 1 })).toBe(false);
		expect(isReadiness(null)).toBe(false);
	});
});
