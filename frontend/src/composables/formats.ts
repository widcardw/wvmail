// `locale` is the app language (locale store), not the system one: every Ark
// component in the app follows LocaleProvider, so dates must too.
function formatDate(dateStr: string, locale?: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleString(locale, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatDateRecent(dateStr: string, locale?: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const isToday = date.toDateString() === now.toDateString()
  if (isToday) {
    return date.toLocaleTimeString(locale, { hour: '2-digit', minute: '2-digit' })
  }
  return date.toLocaleDateString(locale, { month: 'short', day: 'numeric' })
}

function getInitials(name: string, addr: string): string {
  return (name || addr || '?').charAt(0).toUpperCase()
}

function truncateSubject(subject: string, maxLen = 60): string {
  if (!subject) return '(No Subject)'
  return subject.length > maxLen ? subject.slice(0, maxLen) + '…' : subject
}

function displaySender(email: { from_name?: string; from_addr?: string; from?: string }): string {
  if (email.from_name) return email.from_name
  if (email.from_addr) return email.from_addr.split('@')[0]
  if (email.from) return email.from
  return 'Unknown'
}

export { displaySender, truncateSubject, formatDateRecent, formatDate, getInitials }
