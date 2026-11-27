<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		onclose: () => void;
		children: Snippet;
	}

	let { title, onclose, children }: Props = $props();

	const STEP = 16;
	const MIN_W = 280;
	const MIN_H = 160;

	let x = $state(32);
	let y = $state(96);
	let w = $state(640);
	let h = $state(380);
	let minimized = $state(false);

	function clampPosition(): void {
		x = Math.max(0, Math.min(x, window.innerWidth - 80));
		y = Math.max(0, Math.min(y, window.innerHeight - 40));
	}

	function setSize(nw: number, nh: number): void {
		w = Math.max(MIN_W, Math.min(nw, window.innerWidth - x));
		h = Math.max(MIN_H, Math.min(nh, window.innerHeight - y));
	}

	let drag: { px: number; py: number; ox: number; oy: number } | null = null;

	function dragStart(e: PointerEvent): void {
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
	style:left="{x}px"
	style:top="{y}px"
	style:width="{w}px"
	style:height={minimized ? 'auto' : `${h}px`}
	aria-label={title}
>
	<header
		role="toolbar"
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
		<button type="button" onclick={() => (minimized = !minimized)} aria-pressed={minimized}>
			{minimized ? 'Restore' : 'Minimize'}
		</button>
		<button type="button" onclick={onclose}>Close</button>
	</header>
	<div class="body" hidden={minimized}>
		{@render children()}
	</div>
	{#if !minimized}
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
