<script lang="ts">
	import { yaml } from '@codemirror/lang-yaml';
	import { EditorState } from '@codemirror/state';
	import { EditorView } from '@codemirror/view';
	import { basicSetup } from 'codemirror';
	import { onMount } from 'svelte';

	interface Props {
		value: string;
		label: string;
	}

	let { value, label }: Props = $props();
	let host: HTMLDivElement;

	const theme = EditorView.theme(
		{
			'&': { backgroundColor: 'var(--terminal)', color: 'var(--text)', height: '100%' },
			'.cm-content': { fontFamily: 'var(--font-mono)', caretColor: 'var(--accent)' },
			'.cm-gutters': {
				backgroundColor: 'var(--panel)',
				color: 'var(--muted)',
				border: 'none'
			},
			'.cm-activeLine': { backgroundColor: 'rgb(255 255 255 / 5%)' },
			'&.cm-focused': { outline: '2px solid var(--accent)' }
		},
		{ dark: true }
	);

	onMount(() => {
		const view = new EditorView({
			parent: host,
			state: EditorState.create({
				doc: value,
				extensions: [
					basicSetup,
					yaml(),
					theme,
					EditorView.contentAttributes.of({ 'aria-label': label })
				]
			})
		});
		return () => view.destroy();
	});
</script>

<div class="editor" bind:this={host}></div>

<style>
	.editor {
		height: 18rem;
		border: 1px solid var(--control-border);
		border-radius: var(--radius-control);
		overflow: hidden;
	}
</style>
