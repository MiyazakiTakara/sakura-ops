<script lang="ts">
	import { onMount, tick, type Snippet } from 'svelte';

	interface Props {
		title: string;
		onclose: () => void;
		children: Snippet;
		/** Initial position; defaults suit a bare page. The shell opens it clear of the navigation. */
		initialX?: number;
		initialY?: number;
	}

	let { title, onclose, children, initialX = 32, initialY = 96 }: Props = $props();

	const STEP = 16;
	const MIN_W = 280;
	const MIN_H = 160;
	// DESIGN.md: below 768 px (phone, or a 200% zoom of a laptop) the terminal uses the whole
	// available area; moving and resizing are then neither possible nor needed.
	const COMPACT_QUERY = '(max-width: 767.98px)';

	// svelte-ignore state_referenced_locally
	let x = $state(initialX);
	// svelte-ignore state_referenced_locally
	let y = $state(initialY);
	let w = $state(640);
	let h = $state(380);
	let minimized = $state(false);
	let compact = $state(false);
	let root: HTMLElement;

	function boxHeight(): number {
		return minimized && root ? root.offsetHeight : h;
	}

	// Keeps the whole window inside the viewport (docs/09-terminal.md).
	function clampPosition(): void {
		w = Math.min(w, window.innerWidth);
		h = Math.min(h, window.innerHeight);
		x = Math.max(0, Math.min(x, window.innerWidth - w));
		y = Math.max(0, Math.min(y, window.innerHeight - boxHeight()));
	}

	let titlebar: HTMLElement;

	// xterm consumes Tab for completion, so Tab cannot leave the terminal. Escape and Ctrl+]
	// (the telnet/SSH escape character) are the keyboard exits (WCAG 2.1.2): focus moves to the
	// title bar, from where Tab and Shift+Tab continue. Neither closes or minimizes the window.
	// Escape is expected to be forwarded to a future SSH PTY (issue #26); Ctrl+] stays the exit.
	function escapeKey(e: KeyboardEvent): void {
		const exit = e.key === 'Escape' || (e.ctrlKey && !e.altKey && !e.metaKey && e.key === ']');
		if (exit && e.target !== titlebar) {
			e.preventDefault();
			titlebar.focus();
		}
	}

	/** Brings a minimized window back and keeps it inside the viewport (called by the shell). */
	export async function reveal(): Promise<void> {
		if (minimized) await toggleMinimized();
	}

	async function toggleMinimized(): Promise<void> {
		minimized = !minimized;
		await tick();
		clampPosition();
	}

	function setSize(nw: number, nh: number): void {
		w = Math.max(Math.min(MIN_W, window.innerWidth - x), Math.min(nw, window.innerWidth - x));
		h = Math.max(Math.min(MIN_H, window.innerHeight - y), Math.min(nh, window.innerHeight - y));
	}

	onMount(() => {
		const mq = window.matchMedia(COMPACT_QUERY);
		const sync = (): void => {
			compact = mq.matches;
		};
		sync();
		clampPosition();
		mq.addEventListener('change', sync);
		return () => mq.removeEventListener('change', sync);
	});

	let drag: { px: number; py: number; ox: number; oy: number } | null = null;

	function dragStart(e: PointerEvent): void {
		if (compact) return;
		if ((e.target as HTMLElement).closest('button')) return;
		drag = { px: e.clientX, py: e.clientY, ox: x, oy: y };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function dragMove(e: PointerEvent): void {
		if (!drag) return;
		x = drag.ox + e.clientX - drag.px;
		y = drag.oy + e.clientY - drag.py;
		clampPosition();
	}
	function dragEnd(): void {
		drag = null;
	}

	let resize: { px: number; py: number; ow: number; oh: number } | null = null;

	function resizeStart(e: PointerEvent): void {
		if (compact) return;
		resize = { px: e.clientX, py: e.clientY, ow: w, oh: h };
		(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
	}
	function resizeMove(e: PointerEvent): void {
		if (!resize) return;
		setSize(resize.ow + e.clientX - resize.px, resize.oh + e.clientY - resize.py);
	}
	function resizeEnd(): void {
		resize = null;
	}

	function arrowDelta(e: KeyboardEvent): [number, number] | null {
		switch (e.key) {
			case 'ArrowLeft':
				return [-STEP, 0];
			case 'ArrowRight':
				return [STEP, 0];
			case 'ArrowUp':
				return [0, -STEP];
			case 'ArrowDown':
				return [0, STEP];
			default:
				return null;
		}
	}

	function moveKey(e: KeyboardEvent): void {
		const d = arrowDelta(e);
		if (!d) return;
		e.preventDefault();
		if (compact) return;
		x += d[0];
		y += d[1];
		clampPosition();
	}

	function resizeKey(e: KeyboardEvent): void {
		const d = arrowDelta(e);
		if (!d) return;
		e.preventDefault();
		setSize(w + d[0], h + d[1]);
	}
</script>

<svelte:window onresize={clampPosition} />

<section
	class="window"
	class:minimized
	class:compact
	bind:this={root}
	onkeydowncapture={escapeKey}
	style:left={compact ? '0' : `${x}px`}
	style:top={compact ? '0' : `${y}px`}
	style:width={compact ? '100vw' : `${w}px`}
	style:height={minimized ? 'auto' : compact ? '100dvh' : `${h}px`}
	aria-label={title}
>
	<div
		role="toolbar"
		bind:this={titlebar}
		class="titlebar"
		tabindex="0"
		aria-label="{title}: window title bar. Arrow keys move the window."
		onpointerdown={dragStart}
		onpointermove={dragMove}
		onpointerup={dragEnd}
		onpointercancel={dragEnd}
		onkeydown={moveKey}
	>
		<h2>{title}</h2>
		<button type="button" onclick={toggleMinimized} aria-pressed={minimized}>
			{minimized ? 'Restore' : 'Minimize'}
		</button>
		<button type="button" onclick={onclose}>Close</button>
	</div>
	<div class="body" hidden={minimized}>
		{@render children()}
	</div>
	{#if !minimized && !compact}
		<div
			class="handle"
			role="slider"
			tabindex="0"
			aria-label="Resize window. Arrow keys change size."
			aria-orientation="horizontal"
			aria-valuemin={MIN_W}
			aria-valuenow={w}
			aria-valuetext="{w} by {h} pixels"
			onpointerdown={resizeStart}
			onpointermove={resizeMove}
			onpointerup={resizeEnd}
			onpointercancel={resizeEnd}
			onkeydown={resizeKey}
		></div>
	{/if}
</section>

<style>
	.window {
		position: fixed;
		display: flex;
		flex-direction: column;
		background: var(--terminal);
		border: 1px solid var(--control-border);
		border-radius: var(--radius-panel);
		box-shadow: 0 8px 32px rgb(0 0 0 / 50%);
		overflow: hidden;
		max-width: 100vw;
	}
	.window.compact {
		border-radius: 0;
		border-width: 0;
	}
	.titlebar {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		padding: var(--space-2) var(--space-3);
		background: var(--elevated);
		cursor: grab;
		touch-action: none;
		user-select: none;
	}
	.titlebar h2 {
		flex: 1;
		margin: 0;
		font-size: 1rem;
		font-weight: 600;
	}
	button {
		font: inherit;
		color: var(--text);
		background: var(--panel);
		border: 1px solid var(--control-border);
		border-radius: var(--radius-control);
		padding: var(--space-1) var(--space-3);
		cursor: pointer;
	}
	.body {
		flex: 1;
		min-height: 0;
		display: flex;
	}
	.body[hidden] {
		display: none;
	}
	.handle {
		position: absolute;
		right: 0;
		bottom: 0;
		width: 20px;
		height: 20px;
		cursor: nwse-resize;
		touch-action: none;
		background: linear-gradient(135deg, transparent 50%, var(--control-border) 50%);
	}
</style>
