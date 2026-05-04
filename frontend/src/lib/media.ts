export const VIDEO_MIME = /^video\//;
export const VIDEO_EXTS = /\.(mp4|webm)(\?|#|$)/i;

export function isVideo(input: { mime_type?: string | null; url?: string | null }): boolean {
  if (input.mime_type && VIDEO_MIME.test(input.mime_type)) return true;
  if (input.url && VIDEO_EXTS.test(input.url)) return true;
  return false;
}

export const DEFAULT_IMAGE_MAX_MB = 1;
export const DEFAULT_VIDEO_MAX_MB = 10;

export type MediaUploadLimits = { imageMB: number; videoMB: number };

export type MediaSizeRejection = {
  kind: 'image' | 'video';
  limitMB: number;
  fileSizeMB: number;
  fileName: string;
};

export function extractMediaUploadLimits(
  settings: ReadonlyArray<{ key: string; value: string }> | null | undefined
): MediaUploadLimits {
  const pick = (key: string, fallback: number): number => {
    const raw = settings?.find((s) => s.key === key)?.value ?? '';
    const n = parseInt(raw, 10);
    return Number.isFinite(n) && n > 0 ? n : fallback;
  };
  return {
    imageMB: pick('upload_max_image_mb', DEFAULT_IMAGE_MAX_MB),
    videoMB: pick('upload_max_video_mb', DEFAULT_VIDEO_MAX_MB)
  };
}

export function checkMediaSize(
  file: File,
  limits: MediaUploadLimits
): MediaSizeRejection | null {
  const isVideoFile = (file.type ?? '').startsWith('video/');
  const limitMB = isVideoFile ? limits.videoMB : limits.imageMB;
  if (file.size <= limitMB * 1024 * 1024) return null;
  return {
    kind: isVideoFile ? 'video' : 'image',
    limitMB,
    fileSizeMB: Math.round((file.size / (1024 * 1024)) * 10) / 10,
    fileName: file.name
  };
}
