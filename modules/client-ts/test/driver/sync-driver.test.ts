import { Code, ConnectError } from "@connectrpc/connect";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { settle, startHarness, type Harness } from "../support/harness.js";

describe("createSyncDriver", () => {
  let harness: Harness | null = null;

  beforeEach(() => {
    vi.useFakeTimers({ now: new Date("2026-11-08T12:00:00Z") });
  });

  afterEach(async () => {
    await harness?.driver.stop();
    harness = null;
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("pushes pending mutations before pulling on start", async () => {
    harness = startHarness({}, (core) => {
      core.mutate(2);
    });
    await settle();

    expect(harness.core.calls.slice(0, 4)).toEqual([
      "pushRequest",
      "applyPushResponse",
      "pullRequest",
      "applyPullResponse",
    ]);
    expect(harness.transports.calls.slice(0, 2)).toEqual(["push", "pull"]);
    expect(harness.server.head).toBe(2n);
    expect(harness.core.status()).toMatchObject({ pending: 0, cursorVersion: 2n });
  });

  it("pushes in batches and pulls every page", async () => {
    harness = startHarness({ pushLimit: 2, pullLimit: 2 }, (core, server, transports) => {
      transports.pokesDown = true;
      core.mutate(3);
      for (let i = 0; i < 3; i++) server.write();
    });
    await settle();

    expect(harness.transports.calls.filter((call) => call === "push")).toHaveLength(2);
    expect(harness.transports.calls.filter((call) => call === "pull")).toHaveLength(3);
    expect(harness.core.cursorVersion).toBe(6n);
    expect(harness.changes).toEqual([["task"], ["task"], ["task"]]);
  });

  it("pulls when a poke is ahead of the cursor and ignores stale pokes", async () => {
    harness = startHarness();
    await settle();
    const pulls = () => harness?.transports.calls.filter((call) => call === "pull").length;
    const before = pulls();

    harness.transports.poke(0n);
    await settle();
    expect(pulls()).toBe(before);

    harness.server.write();
    harness.transports.poke(1n);
    await settle();
    expect(pulls()).toBe((before ?? 0) + 1);
    expect(harness.core.cursorVersion).toBe(1n);
  });

  it("pulls again whenever the poke stream reconnects", async () => {
    const lost = new ConnectError("lost", Code.Unavailable);
    harness = startHarness({}, () => undefined);
    harness.transports.pokeFailures.push(lost);
    await settle();
    const pullsBefore = harness.transports.calls.filter((call) => call === "pull").length;

    await settle(1_000);

    expect(harness.transports.streams).toBe(1);
    expect(harness.transports.calls.filter((call) => call === "pull").length).toBe(pullsBefore + 1);
    expect(harness.errors).toContain(lost);
  });

  it("reconnects the poke stream with full-jitter exponential backoff", async () => {
    const random = vi.fn(() => 0.5);
    harness = startHarness({ random, backoff: { baseMs: 1_000, maxMs: 4_000 } });
    const down = new ConnectError("down", Code.Unavailable);
    harness.transports.pokeFailures.push(down, down, down, down);
    await settle();
    expect(harness.transports.pokeTokens).toHaveLength(1);

    await settle(499);
    expect(harness.transports.pokeTokens).toHaveLength(1);
    await settle(1);
    expect(harness.transports.pokeTokens).toHaveLength(2);
    await settle(1_000);
    expect(harness.transports.pokeTokens).toHaveLength(3);
    await settle(2_000);
    expect(harness.transports.pokeTokens).toHaveLength(4);
    await settle(2_000);
    expect(harness.transports.pokeTokens).toHaveLength(5);
    expect(harness.transports.streams).toBe(1);
  });

  it("retries a failed push with backoff and never drops mutations", async () => {
    harness = startHarness({}, (core, _server, transports) => {
      transports.pokesDown = true;
      core.mutate(2);
    });
    const offline = new ConnectError("offline", Code.Unavailable);
    harness.transports.pushFailures.push(offline, offline);
    await settle();
    expect(harness.core.status().pending).toBe(2);
    expect(harness.statuses.at(-1)).toMatchObject({ online: false, pending: 2 });

    await settle(500);
    expect(harness.transports.calls.filter((call) => call === "push")).toHaveLength(2);
    expect(harness.core.status().pending).toBe(2);

    await settle(1_000);
    expect(harness.server.head).toBe(2n);
    expect(harness.core.status().pending).toBe(0);
    expect(harness.statuses.at(-1)).toMatchObject({ online: true, pending: 0 });
  });

  it("refreshes the sync token after a 401 and retries the pull", async () => {
    harness = startHarness({}, (_core, _server, transports) => {
      transports.pokesDown = true;
    });
    harness.transports.pullFailures.push(new ConnectError("expired", Code.Unauthenticated));
    await settle();

    expect(harness.transports.pullTokens).toEqual(["token-1", "token-2"]);
    expect(harness.errors.every((error) => String(error).includes("pokes are down"))).toBe(true);
  });

  it("refreshes the sync token shortly before it expires", async () => {
    harness = startHarness({ tokenRefreshMarginMs: 60_000, pollIntervalMs: 10 * 60_000 });
    await settle();
    expect(harness.getSyncToken).toHaveBeenCalledTimes(1);

    await settle(10 * 60_000);
    expect(harness.transports.pullTokens.at(-1)).toBe("token-1");

    await settle(10 * 60_000);
    expect(harness.getSyncToken).toHaveBeenCalledTimes(2);
    expect(harness.transports.pullTokens.at(-1)).toBe("token-2");
  });

  it("pulls on a timer", async () => {
    harness = startHarness({ pollIntervalMs: 30_000 });
    await settle();
    const pulls = harness.transports.calls.length;

    await settle(30_000);
    expect(harness.transports.calls.length).toBe(pulls + 1);
  });

  it("pulls when the device comes online and when the page becomes visible", async () => {
    const window = new EventTarget();
    const document = Object.assign(new EventTarget(), { visibilityState: "hidden" });
    vi.stubGlobal("window", window);
    vi.stubGlobal("document", document);
    harness = startHarness();
    await settle();
    const pulls = () => harness?.transports.calls.filter((call) => call === "pull").length ?? 0;
    const before = pulls();

    window.dispatchEvent(new Event("online"));
    await settle();
    expect(pulls()).toBe(before + 1);

    document.dispatchEvent(new Event("visibilitychange"));
    await settle();
    expect(pulls()).toBe(before + 1);

    document.visibilityState = "visible";
    document.dispatchEvent(new Event("visibilitychange"));
    await settle();
    expect(pulls()).toBe(before + 2);

    window.dispatchEvent(new Event("offline"));
    expect(harness.driver.status().online).toBe(false);
  });

  it("reports online state, pending count, last sync time and issues", async () => {
    harness = startHarness({}, (core) => {
      core.mutate(1);
      core.issues = 2;
    });
    expect(harness.statuses).toEqual([]);
    await settle();

    expect(harness.statuses[0]).toEqual({
      online: false,
      pending: 1,
      lastSyncedAt: null,
      issues: 2,
    });
    expect(harness.statuses.at(-1)).toEqual({
      online: true,
      pending: 0,
      lastSyncedAt: Date.parse("2026-11-08T12:00:00Z"),
      issues: 2,
    });
  });

  it("syncNow resolves after a full cycle and rejects when it fails", async () => {
    harness = startHarness();
    await settle();
    harness.core.mutate(1);

    const done = harness.driver.syncNow();
    await settle();
    await expect(done).resolves.toBeUndefined();
    expect(harness.server.head).toBe(1n);

    harness.transports.pullFailures.push(new ConnectError("boom", Code.Internal));
    const failed = harness.driver.syncNow();
    const assertion = expect(failed).rejects.toThrow("boom");
    await settle();
    await assertion;
  });

  it("stop closes the poke stream and ends syncing", async () => {
    harness = startHarness({ pollIntervalMs: 1_000 });
    await settle();
    expect(harness.transports.streams).toBe(1);

    await harness.driver.stop();
    const calls = harness.transports.calls.length;
    await settle(10_000);

    expect(harness.transports.streams).toBe(0);
    expect(harness.transports.calls.length).toBe(calls);
    await expect(harness.driver.syncNow()).rejects.toThrow("stopped");
  });
});
