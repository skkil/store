import { aborted } from "./aborted.js";
import type { Backoff } from "./backoff.js";

interface Waiter {
  resolve: () => void;
  reject: (error: unknown) => void;
}

/**
 * Runs sync cycles one at a time. Requests made while a cycle runs coalesce into one more cycle,
 * and a failed cycle is retried after a backoff delay until one succeeds.
 */
export class SyncLoop {
  private enabled = false;
  private wanted = false;
  private running: Promise<void> | null = null;
  private waiters: Waiter[] = [];
  private retryTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(
    private readonly cycle: (signal: AbortSignal) => Promise<void>,
    private readonly backoff: Backoff,
    private readonly signal: AbortSignal,
    private readonly onFailure: (error: unknown) => void,
  ) {}

  /** Starts running requested cycles. */
  enable(): void {
    this.enabled = true;
    this.drain();
  }

  /** Resolves after a cycle that starts after this call succeeds, or rejects with that cycle's error. */
  request(): Promise<void> {
    if (this.signal.aborted) return Promise.reject(new Error("sync driver stopped"));
    const done = new Promise<void>((resolve, reject) => this.waiters.push({ resolve, reject }));
    this.wanted = true;
    this.drain();
    return done;
  }

  /** Requests a cycle without waiting for it. */
  kick(): void {
    this.request().catch(() => undefined);
  }

  /** Stops retrying, rejects every waiter and resolves once the running cycle has settled. */
  async stop(): Promise<void> {
    if (this.retryTimer !== null) clearTimeout(this.retryTimer);
    this.retryTimer = null;
    this.settle(this.waiters.splice(0), new Error("sync driver stopped"));
    await this.running;
  }

  private drain(): void {
    if (!this.enabled || this.running !== null || !this.wanted || this.signal.aborted) return;
    this.running = this.run().finally(() => {
      this.running = null;
    });
  }

  private async run(): Promise<void> {
    while (this.wanted && !this.signal.aborted) {
      this.wanted = false;
      const batch = this.waiters.splice(0);
      try {
        await this.cycle(this.signal);
        this.backoff.reset();
        this.cancelRetry();
        this.settle(batch);
      } catch (error) {
        this.settle(batch, error);
        if (aborted(this.signal)) return;
        this.onFailure(error);
        this.scheduleRetry();
        return;
      }
    }
  }

  private scheduleRetry(): void {
    if (this.retryTimer !== null) return;
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null;
      this.kick();
    }, this.backoff.next());
  }

  private cancelRetry(): void {
    if (this.retryTimer === null) return;
    clearTimeout(this.retryTimer);
    this.retryTimer = null;
  }

  private settle(waiters: Waiter[], error?: unknown): void {
    for (const waiter of waiters) {
      if (error === undefined) waiter.resolve();
      else waiter.reject(error);
    }
  }
}
