import { describe, expect, it } from 'vitest';
import { runLocalCommand, SPIKE_COMMANDS } from './commands';
import { LineEditor, MAX_HISTORY, MAX_LINE_LENGTH } from './lineEditor';

const UP = '\x1b[A';
const DOWN = '\x1b[B';
const LEFT = '\x1b[D';

function editor(): LineEditor {
	return new LineEditor(SPIKE_COMMANDS);
}

function submit(e: LineEditor, text: string) {
	return e.feed(text + '\r');
}

describe('editing', () => {
	it('inserts, moves the cursor and deletes', () => {
		const e = editor();
		e.feed('helo');
		e.feed(LEFT);
		e.feed('l');
		expect(e.line).toBe('hello');
		e.feed('\x7f');
		expect(e.line).toBe('helo');
		e.feed('\x01');
		expect(e.cursor).toBe(0);
		e.feed('\x1b[3~');
		expect(e.line).toBe('elo');
	});

	it('submits and resets the buffer', () => {
		const e = editor();
		const fx = submit(e, 'help');
		expect(fx).toContainEqual({ type: 'submit', line: 'help' });
		expect(e.line).toBe('');
	});

	it('Ctrl+L requests a clear without touching the buffer', () => {
		const e = editor();
		e.feed('abc');
		expect(e.feed('\x0c')).toEqual([{ type: 'clear' }]);
		expect(e.line).toBe('abc');
	});
});

describe('history', () => {
	it('navigates up and down and restores the draft', () => {
		const e = editor();
		submit(e, 'help');
		submit(e, 'show version');
		e.feed('dra');
		e.feed(UP);
		expect(e.line).toBe('show version');
		e.feed(UP);
		expect(e.line).toBe('help');
		e.feed(UP); // already at oldest
		expect(e.line).toBe('help');
		e.feed(DOWN);
		expect(e.line).toBe('show version');
		e.feed(DOWN);
		expect(e.line).toBe('dra');
		expect(e.feed(DOWN)).toEqual([{ type: 'bell' }]);
	});

	it('ignores blank lines and consecutive duplicates', () => {
		const e = editor();
		submit(e, '   ');
		submit(e, 'help');
		submit(e, 'help');
		expect(e.historyEntries).toEqual(['help']);
	});

	it('is bounded', () => {
		const e = editor();
		for (let i = 0; i < MAX_HISTORY + 20; i++) submit(e, `cmd${i}`);
		expect(e.historyEntries.length).toBe(MAX_HISTORY);
		expect(e.historyEntries[0]).toBe('cmd20');
	});

	it('Up with empty history only rings the bell', () => {
		expect(editor().feed(UP)).toEqual([{ type: 'bell' }]);
	});
});

describe('tab completion', () => {
	it('completes a unique match', () => {
		const e = editor();
		e.feed('he\t');
		expect(e.line).toBe('help');
		const e2 = editor();
		e2.feed('sh\t');
		expect(e2.line).toBe('show version');
	});

	it('lists candidates when ambiguous and nothing can be extended', () => {
		const e = editor();
		const fx = e.feed('\t');
		expect(e.line).toBe('');
		expect(fx).toEqual([{ type: 'print', lines: ['help  clear  show version'] }]);
	});

	it('extends to the longest common prefix when ambiguous', () => {
		const e = new LineEditor(['deploy', 'deposit']);
		e.feed('d\t');
		expect(e.line).toBe('dep');
	});

	it('does nothing on no match', () => {
		const e = editor();
		e.feed('zzz');
		expect(e.feed('\t')).toEqual([{ type: 'bell' }]);
		expect(e.line).toBe('zzz');
	});
});

describe('input hardening', () => {
	it('caps the line at the maximum length', () => {
		const e = editor();
		e.feed('a'.repeat(MAX_LINE_LENGTH + 100));
		expect(e.line.length).toBe(MAX_LINE_LENGTH);
	});

	it('drops control characters and unknown escape sequences', () => {
		const e = editor();
		e.feed('a\x00\x07\x1b\x1b[31mb\x1b]0;title\x07c‮​d');
		expect(e.line.includes('\x1b')).toBe(false);
		expect(e.line.includes('‮')).toBe(false);
		expect(e.line.includes('​')).toBe(false);
		expect(e.line).not.toMatch(/\p{C}/u);
	});

	it('never produces a submitted line containing control characters', () => {
		const e = editor();
		const fx = e.feed('x\x1b[2Jy\x9bz\r');
		const sub = fx.find((f) => f.type === 'submit');
		expect(sub && sub.type === 'submit' && sub.line).not.toMatch(/\p{C}/u);
	});

	it('survives a truncated escape sequence', () => {
		const e = editor();
		e.feed('ab\x1b[');
		expect(e.line).toBe('ab');
	});
});

describe('local commands', () => {
	it('help lists the commands', () => {
		const r = runLocalCommand('help');
		expect(r.kind).toBe('output');
		expect(r.kind === 'output' && r.lines.join('\n')).toContain('show version');
	});

	it('clear is a clear request', () => {
		expect(runLocalCommand('clear')).toEqual({ kind: 'clear' });
	});

	it('unknown input gives a fixed message and is not echoed', () => {
		const r = runLocalCommand('rm -rf /');
		expect(r).toEqual({ kind: 'output', lines: ['Unknown command (spike: no backend)'] });
	});
});
