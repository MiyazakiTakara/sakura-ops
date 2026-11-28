<script lang="ts">
	// Honest placeholder: states that the section does not exist yet and why. No controls.
	let {
		title,
		reason,
		children
	}: { title: string; reason: string; children?: import('svelte').Snippet } = $props();
</script>

<svelte:head>
	<title>{title} — SakuraOps</title>
</svelte:head>

<h1>{title}</h1>
<section class="empty" aria-labelledby="na-heading">
	<h2 id="na-heading">Niedostępne</h2>
	<p>{reason}</p>
	{#if children}{@render children()}{/if}
</section>

<style>
	h1 {
		font-size: 1.375rem;
		margin: 0 0 var(--space-4);
	}
	.empty {
		background: var(--panel);
		border-radius: var(--radius-panel);
		padding: var(--space-4);
		max-width: 48rem;
	}
	h2 {
		margin: 0 0 var(--space-2);
		font-size: 1.125rem;
		color: var(--warning);
	}
	p {
		margin: 0;
		color: var(--muted);
	}
</style>
