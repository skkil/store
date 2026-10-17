/** Bounds for exponential backoff. */
export interface BackoffOptions {
  /** The ceiling of the first delay. */
  baseMs: number;
  /** The largest ceiling any delay may reach. */
  maxMs: number;
}

/** Full-jitter exponential backoff: each delay is uniform in `[0, min(maxMs, baseMs * 2^attempt))`. */
export class Backoff {
  private attempt = 0;

  constructor(
    private readonly options: BackoffOptions,
    private readonly random: () => number = Math.random,
  ) {}

  /** The delay before the next attempt; each call doubles the ceiling. */
  next(): number {
    const ceiling = Math.min(this.options.maxMs, this.options.baseMs * 2 ** this.attempt);
    this.attempt += 1;
    return Math.floor(this.random() * ceiling);
  }

  /** Starts over from the first delay after a success. */
  reset(): void {
    this.attempt = 0;
  }
}
