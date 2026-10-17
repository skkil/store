import { describe, expect, it, vi } from "vitest";
import { TokenCache } from "../../src/driver/token-cache.js";

function jwt(exp: number): string {
  const encode = (value: object) => Buffer.from(JSON.stringify(value)).toString("base64url");
  return `${encode({ alg: "ES256" })}.${encode({ exp })}.signature`;
}

describe("TokenCache", () => {
  it("reuses a token until the refresh margin before its JWT exp", async () => {
    let now = 1_000_000;
    const fetchToken = vi
      .fn<() => Promise<string>>()
      .mockResolvedValueOnce(jwt(1_000 + 900))
      .mockResolvedValueOnce("second");
    const cache = new TokenCache(fetchToken, () => now, 60_000);

    await cache.get();
    now = 1_000_000 + 839_000;
    await cache.get();
    expect(fetchToken).toHaveBeenCalledTimes(1);

    now = 1_000_000 + 840_000;
    expect(await cache.get()).toBe("second");
  });

  it("shares one fetch between concurrent callers and refetches after invalidate", async () => {
    const fetchToken = vi.fn(() => Promise.resolve({ token: "t", expiresAt: new Date(10 ** 13) }));
    const cache = new TokenCache(fetchToken, () => 0, 60_000);

    await Promise.all([cache.get(), cache.get()]);
    expect(fetchToken).toHaveBeenCalledTimes(1);

    cache.invalidate();
    await cache.get();
    expect(fetchToken).toHaveBeenCalledTimes(2);
  });

  it("keeps a token without a readable expiry until it is invalidated", async () => {
    const fetchToken = vi.fn(() => Promise.resolve("opaque"));
    const cache = new TokenCache(fetchToken, () => 0, 60_000);

    await cache.get();
    await cache.get();
    expect(fetchToken).toHaveBeenCalledTimes(1);
  });
});
