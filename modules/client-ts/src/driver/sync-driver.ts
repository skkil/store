import { fromBinary, toBinary } from "@bufbuild/protobuf";
import {
  PullRequestSchema,
  PullResponseSchema,
  PushRequestSchema,
  PushResponseSchema,
} from "../gen/skkil/store/v1/store_pb.js";
import { defaultLock } from "../lock/default-lock.js";
import type { LeaderLock } from "../lock/leader-lock.js";
import { isUnauthenticated } from "../transport/errors.js";
import { createPushTransport, type PushTransport } from "../transport/push-transport.js";
import { createStoreTransport, type StoreTransport } from "../transport/store-transport.js";
import { Backoff, type BackoffOptions } from "./backoff.js";
import type { CoreHandle } from "./core-handle.js";
import type { DriverStatus } from "./driver-status.js";
import { PokeListener } from "./poke-listener.js";
import { SyncLoop } from "./sync-loop.js";
import { TokenCache, type SyncTokenGrant } from "./token-cache.js";

/** Options for {@link createSyncDriver}. */
export interface SyncDriverOptions {
  /** The store-core handle that builds requests and applies responses. */
  core: CoreHandle;
  /** The store service's base URL, for Pull and Pokes. */
  storeUrl: string;
  /** The product server's base URL, for Push. */
  pushUrl: string;
  /** Fetches a sync token from the product server. */
  getSyncToken: () => Promise<SyncTokenGrant>;
  /** The product's auth headers for each push. */
  getPushHeaders?: () => HeadersInit | Promise<HeadersInit>;
  /** Called with the synced tables a pull changed. */
  onChange?: (changedTables: string[]) => void;
  /** Called whenever the reported status changes. */
  onStatus?: (status: DriverStatus) => void;
  /** Called with every failed sync cycle or poke stream, for logging. */
  onError?: (error: unknown) => void;
  /** The fetch implementation; React Native must pass `fetch` from `expo/fetch` for streaming. */
  fetch?: typeof globalThis.fetch;
  /** Leader election; defaults to a Web Lock where available, otherwise none. */
  lock?: LeaderLock;
  /** The Web Lock name used by the default lock. */
  lockName?: string;
  /** Replaces the Connect transports, for tests or custom networking. */
  transports?: { store?: StoreTransport; push?: PushTransport };
  /** Most mutations per push. */
  pushLimit?: number;
  /** Most versions per pull page. */
  pullLimit?: number;
  /** How often to pull even without pokes. */
  pollIntervalMs?: number;
  /** Retry and reconnect backoff bounds. */
  backoff?: Partial<BackoffOptions>;
  /** How long before expiry a sync token is replaced. */
  tokenRefreshMarginMs?: number;
  /** Random source in [0, 1) for backoff jitter. */
  random?: () => number;
  /** The clock in epoch milliseconds. */
  now?: () => number;
}

/** A running sync driver. */
export interface SyncDriver {
  /** Pushes and pulls now; resolves when that cycle completes and rejects if it fails. */
  syncNow(): Promise<void>;
  /** Stops syncing, closes the poke stream and gives up leadership. */
  stop(): Promise<void>;
  /** The last reported status. */
  status(): DriverStatus;
}

const DEFAULTS = {
  lockName: "skkil-store-sync",
  pushLimit: 100,
  pullLimit: 500,
  pollIntervalMs: 60_000,
  backoff: { baseMs: 1_000, maxMs: 60_000 },
  tokenRefreshMarginMs: 60_000,
};

const MAX_PUSH_PAGES = 100;

/**
 * Starts syncing `core`: once this context leads, it pushes pending mutations, then pulls page by page,
 * on start, after pokes ahead of the cursor, on reconnect, on a timer, and when the device comes back
 * online or the page becomes visible. Failures are retried with full-jitter backoff; mutations stay in
 * the core until a pull acknowledges them.
 */
export function createSyncDriver(options: SyncDriverOptions): SyncDriver {
  return new DefaultSyncDriver(options);
}

class DefaultSyncDriver implements SyncDriver {
  private readonly core: CoreHandle;
  private readonly store: StoreTransport;
  private readonly push: PushTransport;
  private readonly tokens: TokenCache;
  private readonly loop: SyncLoop;
  private readonly pokes: PokeListener;
  private readonly abort = new AbortController();
  private readonly now: () => number;
  private readonly cleanups: (() => void)[] = [];
  private running: Promise<void> = Promise.resolve();
  private online = false;
  private lastSyncedAt: number | null = null;
  private reported: DriverStatus | null = null;

