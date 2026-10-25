/** IMAP system flags are case-insensitive, so every read goes through one helper. */
export function hasMailFlag(flags: readonly string[] | undefined, flag: string): boolean {
  const wanted = flag.toLowerCase()
  return (flags ?? []).some((current) => current.toLowerCase() === wanted)
}

export function isFlagged(flags: readonly string[] | undefined): boolean {
  return hasMailFlag(flags, '\\Flagged')
}

/** Add or remove one flag without reordering the rest of the server's list. */
export function withMailFlag(
  flags: readonly string[] | undefined,
  flag: string,
  enabled: boolean,
): string[] {
  const wanted = flag.toLowerCase()
  const out: string[] = []
  let found = false
  for (const current of flags ?? []) {
    if (current.toLowerCase() === wanted) {
      found = true
      if (!enabled) continue
    }
    out.push(current)
  }
  if (enabled && !found) out.push(flag)
  return out
}
