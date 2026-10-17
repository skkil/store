/** What the core reports about its sync state. */
export interface SyncStatus {
  /** The install's client id, sent with Pokes. */
  clientId: string;
  /** Mutations not yet acknowledged by a pull. */
  pending: number;
  /** Unresolved issues, such as rejected mutations, for the user to review. */
  issues: number;
  /** The scope version the local cursor has reached. */
  cursorVersion: bigint;
}

/** What applying one pull page did. */
export interface PullApplied {
  /** Synced tables whose rows changed. */
  changedTables: string[];
  /** True when more versions are waiting on the server. */
  more: boolean;
  /** True when the server answered Reset and the core dropped its synced rows. */
  reset: boolean;
}

/**
 * A synchronous `store-core` handle: the WASM build in a Web Worker or the uniffi module in React Native.
 * Requests and responses are binary `skkil.store.v1` messages.
 */
export interface CoreHandle {
  /** The next PushRequest with at most `limit` mutations, or null when nothing is waiting. */
  pushRequest(limit: number): Uint8Array | null;
  /** Applies a PushResponse. */
  applyPushResponse(bytes: Uint8Array): void;
  /** The next PullRequest asking for at most `limit` versions. */
  pullRequest(limit: number): Uint8Array;
  /** Applies a PullResponse. */
  applyPullResponse(bytes: Uint8Array): PullApplied;
  /** The current sync state. */
  status(): SyncStatus;
}
