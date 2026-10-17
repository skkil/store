/** Waits `ms` milliseconds, or less when any of `signals` aborts. Never rejects. */
export function sleep(ms: number, ...signals: AbortSignal[]): Promise<void> {
  return new Promise((resolve) => {
    if (signals.some((signal) => signal.aborted)) {
      resolve();
      return;
    }
    const done = () => {
      clearTimeout(timer);
      for (const signal of signals) signal.removeEventListener("abort", done);
      resolve();
    };
    const timer = setTimeout(done, ms);
    for (const signal of signals) signal.addEventListener("abort", done, { once: true });
  });
}
