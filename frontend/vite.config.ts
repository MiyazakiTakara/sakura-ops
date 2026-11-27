import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [
		sveltekit({
			// SPA mode: no SSR, no server routes. Unknown paths fall back to index.html,
			// but the Go server only serves that fallback for non-reserved paths.
			adapter: adapter({ pages: 'build', assets: 'build', fallback: 'index.html', strict: true })
		})
	],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'node'
	}
});
