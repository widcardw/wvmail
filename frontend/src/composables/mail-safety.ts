/**
 * The spam gate, as one decision.
 *
 * A message stays behind the gate when all three hold: the server flagged it as
 * junk (a spam/Junk folder, or the `\Junk` flag — see `isSuspiciousMail` in
 * services/emailbody.go), the user asked for suspicious mail to be intercepted
 * (Settings → Security), and this particular message has not been revealed yet.
 *
 * The second condition is the switch's whole job. It defaults to on, which is why
 * the gate used to be unconditional: the flag existed but nothing read it, so the
 * switch was a placebo. Turning it off is the user saying "show me junk as it is"
 * — the remote-content switch still applies on top.
 */
export function isSpamGated(isSuspicious: boolean, intercept: boolean, revealed: boolean): boolean {
  return isSuspicious && intercept && !revealed
}
