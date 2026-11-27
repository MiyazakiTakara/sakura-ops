// Pure line-editing state machine for the technical spike terminal.
// No DOM, no xterm, no network: it turns raw terminal input into effects.

export const MAX_LINE_LENGTH = 512;
export const MAX_HISTORY = 100;
export const PROMPT = 'skr > ';

export type Effect =
	| { type: 'redraw' }
	| { type: 'submit'; line: string }
	| { type: 'clear' }
	| { type: 'print'; lines: string[] }
	| { type: 'bell' };

// Any control, format (bidi, zero-width), private-use or unassigned code point.
const NON_PRINTABLE = /\p{C}/u;

export class LineEditor {
	private chars: string[] = [];
	private pos = 0;
	private history: string[] = [];
	private historyIndex: number | null = null;
	private draft = '';

	constructor(private readonly commands: readonly string[]) {}

	get line(): string {
		return this.chars.join('');
	}

	get cursor(): number {
		return this.pos;
	}

	get historyEntries(): readonly string[] {
		return this.history;
	}

	/** Feed one chunk of terminal input (a key, an escape sequence or pasted text). */
	feed(data: string): Effect[] {
		const effects: Effect[] = [];
		const cps = Array.from(data);
		let i = 0;
		while (i < cps.length) {
			const ch = cps[i] as string;
			if (ch === '\x1b') {
				i = this.consumeEscape(cps, i, effects);
				continue;
			}
			i++;
			this.handleChar(ch, effects);
		}
		return effects;
	}

	private handleChar(ch: string, out: Effect[]): void {
		switch (ch) {
			case '\r':
			case '\n':
				return this.submit(out);
			case '\x7f':
			case '\b':
				return this.backspace(out);
			case '\t':
				return this.complete(out);
			case '\x0c':
				out.push({ type: 'clear' });
				return;
			case '\x03': // Ctrl+C: abandon the current line
				this.chars = [];
				this.pos = 0;
				this.historyIndex = null;
				out.push({ type: 'print', lines: ['^C'] });
				return;
			case '\x01':
				this.pos = 0;
				out.push({ type: 'redraw' });
				return;
			case '\x05':
				this.pos = this.chars.length;
				out.push({ type: 'redraw' });
				return;
			case '\x15':
				this.chars.splice(0, this.pos);
				this.pos = 0;
				out.push({ type: 'redraw' });
				return;
		}
		if (NON_PRINTABLE.test(ch)) {
			return; // other control/format characters are dropped, never echoed
		}
		if (this.chars.length >= MAX_LINE_LENGTH) {
			out.push({ type: 'bell' });
			return;
		}
		this.chars.splice(this.pos, 0, ch);
		this.pos++;
		this.historyIndex = null;
		out.push({ type: 'redraw' });
	}

	/** Consumes an escape sequence starting at cps[start]; returns the next index. */
	private consumeEscape(cps: string[], start: number, out: Effect[]): number {
		let i = start + 1;
		const next = cps[i];
		if (next === undefined) {
			return i;
		}
		if (next !== '[' && next !== 'O') {
			return i + 1; // unknown two-character escape: dropped
		}
		i++;
		let params = '';
		while (i < cps.length && /[0-9;]/.test(cps[i] as string) && params.length < 16) {
			params += cps[i];
			i++;
		}
		const final = cps[i];
		if (final === undefined) {
			return i;
		}
		i++;
		const key = params + final;
		switch (key) {
			case 'A':
				this.historyMove(-1, out);
				break;
			case 'B':
				this.historyMove(1, out);
				break;
			case 'C':
				if (this.pos < this.chars.length) this.pos++;
				out.push({ type: 'redraw' });
				break;
			case 'D':
				if (this.pos > 0) this.pos--;
				out.push({ type: 'redraw' });
				break;
			case 'H':
			case '1~':
				this.pos = 0;
				out.push({ type: 'redraw' });
				break;
			case 'F':
			case '4~':
				this.pos = this.chars.length;
				out.push({ type: 'redraw' });
				break;
			case '3~':
				if (this.pos < this.chars.length) {
					this.chars.splice(this.pos, 1);
					out.push({ type: 'redraw' });
				}
				break;
			default:
				break; // unknown sequences are consumed and ignored
		}
		return i;
	}

	private backspace(out: Effect[]): void {
		if (this.pos === 0) return;
		this.chars.splice(this.pos - 1, 1);
		this.pos--;
		this.historyIndex = null;
		out.push({ type: 'redraw' });
	}

	private submit(out: Effect[]): void {
		const line = this.line;
		this.chars = [];
		this.pos = 0;
		this.historyIndex = null;
		this.draft = '';
		const trimmed = line.trim();
		if (trimmed !== '' && this.history[this.history.length - 1] !== line) {
			this.history.push(line);
			if (this.history.length > MAX_HISTORY) this.history.shift();
		}
		out.push({ type: 'submit', line });
	}

	private historyMove(dir: -1 | 1, out: Effect[]): void {
		if (this.history.length === 0) {
			out.push({ type: 'bell' });
			return;
		}
		if (dir === -1) {
			if (this.historyIndex === null) {
				this.draft = this.line;
				this.historyIndex = this.history.length - 1;
			} else if (this.historyIndex > 0) {
				this.historyIndex--;
			} else {
				out.push({ type: 'bell' });
				return;
			}
		} else {
			if (this.historyIndex === null) {
				out.push({ type: 'bell' });
				return;
			}
			if (this.historyIndex < this.history.length - 1) {
				this.historyIndex++;
			} else {
				this.historyIndex = null;
				this.setLine(this.draft);
				out.push({ type: 'redraw' });
				return;
			}
		}
		this.setLine(this.history[this.historyIndex as number] as string);
		out.push({ type: 'redraw' });
	}

	private setLine(text: string): void {
		this.chars = Array.from(text).slice(0, MAX_LINE_LENGTH);
		this.pos = this.chars.length;
	}

	private complete(out: Effect[]): void {
		if (this.pos !== this.chars.length) {
			out.push({ type: 'bell' });
			return;
		}
		const prefix = this.line;
		const matches = this.commands.filter((c) => c.startsWith(prefix));
		if (matches.length === 0) {
			out.push({ type: 'bell' });
			return;
		}
		const common = longestCommonPrefix(matches);
		if (matches.length === 1 || common.length > prefix.length) {
			this.setLine(common);
			this.historyIndex = null;
			out.push({ type: 'redraw' });
			return;
		}
		out.push({ type: 'print', lines: [matches.join('  ')] });
	}
}

export function longestCommonPrefix(words: readonly string[]): string {
	if (words.length === 0) return '';
	let prefix = words[0] as string;
	for (const w of words) {
		while (!w.startsWith(prefix)) prefix = prefix.slice(0, -1);
	}
	return prefix;
}
