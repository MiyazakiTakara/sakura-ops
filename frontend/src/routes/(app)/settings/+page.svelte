<script lang="ts">
	import StatusPanel from '../../../lib/shell/StatusPanel.svelte';
</script>

<svelte:head>
	<title>Ustawienia — SakuraOps</title>
</svelte:head>

<h1>Ustawienia</h1>
<StatusPanel detailed />
<section class="na" aria-labelledby="na-h">
	<h2 id="na-h">Niedostępne</h2>
	<p>
		Adres instancji, strefa czasowa, użytkownicy i uprawnienia nie są jeszcze wdrożone. Stan
		instancji powyżej pochodzi z rzeczywistych odpowiedzi huba.
	</p>
</section>

<style>
	h1 {
		font-size: 1.375rem;
		margin: 0 0 var(--space-4);
	}
	.na {
		margin-top: var(--space-4);
		max-width: 48rem;
	}
	h2 {
		font-size: 1.125rem;
		margin: 0 0 var(--space-2);
		color: var(--warning);
	}
	p {
		margin: 0;
		color: var(--muted);
	}
</style>
