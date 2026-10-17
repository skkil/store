export class FakeLockManager {
  private tails = new Map<string, Promise<unknown>>();

  request(
    name: string,
    options: { signal?: AbortSignal },
    callback: (lock: { name: string }) => Promise<unknown>,
  ): Promise<unknown> {
    const signal = options.signal;
    const previous = this.tails.get(name) ?? Promise.resolve();
    const granted = previous.then(() => {
      if (signal?.aborted) throw new DOMException("aborted", "AbortError");
      return callback({ name });
    });
    this.tails.set(
      name,
      granted.catch(() => undefined),
    );
    if (!signal) return granted;
    const aborted = new Promise<never>((_, reject) => {
      signal.addEventListener(
        "abort",
        () => {
          reject(new DOMException("aborted", "AbortError"));
        },
        {
          once: true,
        },
      );
    });
    return Promise.race([granted, aborted]);
  }
}
