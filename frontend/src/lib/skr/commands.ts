// Local-only command table of the technical spike. Nothing here reaches a backend
// or executes anything; output is a fixed set of static strings.

export const SPIKE_COMMANDS = ['help', 'clear', 'show version'] as const;

export type CommandResult = { kind: 'clear' } | { kind: 'output'; lines: string[] };

export function runLocalCommand(line: string): CommandResult {
	switch (line.trim()) {
		case '':
			return { kind: 'output', lines: [] };
		case 'help':
			return {
				kind: 'output',
				lines: ['Available commands (local spike):', ...SPIKE_COMMANDS.map((c) => `  ${c}`)]
			};
		case 'clear':
			return { kind: 'clear' };
		case 'show version':
			return { kind: 'output', lines: ['SakuraOps Next technical spike (no backend)'] };
		default:
			// The input is deliberately not echoed back.
			return { kind: 'output', lines: ['Unknown command (spike: no backend)'] };
	}
}
