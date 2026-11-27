// Runtime narrowing of untrusted JSON into the types generated from the OpenAPI document
// (schema.d.ts). The checks mirror the schemas: required keys, string types, minLength
// where declared and additionalProperties: false.
import type { components } from './schema';

export type ApiError = components['schemas']['Error'];
export type FieldError = components['schemas']['FieldError'];
export type Health = components['schemas']['Health'];

function isRecord(v: unknown): v is Record<string, unknown> {
	return typeof v === 'object' && v !== null && !Array.isArray(v);
}

function onlyKeys(v: Record<string, unknown>, allowed: readonly string[]): boolean {
	return Object.keys(v).every((k) => allowed.includes(k));
}

function nonEmptyString(v: unknown): v is string {
	return typeof v === 'string' && v.length > 0;
}

export function isFieldError(v: unknown): v is FieldError {
	return (
		isRecord(v) &&
		onlyKeys(v, ['path', 'code', 'message']) &&
		nonEmptyString(v.path) &&
		nonEmptyString(v.code) &&
		typeof v.message === 'string'
	);
}

export function isApiError(v: unknown): v is ApiError {
	if (!isRecord(v) || !onlyKeys(v, ['code', 'message', 'request_id', 'field_errors'])) {
		return false;
	}
	if (!nonEmptyString(v.code) || typeof v.message !== 'string') return false;
	if (typeof v.request_id !== 'string') return false;
	if (v.field_errors !== undefined) {
		if (!Array.isArray(v.field_errors) || !v.field_errors.every(isFieldError)) return false;
	}
	return true;
}

export function isHealth(v: unknown): v is Health {
	return isRecord(v) && onlyKeys(v, ['status']) && v.status === 'ok';
}

/** Returns the error envelope, or null when the value does not match the contract. */
export function parseApiError(v: unknown): ApiError | null {
	return isApiError(v) ? v : null;
}

/**
 * Reads a failed response body as an error envelope. Anything that is not valid JSON or
 * does not match the contract yields null; callers must then show a generic failure and
 * never display the raw body.
 */
export async function readApiError(res: Response): Promise<ApiError | null> {
	try {
		return parseApiError(await res.json());
	} catch {
		return null;
	}
}
