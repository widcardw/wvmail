function getAttachmentIcon(mimeType: string, fileName: string): string {
  const nameSlices = fileName?.split('.')
  const ext = nameSlices?.[nameSlices.length - 1]?.toLowerCase() ?? ''

  // Extension-based lookup
  const extMap: Record<string, string> = {
    // Images
    jpg: 'vscode-icons:file-type-image',
    jpeg: 'vscode-icons:file-type-image',
    png: 'vscode-icons:file-type-image',
    gif: 'vscode-icons:file-type-image',
    webp: 'vscode-icons:file-type-image',
    svg: 'vscode-icons:file-type-image',
    bmp: 'vscode-icons:file-type-image',
    ico: 'vscode-icons:file-type-image',
    // Videos
    mp4: 'vscode-icons:file-type-video',
    avi: 'vscode-icons:file-type-video',
    mov: 'vscode-icons:file-type-video',
    mkv: 'vscode-icons:file-type-video',
    webm: 'vscode-icons:file-type-video',
    // Audio
    mp3: 'vscode-icons:file-type-audio',
    wav: 'vscode-icons:file-type-audio',
    flac: 'vscode-icons:file-type-audio',
    aac: 'vscode-icons:file-type-audio',
    ogg: 'vscode-icons:file-type-audio',
    // PDF
    pdf: 'vscode-icons:file-type-pdf2',
    // Archives
    zip: 'vscode-icons:file-type-zip',
    rar: 'vscode-icons:file-type-zip',
    '7z': 'vscode-icons:file-type-zip',
    tar: 'vscode-icons:file-type-zip',
    gz: 'vscode-icons:file-type-zip',
    bz2: 'vscode-icons:file-type-zip',
    // Office — Word
    doc: 'vscode-icons:file-type-word',
    docx: 'vscode-icons:file-type-word',
    // Office — Excel
    xls: 'vscode-icons:file-type-excel',
    xlsx: 'vscode-icons:file-type-excel',
    csv: 'vscode-icons:file-type-excel',
    // Office — PowerPoint
    ppt: 'vscode-icons:file-type-powerpoint',
    pptx: 'vscode-icons:file-type-powerpoint',
    // Code / text
    txt: 'vscode-icons:file-type-text',
    json: 'vscode-icons:file-type-json',
    xml: 'vscode-icons:file-type-xml',
    md: 'vscode-icons:file-type-markdown',
    yaml: 'vscode-icons:file-type-light-yaml',
    yml: 'vscode-icons:file-type-light-yaml',
    ts: 'logos:typescript-icon',
    js: 'logos:javascript',
    py: 'logos:python',
    java: 'logos:java',
  }

  if (ext && ext in extMap) return extMap[ext]

  // Fallback to MIME type
  if (mimeType.startsWith('image/')) return 'vscode-icons:file-type-image'
  if (mimeType.startsWith('video/')) return 'vscode-icons:file-type-video'
  if (mimeType.startsWith('audio/')) return 'vscode-icons:file-type-audio'
  if (mimeType.includes('pdf')) return 'vscode-icons:file-type-pdf2'
  if (
    mimeType.includes('zip') ||
    mimeType.includes('rar') ||
    mimeType.includes('tar') ||
    mimeType.includes('7z')
  )
    return 'vscode-icons:file-type-zip'
  if (mimeType.includes('word') || mimeType.includes('document'))
    return 'vscode-icons:file-type-word'
  if (mimeType.includes('excel') || mimeType.includes('spreadsheet'))
    return 'vscode-icons:file-type-excel'
  if (mimeType.includes('powerpoint') || mimeType.includes('presentation'))
    return 'vscode-icons:file-type-powerpoint'
  if (mimeType.includes('text/')) return 'vscode-icons:file-type-text'
  return 'vscode-icons:default-file'
}

export { getAttachmentIcon }
