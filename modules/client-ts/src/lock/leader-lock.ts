/** Elects the one context (tab or worker) that runs the sync loop. */
export interface LeaderLock {
  /** Resolves once this context leads; leadership is held until `signal` aborts. Rejects if aborted first. */
  acquire(signal: AbortSignal): Promise<void>;
}
