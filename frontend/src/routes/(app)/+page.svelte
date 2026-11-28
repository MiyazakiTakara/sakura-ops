<script lang="ts">
	import StatusPanel from '../../lib/shell/StatusPanel.svelte';
</script>

<svelte:head>
	<title>Pulpit — SakuraOps</title>
</svelte:head>

<h1>Pulpit</h1>
<p class="lead">
	Prototyp powłoki aplikacji. Poniżej jedyne realne dane, jakie hub udostępnia: stan procesu i
	gotowość. Serwery, zadania i historia pojawią się po wdrożeniu odpowiednich API.
</p>
<StatusPanel />

<style>
	h1 {
		font-size: 1.375rem;
		margin: 0 0 var(--space-2);
	}
	.lead {
		color: var(--muted);
		max-width: 48rem;
		margin: 0 0 var(--space-4);
	}
</style>
