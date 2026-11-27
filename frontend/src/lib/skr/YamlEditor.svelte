<script lang="ts">
	import { yaml } from '@codemirror/lang-yaml';
	import { HighlightStyle, syntaxHighlighting } from '@codemirror/language';
	import { EditorState } from '@codemirror/state';
	import { EditorView } from '@codemirror/view';
	import { tags as t } from '@lezer/highlight';
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

	// Token colours from the DESIGN.md palette; the default highlight style is made for light
	// backgrounds and fails WCAG AA contrast on the dark editor.
	const highlight = HighlightStyle.define([
		{ tag: [t.propertyName, t.keyword, t.definition(t.propertyName)], color: '#d9a0b7' },
		{ tag: [t.string, t.content], color: '#98cfa8' },
		{ tag: [t.number, t.bool, t.null, t.atom], color: '#e9bc83' },
		{ tag: [t.comment, t.lineComment], color: '#b9c1cb', fontStyle: 'italic' },
		{ tag: [t.meta, t.typeName, t.labelName], color: '#e9bc83' },
		{ tag: [t.punctuation, t.separator, t.brace, t.squareBracket], color: '#f0f2f5' }
	]);

	onMount(() => {
		const view = new EditorView({
			parent: host,
			state: EditorState.create({
				doc: value,
				extensions: [
					basicSetup,
					yaml(),
					theme,
					syntaxHighlighting(highlight),
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
