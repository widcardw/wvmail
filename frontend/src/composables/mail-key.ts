export interface MailIdentity {
  uid: number
  account_id?: string
  folder_name?: string
}

/**
 * Stable key for one message. Selection and in-memory updates use
 * account+folder+UID: aggregate and search views can contain the same UID in
 * several mailboxes, and the same number in another folder is another mail.
 *
 * The zero-width spaces keep the three parts unambiguous even when an account
 * ID or folder name contains a colon.
 */
export function mailKey(accountId: string, folder: string, uid: number): string {
  return `${accountId}\u200b${folder}\u200b${uid}`
}

/** Key for a list row, falling back to the open folder for plain folder pages. */
export function mailKeyFor(
  email: MailIdentity,
  fallbackAccountId?: string | null,
  fallbackFolder?: string | null,
): string {
  const accountId = email.account_id || fallbackAccountId
  const folder = email.folder_name || fallbackFolder
  return accountId && folder ? mailKey(accountId, folder, email.uid) : ''
}

/** Key for the currently selected message, or '' when nothing is selected. */
export function selectedMailKey(
  accountId?: string | null,
  folder?: string | null,
  uid?: number | null,
): string {
  return uid != null && accountId && folder ? mailKey(accountId, folder, uid) : ''
}
