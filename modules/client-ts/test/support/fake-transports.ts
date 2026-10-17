import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import type {
  Poke,
  PokesRequest,
  PullRequest,
  PullResponse,
  PushRequest,
  PushResponse,
  PushTransport,
  StoreTransport,
} from "../../src/index.js";
import { PokeSchema } from "../../src/index.js";
import type { FakeServer } from "./fake-server.js";

export class FakeTransports {
  readonly calls: string[] = [];
  readonly pullTokens: string[] = [];
  readonly pokeTokens: string[] = [];
  pushFailures: Error[] = [];
  pullFailures: Error[] = [];
  pokeFailures: Error[] = [];
  pokesDown = false;
  private pokeQueue: Poke[] = [];
  private wakePokes: (() => void) | null = null;
  private openStreams = 0;

  constructor(private readonly server: FakeServer) {}

  get streams(): number {
    return this.openStreams;
  }

  poke(version: bigint): void {
    this.pokeQueue.push(create(PokeSchema, { version }));
    this.wakePokes?.();
  }

  readonly push: PushTransport = {
    push: (request: PushRequest): Promise<PushResponse> => {
      this.calls.push("push");
      const failure = this.pushFailures.shift();
      if (failure !== undefined) return Promise.reject(failure);
      return Promise.resolve(this.server.push(request));
    },
  };

  readonly store: StoreTransport = {
    pull: (request: PullRequest, token: string): Promise<PullResponse> => {
      this.calls.push("pull");
      this.pullTokens.push(token);
      const failure = this.pullFailures.shift();
      if (failure !== undefined) return Promise.reject(failure);
      return Promise.resolve(this.server.pull(request));
    },
    pokes: (_request: PokesRequest, token: string, signal: AbortSignal, onOpen: () => void) => {
      this.pokeTokens.push(token);
      const failure =
        this.pokeFailures.shift() ??
        (this.pokesDown ? new ConnectError("pokes are down", Code.Unavailable) : undefined);
      const stream = async function* (this: FakeTransports): AsyncGenerator<Poke> {
        if (failure !== undefined) throw failure;
        this.openStreams += 1;
        try {
          onOpen();
          while (!signal.aborted) {
            const next = this.pokeQueue.shift();
            if (next) {
              yield next;
              continue;
            }
            await new Promise<void>((resolve) => {
              this.wakePokes = resolve;
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
          this.openStreams -= 1;
        }
      };
      return stream.call(this);
    },
  };
}
