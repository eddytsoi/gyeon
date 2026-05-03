export const VIDEO_MIME = /^video\//;
export const VIDEO_EXTS = /\.(mp4|webm)(\?|#|$)/i;

export function isVideo(input: { mime_type?: string | null; url?: string | null }): boolean {
  if (input.mime_type && VIDEO_MIME.test(input.mime_type)) return true;
  if (input.url && VIDEO_EXTS.test(input.url)) return true;
  return false;
}
