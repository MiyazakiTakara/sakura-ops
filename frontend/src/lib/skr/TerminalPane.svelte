<script lang="ts">
	import { FitAddon } from '@xterm/addon-fit';
	import { Terminal } from '@xterm/xterm';
	import '@xterm/xterm/css/xterm.css';
	import { onMount } from 'svelte';
	import { runLocalCommand, SPIKE_COMMANDS } from './commands';
	import { LineEditor, PROMPT, type Effect } from './lineEditor';

	let host: HTMLDivElement;

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
		const fit = new FitAddon();
		term.loadAddon(fit);
		term.open(host);
		fit.fit();

		const editor = new LineEditor(SPIKE_COMMANDS);

		const redraw = (): void => {
			const back = Array.from(editor.line).length - editor.cursor;
			term.write(`\r\x1b[2K${PROMPT_ANSI}${editor.line}${back > 0 ? `\x1b[${back}D` : ''}`);
		};

		const printLines = (lines: string[]): void => {
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
						redraw();
						break;
					case 'submit': {
						const result = runLocalCommand(e.line);
						if (result.kind === 'clear') {
							term.write(CLEAR_SCREEN);
						} else {
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
			term.dispose();
		};
	});
</script>

<div class="term" bind:this={host} role="group" aria-label="skr terminal (local spike)"></div>

<style>
	.term {
		flex: 1;
		min-width: 0;
		padding: var(--space-2);
		background: var(--terminal);
		overflow: hidden;
	}
</style>
