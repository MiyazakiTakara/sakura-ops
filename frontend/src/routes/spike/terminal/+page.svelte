<script lang="ts">
	import FloatingWindow from '../../../lib/skr/FloatingWindow.svelte';
	import TerminalPane from '../../../lib/skr/TerminalPane.svelte';
	import YamlEditor from '../../../lib/skr/YamlEditor.svelte';

	// Fictional task, not a real or runnable definition.
	const sampleTask = `apiVersion: "sakuraops/v1"
kind: "Task"
metadata:
  id: "spike-example"
  version: 1
  name: "Fictional example task"
steps:
  - id: "ping"
    action: "agent.ping"
    version: 1
`;

	let terminalOpen = $state(true);
</script>

<svelte:head>
	<title>Technical spike: terminal</title>
</svelte:head>

<main class="page">
	<h1>Technical spike — not a product feature</h1>
	<p class="note">
		Checks Svelte, xterm.js and CodeMirror integration. The terminal is local echo only and never
		contacts a backend or executes anything.
	</p>

	{#if !terminalOpen}
		<button type="button" onclick={() => (terminalOpen = true)}>Reopen spike terminal</button>
	{/if}

	<h2>Editor (YAML, fictional task)</h2>
	<YamlEditor value={sampleTask} label="Spike task YAML editor" />
</main>

{#if terminalOpen}
	<FloatingWindow title="skr (spike)" onclose={() => (terminalOpen = false)}>
		<TerminalPane />
	</FloatingWindow>
{/if}

<style>
	.page {
		padding: var(--space-5);
		max-width: 60rem;
	}
	h1 {
		font-size: 1.375rem;
		margin: 0 0 var(--space-3);
	}
	h2 {
		font-size: 1.0625rem;
		margin: var(--space-5) 0 var(--space-2);
	}
	.note {
		color: var(--muted);
	}
	button {
		font: inherit;
		color: var(--on-accent);
		background: var(--accent);
		border: 1px solid var(--accent);
		border-radius: var(--radius-control);
		padding: var(--space-2) var(--space-4);
		cursor: pointer;
	}
</style>
