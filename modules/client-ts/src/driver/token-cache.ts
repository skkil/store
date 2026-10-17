/** A sync token, optionally with its expiry; without one the JWT's `exp` claim is used. */
export type SyncTokenGrant = string | { token: string; expiresAt?: number | Date };

/** Keeps the current sync token, fetching a new one shortly before it expires or after a 401. */
export class TokenCache {
  private token: string | null = null;
  private expiresAt: number | null = null;
  private inFlight: Promise<string> | null = null;

  constructor(
    private readonly fetchToken: () => Promise<SyncTokenGrant>,
    private readonly now: () => number,
    private readonly refreshMarginMs: number,
  ) {}

  /** A token valid for at least the refresh margin, fetching one if needed; concurrent callers share one fetch. */
  get(): Promise<string> {
    if (this.token !== null && !this.expiresSoon()) return Promise.resolve(this.token);
    this.inFlight ??= this.refresh().finally(() => {
      this.inFlight = null;
    });
    return this.inFlight;
  }

  /** Drops the current token, so the next `get` fetches a new one. */
  invalidate(): void {
    this.token = null;
    this.expiresAt = null;
  }

  private expiresSoon(): boolean {
    return this.expiresAt !== null && this.now() >= this.expiresAt - this.refreshMarginMs;
  }

  private async refresh(): Promise<string> {
    const grant = await this.fetchToken();
    const token = typeof grant === "string" ? grant : grant.token;
    const expiresAt = typeof grant === "string" ? undefined : grant.expiresAt;
    this.token = token;
    this.expiresAt = expiresAt === undefined ? jwtExpiry(token) : new Date(expiresAt).getTime();
    return token;
  }
}

function jwtExpiry(token: string): number | null {
  const payload = token.split(".")[1];
  if (payload === undefined || typeof globalThis.atob !== "function") return null;
  try {
    const json = globalThis.atob(payload.replace(/-/g, "+").replace(/_/g, "/"));
    const exp: unknown = (JSON.parse(json) as { exp?: unknown }).exp;
    return typeof exp === "number" ? exp * 1000 : null;
  } catch {
    return null;
  }
}
