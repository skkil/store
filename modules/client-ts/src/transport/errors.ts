import { Code, ConnectError } from "@connectrpc/connect";

/** True when a transport call failed with 401 / `unauthenticated`. */
export function isUnauthenticated(error: unknown): boolean {
  return error instanceof ConnectError && error.code === Code.Unauthenticated;
}
