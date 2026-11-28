// Reactive wrapper around the pure logic in status.ts. One instance per shell; it polls the
// two real endpoints and nothing else. Checks are GET only, so retrying never repeats a mutation.
import type { Health, Readiness } from '../api/errors';
import { getHealth, getReadiness, type Fetcher } from '../api/system';
import {
	emptyProbe,
	deriveRow,
	POLL_INTERVAL_MS,
	recordCheck,
	type Probe,
	type RowState
} from './status';

export class StatusStore {
	#health = $state<Probe<Health>>(emptyProbe());
	#ready = $state<Probe<Readiness>>(emptyProbe());
	#now = $state(Date.now());
	checking = $state(false);
	#timer: ReturnType<typeof setInterval> | null = null;
	#fetcher: Fetcher;

	constructor(fetcher: Fetcher = (i, init) => fetch(i, init)) {
		this.#fetcher = fetcher;
	}

	get health(): RowState<Health> {
		return deriveRow(this.#health, this.#now);
	}
	get readiness(): RowState<Readiness> {
		return deriveRow(this.#ready, this.#now);
	}

	async check(): Promise<void> {
		if (this.checking) return;
		this.checking = true;
		try {
			const [h, r] = await Promise.all([getHealth(this.#fetcher), getReadiness(this.#fetcher)]);
			const at = Date.now();
			this.#health = recordCheck(this.#health, h, at);
			this.#ready = recordCheck(this.#ready, r, at);
			this.#now = at;
		} finally {
			this.checking = false;
		}
	}

	/** Starts polling; returns the stop function. Re-checks when the tab becomes visible again. */
	start(): () => void {
		void this.check();
		this.#timer = setInterval(() => {
			this.#now = Date.now(); // lets a successful row age into "stale" even if checks stall
			if (document.visibilityState === 'visible') void this.check();
		}, POLL_INTERVAL_MS);
		const onVisible = (): void => {
			if (document.visibilityState === 'visible') void this.check();
		};
		document.addEventListener('visibilitychange', onVisible);
		return () => {
			if (this.#timer) clearInterval(this.#timer);
			document.removeEventListener('visibilitychange', onVisible);
		};
	}
}

export const STATUS_KEY = Symbol('sakuraops.status');
