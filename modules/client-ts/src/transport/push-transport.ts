import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import {
  PushService,
  type PushRequest,
  type PushResponse,
} from "../gen/skkil/store/v1/store_pb.js";

/** Push on the product server, authorised by the product's own credentials. */
export interface PushTransport {
  /** Pushes one batch of mutations. */
  push(request: PushRequest, signal: AbortSignal): Promise<PushResponse>;
}

/** Options for {@link createPushTransport}. */
export interface PushTransportOptions {
  /** The product server's base URL; requests go to `<baseUrl>/skkil.store.v1.PushService/Push`. */
  baseUrl: string;
  /** Headers for each push, such as the product's `Authorization`; called before every request. */
  getHeaders?: () => HeadersInit | Promise<HeadersInit>;
  /** The fetch implementation; pass `expo/fetch` in React Native. */
  fetch?: typeof globalThis.fetch;
}

/** A Connect (binary protobuf) client for `skkil.store.v1.PushService`. */
export function createPushTransport(options: PushTransportOptions): PushTransport {
  const transport = createConnectTransport({
    baseUrl: options.baseUrl,
    useBinaryFormat: true,
    ...(options.fetch ? { fetch: options.fetch } : {}),
  });
  const client = createClient(PushService, transport);
  return {
    push: async (request, signal) =>
      client.push(request, { signal, headers: (await options.getHeaders?.()) ?? {} }),
  };
}
