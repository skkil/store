import type { LeaderLock } from "./leader-lock.js";
import { noopLock } from "./noop-lock.js";
import { webLock } from "./web-lock.js";

/** A Web Lock named `name` where `navigator.locks` exists, otherwise {@link noopLock}. */
export function defaultLock(name: string): LeaderLock {
  const locks = (globalThis as { navigator?: { locks?: LockManager } }).navigator?.locks;
  return locks ? webLock(name, locks) : noopLock();
}
