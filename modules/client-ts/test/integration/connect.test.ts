import { create } from "@bufbuild/protobuf";
import { Code, ConnectError, type ConnectRouter, type HandlerContext } from "@connectrpc/connect";
import { connectNodeAdapter } from "@connectrpc/connect-node";
import { createServer, type Server } from "node:http";
import type { AddressInfo } from "node:net";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  createSyncDriver,
  noopLock,
  PokeSchema,
  PushService,
  StoreService,
  type Poke,
  type SyncDriver,
} from "../../src/index.js";
import { FakeCore } from "../support/fake-core.js";
import { FakeServer } from "../support/fake-server.js";

class Pokes {
  private listeners = new Set<(poke: Poke) => void>();

  publish(version: bigint): void {
    for (const listener of this.listeners) listener(create(PokeSchema, { version }));
  }

  get subscribers(): number {
    return this.listeners.size;
  }

  async *subscribe(signal: AbortSignal): AsyncGenerator<Poke> {
    const queue: Poke[] = [];
    let wake: (() => void) | null = null;
    const listener = (poke: Poke) => {
      queue.push(poke);
      wake?.();
    };
    this.listeners.add(listener);
    try {
      while (!signal.aborted) {
        const next = queue.shift();
        if (next) {
          yield next;
          continue;
        }
        await new Promise<void>((resolve) => {
          wake = resolve;
          signal.addEventListener(
            "abort",
            () => {
              resolve();
            },
            { once: true },
          );
        });
      }
    } finally {
      this.listeners.delete(listener);
    }
  }
}

async function until(condition: () => boolean, timeoutMs = 5_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (!condition()) {
    if (Date.now() > deadline) throw new Error("condition not met in time");
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
}

describe("over Connect", () => {
  const log = new FakeServer();
  const pokes = new Pokes();
  const validTokens = new Set<string>();
  const seenTokens: string[] = [];
  let httpServer: Server;
  let baseUrl: string;
  let driver: SyncDriver | null = null;

  function authorize(context: HandlerContext): void {
    const token = context.requestHeader.get("authorization")?.replace(/^Bearer /, "") ?? "";
    seenTokens.push(token);
    if (!validTokens.has(token)) throw new ConnectError("token expired", Code.Unauthenticated);
  }

  const routes = (router: ConnectRouter) => {
    router.service(StoreService, {
      pull(request, context) {
        authorize(context);
        return log.pull(request);
      },
      pokes(_request, context) {
        authorize(context);
        return pokes.subscribe(context.signal);
      },
    });
    router.service(PushService, {
      push(request, context) {
        if (context.requestHeader.get("x-product-auth") !== "secret") {
          throw new ConnectError("sign in", Code.Unauthenticated);
        }
        if (request.protocolVersion !== 1) {
          throw new ConnectError("protocol", Code.FailedPrecondition);
        }
        const response = log.push(request);
        pokes.publish(log.head);
        return response;
      },
    });
  };

  beforeEach(async () => {
    httpServer = createServer(connectNodeAdapter({ routes }));
    await new Promise<void>((resolve) => httpServer.listen(0, "127.0.0.1", resolve));
    baseUrl = `http://127.0.0.1:${(httpServer.address() as AddressInfo).port.toString()}`;
  });

  afterEach(async () => {
    await driver?.stop();
    httpServer.closeAllConnections();
    await new Promise((resolve) => httpServer.close(resolve));
  });

  it("pushes, pulls, follows pokes and refreshes a rejected token", async () => {
    const core = new FakeCore();
    core.mutate(2);
    const changes: string[][] = [];
    let issued = 0;
    driver = createSyncDriver({
      core,
      storeUrl: baseUrl,
      pushUrl: baseUrl,
      getSyncToken: () => {
        issued += 1;
        const token = `token-${issued.toString()}`;
        if (issued > 1) validTokens.add(token);
        return Promise.resolve(token);
      },
      getPushHeaders: () => ({ "x-product-auth": "secret" }),
      onChange: (tables) => changes.push(tables),
      lock: noopLock(),
      backoff: { baseMs: 20, maxMs: 100 },
    });

    await until(() => core.cursorVersion === 2n && core.status().pending === 0);
    expect(log.head).toBe(2n);
    expect(seenTokens).toContain("token-1");
    expect(seenTokens.filter((token) => token !== "token-1").length).toBeGreaterThan(0);
    expect(changes.flat()).toContain("task");

    await until(() => pokes.subscribers === 1);
    log.write();
    pokes.publish(log.head);
    await until(() => core.cursorVersion === 3n);

    core.mutate(1);
    await driver.syncNow();
    expect(log.head).toBe(4n);
    expect(core.cursorVersion).toBe(4n);
    expect(driver.status()).toMatchObject({ online: true, pending: 0 });
  });
});
