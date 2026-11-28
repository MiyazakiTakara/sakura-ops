<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount, setContext, tick } from 'svelte';
	import FloatingWindow from '../../lib/skr/FloatingWindow.svelte';
	import TerminalPane from '../../lib/skr/TerminalPane.svelte';
	import { isActive, NAV_ITEMS } from '../../lib/shell/nav';
	import { STATUS_KEY, StatusStore } from '../../lib/shell/statusStore.svelte';

	let { children } = $props();

	const status = new StatusStore();
	setContext(STATUS_KEY, status);

	let menuOpen = $state(false);
	let menuButton: HTMLButtonElement;
	let terminalButton: HTMLButtonElement;
	let mainEl: HTMLElement;

	// The terminal lives in the layout, so routing never destroys its session (docs/09-terminal.md).
	let terminalOpen = $state(false);
	let win = $state<FloatingWindow>();
	let startX = $state(32);
	let startY = $state(96);
	let pane = $state<TerminalPane>();

	onMount(() => {
		const prev = document.documentElement.lang;
		document.documentElement.lang = 'pl';
		const stop = status.start();
		return () => {
			stop();
			document.documentElement.lang = prev;
		};
	});

	// After client-side navigation move focus to the content so keyboard and screen-reader users
	// start at the new page. Not on first load, where the browser's own behaviour is right.
	afterNavigate((nav) => {
		menuOpen = false;
		if (nav.type !== 'enter') mainEl?.focus();
	});

	async function openTerminal(): Promise<void> {
		if (!terminalOpen) {
			// Open at the right and below the horizontal capsule on narrower screens, so the window
			// does not cover the navigation it was summoned from.
			startX = Math.max(0, window.innerWidth - 640 - 32);
			startY = window.innerWidth >= 1200 ? 96 : 200;
			terminalOpen = true;
			await tick();
		} else {
			await win?.reveal();
		}
		pane?.focus();
	}

	async function closeTerminal(): Promise<void> {
		terminalOpen = false;
		await tick();
		terminalButton?.focus(); // restore focus to what summoned the window
	}

	function menuKey(e: KeyboardEvent): void {
		if (e.defaultPrevented) return; // Escape already handled (e.g. leaving the terminal)
		if (e.key === 'Escape' && menuOpen) {
			e.preventDefault();
			menuOpen = false;
			menuButton.focus();
		}
	}
</script>

<svelte:window onkeydown={menuKey} />

<a class="skip" href="#main-content">Przejdź do treści</a>

<div class="shell">
	<header class="topbar">
		<a class="brand" href="/">SakuraOps</a>
		<button
			type="button"
			class="menu-button"
			aria-expanded={menuOpen}
			aria-controls="main-nav"
			bind:this={menuButton}
			onclick={() => (menuOpen = !menuOpen)}
		>
			Menu
		</button>
	</header>

	<nav id="main-nav" class="capsule" class:open={menuOpen} aria-label="Główna nawigacja">
		<ul>
			{#each NAV_ITEMS as item (item.href)}
				{@const active = isActive(item.href, page.url.pathname)}
				<li>
					<a href={item.href} aria-current={active ? 'page' : undefined} class:active>
						{item.label}
					</a>
				</li>
			{/each}
		</ul>
		<span class="sep" aria-hidden="true"></span>
		<button type="button" class="terminal-button" bind:this={terminalButton} onclick={openTerminal}>
			Terminal
		</button>
	</nav>

	<main id="main-content" tabindex="-1" bind:this={mainEl}>
		{@render children()}
	</main>
</div>

{#if terminalOpen}
	<div lang="en" class="terminal-host">
		<FloatingWindow
			bind:this={win}
			title="skr"
			initialX={startX}
			initialY={startY}
			onclose={closeTerminal}
		>
			<TerminalPane bind:this={pane} />
		</FloatingWindow>
	</div>
{/if}

<style>
	.skip {
		position: fixed;
		left: var(--space-2);
		top: var(--space-2);
		z-index: 100;
		padding: var(--space-2) var(--space-4);
		background: var(--accent);
		color: var(--on-accent);
		border-radius: var(--radius-control);
		transform: translateY(-200%);
	}
	.skip:focus {
		transform: none;
	}
	.shell {
		min-height: 100dvh;
		display: grid;
		grid-template-columns: max-content minmax(0, 1fr);
		grid-template-rows: auto 1fr;
		grid-template-areas:
			'top top'
			'nav main';
		gap: var(--space-4) var(--space-5);
		padding: 0 var(--space-5) var(--space-5);
	}
	.topbar {
		grid-area: top;
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
		min-height: 56px;
		border-bottom: 1px solid var(--line);
		margin: 0 calc(-1 * var(--space-5));
		padding: 0 var(--space-5);
	}
	.brand {
		font-size: 1.25rem;
		font-weight: 700;
		color: var(--accent);
		text-decoration: none;
		min-height: 44px;
		display: inline-flex;
		align-items: center;
	}
	.menu-button {
		display: none;
	}
	.capsule {
		grid-area: nav;
		align-self: start;
		position: sticky;
		top: var(--space-4);
		width: 180px;
		background: var(--elevated);
		border-radius: 14px;
		padding: var(--space-2);
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}
	ul {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}
	.capsule a,
	.terminal-button,
	.menu-button {
		display: flex;
		align-items: center;
		min-height: 44px;
		padding: var(--space-2) var(--space-3);
		border-radius: var(--radius-control);
		color: var(--text);
		text-decoration: none;
		font: inherit;
		cursor: pointer;
	}
	.capsule a:hover,
	.terminal-button:hover {
		background: var(--panel);
	}
	.capsule a.active {
		color: var(--accent);
		font-weight: 700;
		background: var(--panel);
		box-shadow: inset 3px 0 0 var(--accent);
	}
	.terminal-button,
	.menu-button {
		background: transparent;
		border: 1px solid var(--control-border);
	}
	.menu-button {
		display: none;
	}
	.sep {
		display: block;
		height: 1px;
		background: var(--line);
		margin: var(--space-1) 0;
	}
	main {
		grid-area: main;
		min-width: 0;
	}
	main:focus {
		outline: none;
	}

	/* Tablet: compact horizontal capsule with visible labels. */
	@media (max-width: 1199.98px) {
		.shell {
			grid-template-columns: minmax(0, 1fr);
			grid-template-areas:
				'top'
				'nav'
				'main';
			padding: 0 var(--space-4) var(--space-4);
		}
		.topbar {
			margin: 0 calc(-1 * var(--space-4));
			padding: 0 var(--space-4);
		}
		.capsule {
			position: static;
			width: fit-content;
			max-width: 100%;
			flex-direction: row;
			flex-wrap: wrap;
			align-items: center;
		}
		ul {
			flex-direction: row;
			flex-wrap: wrap;
		}
		.sep {
			width: 1px;
			height: 28px;
			margin: 0 var(--space-1);
		}
	}

	/* Phone: navigation behind a visible Menu button. */
	@media (max-width: 767.98px) {
		.menu-button {
			display: flex;
		}
		.capsule {
			display: none;
			width: 100%;
			flex-direction: column;
			align-items: stretch;
		}
		.capsule.open {
			display: flex;
		}
		ul {
			flex-direction: column;
		}
		.sep {
			width: auto;
			height: 1px;
			margin: var(--space-1) 0;
		}
	}
</style>
