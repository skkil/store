import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  StoreService,
  type Poke,
  type PokesRequest,
  type PullRequest,
  type PullResponse,
} from "../gen/skkil/store/v1/store_pb.js";

/** Pull and Pokes on the store service, authorised by a sync token. */
export interface StoreTransport {
  /** Pulls one page of versions. */
  pull(request: PullRequest, token: string, signal: AbortSignal): Promise<PullResponse>;
  /** Opens the Pokes stream; `onOpen` runs once the server has accepted it. */
  pokes(
    request: PokesRequest,
    token: string,
    signal: AbortSignal,
    onOpen: () => void,
  ): AsyncIterable<Poke>;
}

/** Options for {@link createStoreTransport}. */
export interface StoreTransportOptions {
  /** The store service's base URL. */
  baseUrl: string;
  /** A fetch with streaming response bodies; pass `expo/fetch` in React Native. */
  fetch?: typeof globalThis.fetch;
}

/** A Connect (binary protobuf) client for `skkil.store.v1.StoreService`. */
export function createStoreTransport(options: StoreTransportOptions): StoreTransport {
  const transport = createConnectTransport({
    baseUrl: options.baseUrl,
    useBinaryFormat: true,
    ...(options.fetch ? { fetch: options.fetch } : {}),
  });
  const client = createClient(StoreService, transport);
  return {
    pull: (request, token, signal) => client.pull(request, { signal, headers: bearer(token) }),
    pokes: (request, token, signal, onOpen) =>
      client.pokes(request, { signal, headers: bearer(token), onHeader: onOpen }),
  };
}

function bearer(token: string): HeadersInit {
  return { Authorization: `Bearer ${token}` };
}
