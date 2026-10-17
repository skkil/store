import { create, fromBinary, toBinary } from "@bufbuild/protobuf";
import type { CoreHandle, PullApplied, SyncStatus } from "../../src/index.js";
import {
  MutationSchema,
  PullRequestSchema,
  PullResponseSchema,
  PushRequestSchema,
  PushResponseSchema,
} from "../../src/index.js";

export class FakeCore implements CoreHandle {
  readonly calls: string[] = [];
  readonly clientId = "0192f000-0000-7000-8000-000000000001";
  cursorVersion = 0n;
  issues = 0;
  private nextMutationId = 1n;
  private unsent: bigint[] = [];
  private unacked: bigint[] = [];

  mutate(count = 1): void {
    for (let i = 0; i < count; i++) {
      const id = this.nextMutationId++;
      this.unsent.push(id);
      this.unacked.push(id);
    }
  }

  pushRequest(limit: number): Uint8Array | null {
    const batch = this.unsent.slice(0, limit);
    if (batch.length === 0) return null;
    this.calls.push("pushRequest");
    return toBinary(
      PushRequestSchema,
      create(PushRequestSchema, {
        protocolVersion: 1,
        clientId: this.clientId,
        mutations: batch.map((id) =>
          create(MutationSchema, {
            mutationId: id,
            name: "task.create",
            args: new TextEncoder().encode("{}"),
          }),
        ),
      }),
    );
  }

  applyPushResponse(bytes: Uint8Array): void {
    this.calls.push("applyPushResponse");
    const response = fromBinary(PushResponseSchema, bytes);
    const done = new Set(response.results.map((result) => result.mutationId));
    this.unsent = this.unsent.filter((id) => !done.has(id));
  }

  pullRequest(limit: number): Uint8Array {
    this.calls.push("pullRequest");
    return toBinary(
      PullRequestSchema,
      create(PullRequestSchema, {
        protocolVersion: 1,
        clientId: this.clientId,
        cursor: { epoch: 1, version: this.cursorVersion },
        limit,
      }),
    );
  }

  applyPullResponse(bytes: Uint8Array): PullApplied {
    this.calls.push("applyPullResponse");
    const response = fromBinary(PullResponseSchema, bytes);
    this.unacked = this.unacked.filter((id) => id > response.lastMutationId);
    if (response.body.case === "reset") {
      this.cursorVersion = 0n;
      this.unsent = [...this.unacked];
      return { changedTables: [], more: false, reset: true };
    }
    const entries = response.body.value;
    const tables = new Set<string>();
    for (const version of entries?.versions ?? []) {
      this.cursorVersion = version.version;
      for (const change of version.changes) tables.add(change.table);
    }
    return { changedTables: [...tables], more: entries?.more ?? false, reset: false };
  }

  status(): SyncStatus {
    return {
      clientId: this.clientId,
      pending: this.unacked.length,
      issues: this.issues,
      cursorVersion: this.cursorVersion,
    };
  }
}