  constructor(private readonly options: SyncDriverOptions) {
    this.core = options.core;
    this.now = options.now ?? Date.now;
    const fetch = options.fetch ? { fetch: options.fetch } : {};
    this.store =
      options.transports?.store ?? createStoreTransport({ baseUrl: options.storeUrl, ...fetch });
    this.push =
      options.transports?.push ??
      createPushTransport({
        baseUrl: options.pushUrl,
        ...fetch,
        ...(options.getPushHeaders ? { getHeaders: options.getPushHeaders } : {}),
      });
    const backoff = { ...DEFAULTS.backoff, ...options.backoff };
    const random = options.random ?? Math.random;
    this.tokens = new TokenCache(
      options.getSyncToken,
      this.now,
      options.tokenRefreshMarginMs ?? DEFAULTS.tokenRefreshMarginMs,
    );
    this.loop = new SyncLoop(
      (signal) => this.cycle(signal),
      new Backoff(backoff, random),
      this.abort.signal,
      (error) => {
        this.setOnline(false);
        options.onError?.(error);
      },
    );
    this.pokes = new PokeListener(
      this.core,
      this.store,
      this.tokens,
      new Backoff(backoff, random),
      {
        onConnected: () => {
          this.setOnline(true);
          this.loop.kick();
        },
        onAhead: () => {
          this.loop.kick();
        },
        onFailure: (error) => options.onError?.(error),
      },
    );
    this.running = this.start();
  }

  syncNow(): Promise<void> {
    return this.loop.request();
  }

  async stop(): Promise<void> {
    this.abort.abort();
    for (const cleanup of this.cleanups.splice(0)) cleanup();
    await this.loop.stop();
    await this.running;
  }

  status(): DriverStatus {
    return this.reported ?? this.currentStatus();
  }

  private async start(): Promise<void> {
    const signal = this.abort.signal;
    try {
      await (this.options.lock ?? defaultLock(this.options.lockName ?? DEFAULTS.lockName)).acquire(
        signal,
      );
    } catch {
      return;
    }
    if (signal.aborted) return;
    this.report();
    this.watchEnvironment();
    this.loop.enable();
    this.loop.kick();
    await this.pokes.run(signal);
  }

  private watchEnvironment(): void {
    const interval = setInterval(() => {
      this.loop.kick();
    }, this.options.pollIntervalMs ?? DEFAULTS.pollIntervalMs);
    this.cleanups.push(() => {
      clearInterval(interval);
    });
    if (typeof window === "undefined" || typeof window.addEventListener !== "function") return;
    this.listen(window, "online", () => {
      this.pokes.reconnectNow();
      this.loop.kick();
    });
    this.listen(window, "offline", () => {
      this.setOnline(false);
    });
    if (typeof document === "undefined") return;
    this.listen(document, "visibilitychange", () => {
      if (document.visibilityState === "visible") this.loop.kick();
    });
  }

  private listen(target: EventTarget, type: string, handler: () => void): void {
    target.addEventListener(type, handler);
    this.cleanups.push(() => {
      target.removeEventListener(type, handler);
    });
  }

  private async cycle(signal: AbortSignal): Promise<void> {
    await this.pushAll(signal);
    const reset = await this.pullAll(signal);
    if (reset) this.loop.kick();
  }

  private async pushAll(signal: AbortSignal): Promise<void> {
    let previous: Uint8Array | null = null;
    for (let page = 0; page < MAX_PUSH_PAGES; page++) {
      const bytes = this.core.pushRequest(this.options.pushLimit ?? DEFAULTS.pushLimit);
      if (bytes === null || (previous !== null && sameBytes(bytes, previous))) return;
      const response = await this.push.push(fromBinary(PushRequestSchema, bytes), signal);
      this.core.applyPushResponse(toBinary(PushResponseSchema, response));
      this.setOnline(true);
      previous = bytes;
    }
  }

  private async pullAll(signal: AbortSignal): Promise<boolean> {
    let resets = 0;
    for (;;) {
      const request = fromBinary(
        PullRequestSchema,
        this.core.pullRequest(this.options.pullLimit ?? DEFAULTS.pullLimit),
      );
      const response = await this.withToken((token) => this.store.pull(request, token, signal));
      const applied = this.core.applyPullResponse(toBinary(PullResponseSchema, response));
      this.setOnline(true);
      if (applied.changedTables.length > 0) this.options.onChange?.(applied.changedTables);
      if (applied.reset) {
        resets += 1;
        if (resets > 1) throw new Error("store answered Reset twice in one pull");
      }
      if (!applied.more && !applied.reset) {
        this.lastSyncedAt = this.now();
        this.report();
        return resets > 0;
      }
    }
  }

  private async withToken<T>(call: (token: string) => Promise<T>): Promise<T> {
    try {
      return await call(await this.tokens.get());
    } catch (error) {
      if (!isUnauthenticated(error)) throw error;
      this.tokens.invalidate();
      return call(await this.tokens.get());
    }
  }

  private setOnline(online: boolean): void {
    this.online = online;
    this.report();
  }

  private report(): void {
    if (this.abort.signal.aborted) return;
    const status = this.currentStatus();
    const last = this.reported;
    if (
      last?.online === status.online &&
      last.pending === status.pending &&
      last.issues === status.issues &&
      last.lastSyncedAt === status.lastSyncedAt
    ) {
      return;
    }
    this.reported = status;
    this.options.onStatus?.(status);
  }

  private currentStatus(): DriverStatus {
    const { pending, issues } = this.core.status();
    return { online: this.online, pending, lastSyncedAt: this.lastSyncedAt, issues };
  }
}

function sameBytes(a: Uint8Array, b: Uint8Array): boolean {
  return a.length === b.length && a.every((byte, index) => byte === b[index]);
}
