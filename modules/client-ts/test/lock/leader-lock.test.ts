import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defaultLock, noopLock } from "../../src/index.js";
import { FakeLockManager } from "../support/fake-lock-manager.js";
import { settle, startHarness } from "../support/harness.js";

describe("leader election", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("navigator", { locks: new FakeLockManager() });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("lets only the leading tab sync and hands over when it stops", async () => {
    const first = startHarness({ lock: defaultLock("skkil-store-sync") });
    const second = startHarness({ lock: defaultLock("skkil-store-sync") });
    await settle();

    expect(first.transports.calls).toContain("pull");
    expect(second.transports.calls).toEqual([]);
    expect(second.transports.streams).toBe(0);

    await first.driver.stop();
    await settle();

    expect(second.transports.calls).toContain("pull");
    expect(second.transports.streams).toBe(1);
    await second.driver.stop();
  });

  it("a follower that stops before leading never syncs", async () => {
    const first = startHarness({ lock: defaultLock("skkil-store-sync") });
    const second = startHarness({ lock: defaultLock("skkil-store-sync") });
    await settle();

    await second.driver.stop();
    await first.driver.stop();
    await settle();

    expect(second.transports.calls).toEqual([]);
  });

  it("falls back to no lock without navigator.locks", async () => {
    vi.stubGlobal("navigator", {});
    const lock = defaultLock("skkil-store-sync");

    await expect(lock.acquire(new AbortController().signal)).resolves.toBeUndefined();
    await expect(noopLock().acquire(AbortSignal.abort())).rejects.toThrow();
  });
});
