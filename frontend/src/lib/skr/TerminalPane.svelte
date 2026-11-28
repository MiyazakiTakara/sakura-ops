<script lang="ts">
	import { FitAddon } from '@xterm/addon-fit';
	import { Terminal } from '@xterm/xterm';
	import '@xterm/xterm/css/xterm.css';
	import { onMount } from 'svelte';
	import { runLocalCommand, SPIKE_COMMANDS } from './commands';
	import { LineEditor, PROMPT, type Effect } from './lineEditor';
	import { INITIAL_REDRAW, redrawSequence, toLineEnd, type RedrawState } from './redraw';

	let host: HTMLDivElement;
	let instance: Terminal | undefined;

	/** Moves keyboard focus into the terminal input. */
	export function focus(): void {
		instance?.focus();
	}

	// Prompt colour is the DESIGN.md "success" token (#98CFA8).
	const PROMPT_ANSI = `\x1b[38;2;152;207;168m${PROMPT}\x1b[0m`;
	const CLEAR_SCREEN = '\x1b[2J\x1b[3J\x1b[H';

	onMount(() => {
		const term = new Terminal({
			convertEol: false,
			cursorBlink: true,
			fontFamily: 'ui-monospace, "Cascadia Mono", "DejaVu Sans Mono", monospace',
			fontSize: 14,
			scrollback: 500,
			theme: { background: '#101820', foreground: '#f0f2f5', cursor: '#d9a0b7' }
		});
		instance = term;
		const fit = new FitAddon();
		term.loadAddon(fit);
		term.open(host);
		fit.fit();

		const editor = new LineEditor(SPIKE_COMMANDS);

		let drawn: RedrawState = INITIAL_REDRAW;

		const redraw = (): void => {
			const [seq, next] = redrawSequence(
				{
					promptAnsi: PROMPT_ANSI,
					promptWidth: Array.from(PROMPT).length,
					line: editor.line,
					cursor: editor.cursor,
					cols: term.cols
				},
				drawn
			);
			drawn = next;
			term.write(seq);
		};

		// Output goes below the (possibly wrapped) prompt line; afterwards the cursor is at column 0
		// of a fresh row and the next redraw starts there.
		const below = (): void => {
			const [seq] = toLineEnd(drawn);
			term.write(seq);
			drawn = INITIAL_REDRAW;
		};

		const printLines = (lines: string[]): void => {
			below();
			for (const l of lines) term.write(`\r\n${l}`);
			term.write('\r\n');
		};

		const apply = (effects: Effect[]): void => {
			for (const e of effects) {
				switch (e.type) {
					case 'redraw':
						redraw();
						break;
					case 'bell':
						break;
					case 'print':
						printLines(e.lines);
						redraw();
						break;
					case 'clear':
						term.write(CLEAR_SCREEN);
						drawn = INITIAL_REDRAW;
						redraw();
						break;
					case 'submit': {
						const result = runLocalCommand(e.line);
						if (result.kind === 'clear') {
							term.write(CLEAR_SCREEN);
							drawn = INITIAL_REDRAW;
						} else {
							below();
							term.write('\r\n');
							for (const l of result.lines) term.write(`${l}\r\n`);
						}
						redraw();
						break;
					}
				}
			}
		};

		term.write('Technical spike: local echo only, no backend. Type "help".\r\n');
		redraw();
		const sub = term.onData((data) => apply(editor.feed(data)));

		const ro = new ResizeObserver(() => fit.fit());
		ro.observe(host);

		return () => {
			ro.disconnect();
			sub.dispose();
			instance = undefined;
			term.dispose();
		};
	});
</script>

<div
	class="term"
	bind:this={host}
	role="group"
	aria-label="skr terminal (local spike). Press Escape or Control+] to leave the terminal."
></div>

<style>
	.term {
		flex: 1;
		min-width: 0;
		padding: var(--space-2);
		background: var(--terminal);
		overflow: hidden;
	}
</style>
