const vscode = (name: string) => `vscode-icons:${name}`
const logo = (name: string) => `logos:${name}`

const ICONS = {
  image: vscode('file-type-image'),
  video: vscode('file-type-video'),
  audio: vscode('file-type-audio'),
  pdf: vscode('file-type-pdf2'),
  archive: vscode('file-type-zip'),
  word: vscode('file-type-word'),
  excel: vscode('file-type-excel'),
  powerpoint: vscode('file-type-powerpoint'),
  text: vscode('file-type-text'),
  json: vscode('file-type-json'),
  xml: vscode('file-type-xml'),
  markdown: vscode('file-type-markdown'),
  yaml: vscode('file-type-light-yaml'),
  unknown: vscode('default-file'),
  typescript: logo('typescript-icon'),
  javascript: logo('javascript'),
  python: logo('python'),
  java: logo('java'),
} as const

function byExtensions(...groups: Array<readonly [readonly string[], string]>) {
  return Object.fromEntries(
    groups.flatMap(([extensions, icon]) => extensions.map((ext) => [ext, icon])),
  ) as Record<string, string>
}

const extMap = byExtensions(
  [['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'], ICONS.image],
  [['mp4', 'avi', 'mov', 'mkv', 'webm'], ICONS.video],
  [['mp3', 'wav', 'flac', 'aac', 'ogg'], ICONS.audio],
  [['pdf'], ICONS.pdf],
  [['zip', 'rar', '7z', 'tar', 'gz', 'bz2'], ICONS.archive],
  [['doc', 'docx'], ICONS.word],
  [['xls', 'xlsx', 'csv'], ICONS.excel],
  [['ppt', 'pptx'], ICONS.powerpoint],
  [['txt', 'log'], ICONS.text],
  [['json'], ICONS.json],
  [['xml', 'xaml'], ICONS.xml],
  [['md'], ICONS.markdown],
  [['yaml', 'yml'], ICONS.yaml],
  [['ts'], ICONS.typescript],
  [['js'], ICONS.javascript],
  [['py'], ICONS.python],
  [['java'], ICONS.java],
)

function getAttachmentIcon(mimeType: string, fileName: string): string {
  const ext = fileName?.split('.').pop()?.toLowerCase() ?? ''

  if (ext && ext in extMap) return extMap[ext]

  // Fallback to MIME type
  if (mimeType.startsWith('image/')) return ICONS.image
  if (mimeType.startsWith('video/')) return ICONS.video
  if (mimeType.startsWith('audio/')) return ICONS.audio
  if (mimeType.includes('pdf')) return ICONS.pdf
  if (['zip', 'rar', 'tar', '7z'].some((type) => mimeType.includes(type)))
    return ICONS.archive
  if (['word', 'document'].some((type) => mimeType.includes(type))) return ICONS.word
  if (['excel', 'spreadsheet'].some((type) => mimeType.includes(type))) return ICONS.excel
  if (['powerpoint', 'presentation'].some((type) => mimeType.includes(type)))
    return ICONS.powerpoint
  if (mimeType.includes('text/')) return ICONS.text
  return ICONS.unknown
}

export { getAttachmentIcon }
