import type { LeaderLock } from "./leader-lock.js";

/** Leadership through the Web Locks API: one tab or worker per origin holds `name` at a time. */
export function webLock(name: string, locks: LockManager): LeaderLock {
  return {
    acquire: (signal) =>
      new Promise<void>((resolve, reject) => {
        locks
          .request(name, { signal }, () => {
            resolve();
            return new Promise<void>((release) => {
              if (signal.aborted) release();
              else
                signal.addEventListener(
                  "abort",
                  () => {
                    release();
                  },
                  { once: true },
                );
            });
          })
          .catch((error: unknown) => {
            reject(error instanceof Error ? error : new Error(String(error)));
          });
      }),
  };
}
