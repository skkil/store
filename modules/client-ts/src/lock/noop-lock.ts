import type { LeaderLock } from "./leader-lock.js";

/** Always leads at once, for hosts with a single context such as React Native. */
export function noopLock(): LeaderLock {
  return {
    acquire: (signal) =>
      signal.aborted ? Promise.reject(new Error("aborted")) : Promise.resolve(),
  };
}
