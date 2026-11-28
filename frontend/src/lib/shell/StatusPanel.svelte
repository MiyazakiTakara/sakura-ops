<script lang="ts">
	import { getContext } from 'svelte';
	import { explainReadiness, formatTime, type RowState } from './status';
	import { STATUS_KEY, type StatusStore } from './statusStore.svelte';

	let { detailed = false }: { detailed?: boolean } = $props();
	const store = getContext<StatusStore>(STATUS_KEY);

	interface View {
		tone: 'neutral' | 'ok' | 'warn' | 'error';
		label: string;
		note?: string;
		next?: string;
		code?: string;
		requestId?: string;
	}

	function liveness(r: RowState<unknown>): View {
		switch (r.kind) {
			case 'loading':
				return { tone: 'neutral', label: 'Sprawdzanie…' };
			case 'ok':
				return { tone: 'ok', label: 'Działa', note: `Sprawdzono ${formatTime(r.at)}` };
			case 'stale':
				return {
					tone: 'warn',
					label: 'Nieaktualne',
					note: `Ostatni udany odczyt ${formatTime(r.at)}; nowe sprawdzenie nie powiodło się.`
				};
			case 'refused':
				return {
					tone: 'error',
					label: 'Błąd odpowiedzi',
					code: r.code,
					requestId: r.requestId,
					next: 'Wybierz Sprawdź teraz; jeśli błąd wraca, sprawdź logi huba.'
				};
			case 'unreachable':
				return {
					tone: 'error',
					label: 'Brak odpowiedzi',
					note: r.last
						? `Ostatni udany odczyt ${formatTime(r.last.at)} jest nieaktualny.`
						: 'Nie ma wcześniejszego udanego odczytu.',
					next: 'Sprawdź, czy hub działa i czy przeglądarka ma z nim połączenie.'
				};
		}
	}

	function readiness(r: RowState<{ schema_version: number }>): View {
		switch (r.kind) {
			case 'loading':
				return { tone: 'neutral', label: 'Sprawdzanie…' };
			case 'ok':
				return {
					tone: 'ok',
					label: 'Gotowy',
					note: `Wersja schematu bazy: ${r.data.schema_version}. Sprawdzono ${formatTime(r.at)}.`
				};
			case 'stale':
				return {
					tone: 'warn',
					label: 'Nieaktualne',
					note: `Ostatni udany odczyt ${formatTime(r.at)} (schemat ${r.data.schema_version}).`
				};
			case 'refused': {
				const e = explainReadiness(r.code);
				return {
					tone: 'error',
					label: 'Niegotowy',
					note: e.summary,
					next: e.next,
					code: r.code,
					requestId: r.requestId
				};
			}
			case 'unreachable':
				return {
					tone: 'error',
					label: 'Niedostępne',
					note: r.last
						? `Ostatni udany odczyt ${formatTime(r.last.at)} (schemat ${r.last.data.schema_version}) jest nieaktualny.`
						: 'Nie ma wcześniejszego udanego odczytu.',
					next: 'Sprawdź, czy hub działa, potem wybierz Sprawdź teraz.'
				};
		}
	}

	const rows = $derived([
		{ id: 'health', name: 'Proces huba', source: 'GET /healthz', view: liveness(store.health) },
		{ id: 'ready', name: 'Gotowość huba', source: 'GET /readyz', view: readiness(store.readiness) }
	]);

	// Announce only the outcome, never whole logs (DESIGN.md). Polite, one short sentence.
	const announcement = $derived(rows.map((r) => `${r.name}: ${r.view.label}`).join('. '));
</script>

<section class="panel" aria-labelledby="status-heading">
	<div class="head">
		<h2 id="status-heading">Stan instancji</h2>
		<button type="button" class="primary" onclick={() => store.check()} disabled={store.checking}>
			{store.checking ? 'Sprawdzanie…' : 'Sprawdź teraz'}
		</button>
	</div>
	<p class="muted">
		Dane z endpointów <code>/healthz</code> i <code>/readyz</code>. To jedyne dane, które hub
		udostępnia na tym etapie. Odświeżane co 30 s.
	</p>
	<ul class="rows">
		{#each rows as r (r.id)}
			<li class="row" data-testid="status-{r.id}">
				<div class="name">
					<span class="dot {r.view.tone}" aria-hidden="true"></span>
					<strong>{r.name}</strong>
					<span class="state {r.view.tone}">{r.view.label}</span>
				</div>
				{#if r.view.note}<p class="note">{r.view.note}</p>{/if}
				{#if r.view.next}<p class="note">Następny krok: {r.view.next}</p>{/if}
				{#if r.view.code || detailed}
					<details>
						<summary>Szczegóły techniczne</summary>
						<dl>
							<dt>Źródło</dt>
							<dd><code>{r.source}</code></dd>
							{#if r.view.code}
								<dt>Kod</dt>
								<dd><code>{r.view.code}</code></dd>
								<dt>Identyfikator żądania</dt>
								<dd><code>{r.view.requestId}</code></dd>
							{/if}
						</dl>
					</details>
				{/if}
			</li>
		{/each}
	</ul>
	<p class="visually-hidden" role="status">{announcement}</p>
</section>

<style>
	.panel {
		background: var(--panel);
		border-radius: var(--radius-panel);
		padding: var(--space-4);
		max-width: 48rem;
	}
	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}
	h2 {
		margin: 0;
		font-size: 1.125rem;
	}
	.muted,
	.note {
		color: var(--muted);
		margin: var(--space-2) 0 0;
	}
	.rows {
		list-style: none;
		padding: 0;
		margin: var(--space-4) 0 0;
		display: grid;
		gap: var(--space-3);
	}
	.row {
		border-top: 1px solid var(--line);
		padding-top: var(--space-3);
	}
	.name {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--space-2);
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--muted);
	}
	.dot.ok {
		background: var(--success);
	}
	.dot.warn {
		background: var(--warning);
	}
	.dot.error {
		background: var(--danger);
	}
	.state.ok {
		color: var(--success);
	}
	.state.warn {
		color: var(--warning);
	}
	.state.error {
		color: var(--danger);
	}
	details {
		margin-top: var(--space-2);
		color: var(--muted);
	}
	summary {
		cursor: pointer;
		min-height: 24px;
	}
	dl {
		display: grid;
		grid-template-columns: max-content 1fr;
		gap: var(--space-1) var(--space-3);
		margin: var(--space-2) 0 0;
	}
	dd {
		margin: 0;
		overflow-wrap: anywhere;
	}
	code {
		font-family: var(--font-mono);
		font-size: 0.875rem;
	}
	.primary {
		font: inherit;
		min-height: 44px;
		color: var(--on-accent);
		background: var(--accent);
		border: 1px solid var(--accent);
		border-radius: var(--radius-control);
		padding: var(--space-2) var(--space-4);
		cursor: pointer;
	}
	.primary:disabled {
		cursor: progress;
		opacity: 0.8;
	}
	.visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
</style>
