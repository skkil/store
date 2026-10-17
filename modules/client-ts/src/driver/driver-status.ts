/** What the driver reports to the UI through `onStatus`. */
export interface DriverStatus {
  /** True while the last request reached the servers. */
  online: boolean;
  /** Mutations not yet acknowledged by a pull. */
  pending: number;
  /** When a pull last completed, in epoch milliseconds, or null before the first one. */
  lastSyncedAt: number | null;
  /** Unresolved issues reported by the core. */
  issues: number;
}
