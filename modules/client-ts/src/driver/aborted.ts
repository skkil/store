/** Reads `signal.aborted` afresh, since it can change across an `await`. */
export function aborted(signal: AbortSignal): boolean {
  return signal.aborted;
}
