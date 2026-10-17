import { vi } from "vitest";
import {
  createSyncDriver,
  noopLock,
  type DriverStatus,
  type SyncDriver,
  type SyncDriverOptions,
} from "../../src/index.js";
import { FakeCore } from "./fake-core.js";
import { FakeServer } from "./fake-server.js";
import { FakeTransports } from "./fake-transports.js";

export interface Harness {
  driver: SyncDriver;
  core: FakeCore;
  server: FakeServer;
  transports: FakeTransports;
  statuses: DriverStatus[];
  changes: string[][];
  errors: unknown[];
  getSyncToken: ReturnType<typeof vi.fn<() => Promise<{ token: string; expiresAt: number }>>>;
}

export function startHarness(
  overrides: Partial<SyncDriverOptions> = {},
  prepare: (core: FakeCore, server: FakeServer, transports: FakeTransports) => void = () =>
    undefined,
): Harness {
  const core = new FakeCore();
  const server = new FakeServer();
  const transports = new FakeTransports(server);
  prepare(core, server, transports);
  const statuses: DriverStatus[] = [];
  const changes: string[][] = [];
  const errors: unknown[] = [];
  let issued = 0;
  const getSyncToken = vi.fn(() => {
    issued += 1;
    return Promise.resolve({
      token: `token-${issued.toString()}`,
      expiresAt: Date.now() + 15 * 60_000,
    });
  });
  const driver = createSyncDriver({
    core,
    storeUrl: "http://store.test",
    pushUrl: "http://product.test",
    getSyncToken,
    transports,
    lock: noopLock(),
    onStatus: (status) => statuses.push(status),
    onChange: (tables) => changes.push(tables),
    onError: (error) => errors.push(error),
    random: () => 0.5,
    ...overrides,
  });
  return { driver, core, server, transports, statuses, changes, errors, getSyncToken };
}

export async function settle(ms = 0): Promise<void> {
  await vi.advanceTimersByTimeAsync(ms);
}
