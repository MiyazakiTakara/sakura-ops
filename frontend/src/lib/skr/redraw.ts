// Terminal escape sequences that repaint the prompt line when it wraps over several rows.
// Pure functions: no DOM, no xterm. Widths are counted in code points (the spike line editor
// accepts any printable character but double-width glyphs are not measured).

export interface RedrawState {
	/** Rows below the first row of the prompt line where the cursor currently is. */
	cursorRow: number;
	/** Rows below the first row of the prompt line where the drawn text ends. */
	endRow: number;
}

export const INITIAL_REDRAW: RedrawState = { cursorRow: 0, endRow: 0 };

export interface RedrawInput {
	promptAnsi: string;
	promptWidth: number;
	line: string;
	cursor: number; // in code points
	cols: number;
}

export function redrawSequence(input: RedrawInput, prev: RedrawState): [string, RedrawState] {
	const { promptAnsi, promptWidth, line, cursor, cols } = input;
	const width = Math.max(1, cols);
	const len = Array.from(line).length;
	let out = '';
	if (prev.cursorRow > 0) out += `\x1b[${prev.cursorRow}A`;
	out += `\r\x1b[J${promptAnsi}${line}`;

	const total = promptWidth + len;
	let endRow = total > 0 ? Math.floor((total - 1) / width) : 0;
	const target = promptWidth + cursor;
	const row = Math.floor(target / width);
	const col = target % width;
	if (row > endRow) {
		// The text fills its last row exactly and the cursor belongs on the next one.
		out += '\r\n';
		endRow = row;
	} else if (endRow > row) {
		out += `\x1b[${endRow - row}A`;
	}
	out += '\r';
	if (col > 0) out += `\x1b[${col}C`;
	return [out, { cursorRow: row, endRow }];
}

/** Moves from the cursor row to the last row of the drawn line; call before writing output below it. */
export function toLineEnd(prev: RedrawState): [string, RedrawState] {
	const down = prev.endRow - prev.cursorRow;
	return [down > 0 ? `\x1b[${down}B` : '', { cursorRow: prev.endRow, endRow: prev.endRow }];
}
