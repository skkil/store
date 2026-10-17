import { describe, expect, it } from "vitest";
import { Backoff } from "../../src/driver/backoff.js";

describe("Backoff", () => {
  it("draws each delay uniformly below a doubling, capped ceiling", () => {
    const backoff = new Backoff({ baseMs: 100, maxMs: 1_000 }, () => 0.999);

    expect([1, 2, 3, 4, 5, 6].map(() => backoff.next())).toEqual([99, 199, 399, 799, 999, 999]);
  });

  it("starts over after reset", () => {
    const backoff = new Backoff({ baseMs: 100, maxMs: 1_000 }, () => 0.5);
    backoff.next();
    backoff.next();
    backoff.reset();

    expect(backoff.next()).toBe(50);
  });
});
