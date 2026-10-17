import { create } from "@bufbuild/protobuf";
import { PokesRequestSchema } from "../gen/skkil/store/v1/store_pb.js";
import type { StoreTransport } from "../transport/store-transport.js";
import { isUnauthenticated } from "../transport/errors.js";
import { aborted } from "./aborted.js";
import type { Backoff } from "./backoff.js";
import type { CoreHandle } from "./core-handle.js";
import { sleep } from "./sleep.js";
import type { TokenCache } from "./token-cache.js";

/** Callbacks from {@link PokeListener}. */
export interface PokeEvents {
  /** The stream (re)connected; the cursor may have fallen behind meanwhile. */
  onConnected(): void;
  /** A poke announced a version ahead of the local cursor. */
  onAhead(): void;
  /** The stream failed; it reconnects after a backoff delay. */
  onFailure(error: unknown): void;
}

/** Keeps the Pokes stream open, reconnecting with full-jitter backoff. */
export class PokeListener {
  private wake = new AbortController();

  constructor(
    private readonly core: CoreHandle,
    private readonly transport: StoreTransport,
    private readonly tokens: TokenCache,
    private readonly backoff: Backoff,
    private readonly events: PokeEvents,
  ) {}

  /** Listens until `signal` aborts. */
  async run(signal: AbortSignal): Promise<void> {
    while (!signal.aborted) {
      this.wake = new AbortController();
      try {
        await this.listen(signal);
      } catch (error) {
        if (aborted(signal)) return;
        if (isUnauthenticated(error)) this.tokens.invalidate();
        this.events.onFailure(error);
      }
      await sleep(this.backoff.next(), signal, this.wake.signal);
    }
  }

  /** Cuts a pending reconnect delay short, for example when the device comes back online. */
  reconnectNow(): void {
    this.wake.abort();
  }

  private async listen(signal: AbortSignal): Promise<void> {
    const token = await this.tokens.get();
    const request = create(PokesRequestSchema, {
      protocolVersion: 1,
      clientId: this.core.status().clientId,
    });
    const opened = () => {
      this.backoff.reset();
      this.events.onConnected();
    };
    for await (const poke of this.transport.pokes(request, token, signal, opened)) {
      if (poke.version > this.core.status().cursorVersion) this.events.onAhead();
    }
  }
}
