export type MediaRecord = { mime_type?: string; url?: string; file_name?: string; original_file_name?: string }
export const isVideo = (media: MediaRecord) => ['video/mp4', 'video/webm'].includes(String(media?.mime_type || '').split(';')[0].trim().toLowerCase())
export const fileVideoMime = (file: File) => {
  const ext = file.name.split('.').pop()?.toLowerCase()
  return ext === 'mp4' ? 'video/mp4' : ext === 'webm' ? 'video/webm' : ''
}
const html = (value: string) => value.replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]!))
const markdown = (value: string) => value.replace(/[\\`*_{}\[\]()<>!]/g, '\\$&').replace(/[\r\n]/g, ' ')
export function mediaLink(media: MediaRecord, format: string, url = media.url || ''): string {
  const name = media.original_file_name || media.file_name || '文件'
  const video = isVideo(media)
  if (format === 'html' || format === 'gif') {
    if (video) return `<video src="${html(url)}" ${format === 'gif' ? 'autoplay loop muted' : 'controls'} playsinline></video>`
    return `<img src="${html(url)}" alt="${html(name)}">`
  }
  if (format === 'markdown') return `${video ? '' : '!'}[${markdown(name)}](<${url.replace(/[<>\s\\]/g, char => encodeURIComponent(char))}>)`
  if (format === 'bbcode') return video ? `[url=${url}]${name}[/url]` : `[img]${url}[/img]`
  return url
}
export async function copyMediaText(value: string) {
  try { await navigator.clipboard.writeText(value); return } catch { /* HTTP deployments */ }
  const input = document.createElement('textarea')
  input.value = value; input.style.position = 'fixed'; input.style.opacity = '0'
  document.body.append(input); input.select()
  try { if (!document.execCommand('copy')) throw new Error('复制失败，请手动复制') } finally { input.remove() }
}
