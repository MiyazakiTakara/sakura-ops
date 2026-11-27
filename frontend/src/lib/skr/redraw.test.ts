import { describe, expect, it } from 'vitest';
import { INITIAL_REDRAW, redrawSequence, toLineEnd } from './redraw';

const base = { promptAnsi: 'P', promptWidth: 6, cols: 10 };

describe('redrawSequence', () => {
	it('single row: carriage return, erase to end of screen, text, cursor column', () => {
		const [seq, st] = redrawSequence({ ...base, line: 'ab', cursor: 1 }, INITIAL_REDRAW);
		expect(seq).toBe('\r\x1b[JPab\r\x1b[7C');
		expect(st).toEqual({ cursorRow: 0, endRow: 0 });
	});

	it('wrapped line: tracks the end row and puts the cursor on the right row', () => {
		// 6 + 8 = 14 cells over 10 columns: two rows, cursor at the end (row 1, col 4).
		const [seq, st] = redrawSequence({ ...base, line: 'abcdefgh', cursor: 8 }, INITIAL_REDRAW);
		expect(seq).toBe('\r\x1b[JPabcdefgh\r\x1b[4C');
		expect(st).toEqual({ cursorRow: 1, endRow: 1 });
	});

	it('moves up to the first row before repainting and erases the old rows', () => {
		const [seq] = redrawSequence({ ...base, line: '', cursor: 0 }, { cursorRow: 2, endRow: 2 });
		expect(seq.startsWith('\x1b[2A\r\x1b[J')).toBe(true);
	});

	it('cursor in the middle of a wrapped line moves back up', () => {
		const [seq, st] = redrawSequence({ ...base, line: 'abcdefgh', cursor: 1 }, INITIAL_REDRAW);
		expect(seq).toBe('\r\x1b[JPabcdefgh\x1b[1A\r\x1b[7C');
		expect(st).toEqual({ cursorRow: 0, endRow: 1 });
	});

	it('text that fills the row exactly puts the cursor on a new row', () => {
		const [seq, st] = redrawSequence({ ...base, line: 'abcd', cursor: 4 }, INITIAL_REDRAW);
		expect(seq).toBe('\r\x1b[JPabcd\r\n\r');
		expect(st).toEqual({ cursorRow: 1, endRow: 1 });
	});
});

describe('toLineEnd', () => {
	it('moves down to the last drawn row', () => {
		expect(toLineEnd({ cursorRow: 0, endRow: 2 })).toEqual([
			'\x1b[2B',
			{ cursorRow: 2, endRow: 2 }
		]);
		expect(toLineEnd({ cursorRow: 1, endRow: 1 })[0]).toBe('');
	});
});
