import { create } from "@bufbuild/protobuf";
import type { PullRequest, PullResponse, PushRequest, PushResponse } from "../../src/index.js";
import {
  MutationResult_Status,
  PullResponseSchema,
  PushResponseSchema,
  type Version,
  VersionSchema,
} from "../../src/index.js";

export class FakeServer {
  readonly versions: Version[] = [];
  private lastMutation = new Map<string, bigint>();

  push(request: PushRequest): PushResponse {
    const results = request.mutations.map((mutation) => {
      const last = this.lastMutation.get(request.clientId) ?? 0n;
      if (mutation.mutationId <= last) {
        return { mutationId: mutation.mutationId, status: MutationResult_Status.DUPLICATE };
      }
      this.lastMutation.set(request.clientId, mutation.mutationId);
      const version = this.write(request.clientId, mutation.mutationId);
      return { mutationId: mutation.mutationId, status: MutationResult_Status.APPLIED, version };
    });
    return create(PushResponseSchema, { results });
  }

  write(clientId = "00000000-0000-0000-0000-000000000000", mutationId = 0n): bigint {
    const version = BigInt(this.versions.length + 1);
    this.versions.push(
      create(VersionSchema, {
        version,
        clientId,
        mutationId,
        changes: [{ table: "task", rowId: `row-${version.toString()}`, data: new Uint8Array() }],
      }),
    );
    return version;
  }

  pull(request: PullRequest): PullResponse {
    const from = Number(request.cursor?.version ?? 0n);
    const limit = request.limit === 0 ? Infinity : request.limit;
    const page = this.versions.slice(from, from + limit);
    return create(PullResponseSchema, {
      body: {
        case: "entries",
        value: { versions: page, more: from + page.length < this.versions.length },
      },
      lastMutationId: this.lastMutation.get(request.clientId) ?? 0n,
      headVersion: BigInt(this.versions.length),
      epoch: 1,
    });
  }

  get head(): bigint {
    return BigInt(this.versions.length);
  }
}
