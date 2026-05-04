export const VIDEO_MIME = /^video\//;
export const VIDEO_EXTS = /\.(mp4|webm)(\?|#|$)/i;
export const IMAGE_EXTS = /\.(jpe?g|png|gif|webp|svg|avif|heic|bmp)(\?|#|$)/i;

export function isVideo(input: { mime_type?: string | null; url?: string | null }): boolean {
  if (input.mime_type && VIDEO_MIME.test(input.mime_type)) return true;
  if (input.url && VIDEO_EXTS.test(input.url)) return true;
  return false;
}

export function isImage(input: { mime_type?: string | null; url?: string | null }): boolean {
  if (input.mime_type && input.mime_type.startsWith('image/')) return true;
  if (input.mime_type === 'link' && input.url && IMAGE_EXTS.test(input.url)) return true;
  return false;
}

export function isLink(input: { mime_type?: string | null }): boolean {
  return input.mime_type === 'link';
}

// ── Streaming video providers ────────────────────────────────────────────────
// Mirrors backend/internal/media/streamvideo.go. Keep detection rules in sync.

export type StreamingProvider = 'youtube' | 'vimeo' | 'wistia';

export const STREAMING_MIMES: Record<string, StreamingProvider> = {
  'video/youtube': 'youtube',
  'video/vimeo': 'vimeo',
  'video/wistia': 'wistia'
};

export function getStreamingProvider(input: { mime_type?: string | null }): StreamingProvider | null {
  if (!input.mime_type) return null;
  return STREAMING_MIMES[input.mime_type] ?? null;
}

export function isStreamingVideo(input: { mime_type?: string | null }): boolean {
  return getStreamingProvider(input) !== null;
}

const YT_HOSTS = new Set(['youtube.com', 'www.youtube.com', 'm.youtube.com', 'youtu.be']);
const VIMEO_HOSTS = new Set(['vimeo.com', 'www.vimeo.com', 'player.vimeo.com']);
const YT_ID = /^[A-Za-z0-9_-]{6,20}$/;
const VIMEO_ID = /^[0-9]+$/;
const VIMEO_HASH = /^[A-Za-z0-9]+$/;
const WISTIA_ID = /^[A-Za-z0-9]{6,20}$/;

function isWistiaHost(host: string): boolean {
  return host.endsWith('.wistia.com') || host.endsWith('.wistia.net') || host === 'wistia.com' || host === 'wistia.net';
}

/** Pure URL parser — same rules as Go DetectStreamingVideo. */
export function detectStreamingVideoFromURL(
  raw: string
): { provider: StreamingProvider; videoID: string } | null {
  const trimmed = (raw ?? '').trim();
  if (!trimmed) return null;
  let u: URL;
  try {
    u = new URL(trimmed);
  } catch {
    return null;
  }
  const host = u.host.toLowerCase();
  const path = u.pathname.replace(/\/+$/, '');

  if (YT_HOSTS.has(host)) {
    if (host === 'youtu.be') {
      const id = path.replace(/^\//, '');
      return YT_ID.test(id) ? { provider: 'youtube', videoID: id } : null;
    }
    if (path === '/watch') {
      const id = u.searchParams.get('v') ?? '';
      return YT_ID.test(id) ? { provider: 'youtube', videoID: id } : null;
    }
    for (const prefix of ['/embed/', '/shorts/', '/v/']) {
      if (path.startsWith(prefix)) {
        const id = path.slice(prefix.length).split('/')[0];
        return YT_ID.test(id) ? { provider: 'youtube', videoID: id } : null;
      }
    }
    return null;
  }

  if (VIMEO_HOSTS.has(host)) {
    const segs = path.replace(/^\//, '').split('/').filter(Boolean);
    if (host === 'player.vimeo.com' && segs.length >= 2 && segs[0] === 'video' && VIMEO_ID.test(segs[1])) {
      const id = segs[1];
      if (segs.length >= 3 && VIMEO_HASH.test(segs[2])) return { provider: 'vimeo', videoID: `${id}/${segs[2]}` };
      const h = u.searchParams.get('h') ?? '';
      if (h && VIMEO_HASH.test(h)) return { provider: 'vimeo', videoID: `${id}/${h}` };
      return { provider: 'vimeo', videoID: id };
    }
    if (segs.length === 1 && VIMEO_ID.test(segs[0])) return { provider: 'vimeo', videoID: segs[0] };
    if (segs.length === 2 && VIMEO_ID.test(segs[0]) && VIMEO_HASH.test(segs[1]))
      return { provider: 'vimeo', videoID: `${segs[0]}/${segs[1]}` };
    if (segs.length === 3 && segs[0] === 'channels' && VIMEO_ID.test(segs[2]))
      return { provider: 'vimeo', videoID: segs[2] };
    if (segs.length === 4 && segs[0] === 'groups' && segs[2] === 'videos' && VIMEO_ID.test(segs[3]))
      return { provider: 'vimeo', videoID: segs[3] };
    return null;
  }

  if (isWistiaHost(host)) {
    if (path.startsWith('/channel/') || path.startsWith('/showcase/')) return null;
    const segs = path.replace(/^\//, '').split('/').filter(Boolean);
    if (segs.length >= 2 && segs[0] === 'medias' && WISTIA_ID.test(segs[1]))
      return { provider: 'wistia', videoID: segs[1] };
    if (segs.length >= 3 && segs[0] === 'embed' && segs[1] === 'iframe' && WISTIA_ID.test(segs[2]))
      return { provider: 'wistia', videoID: segs[2] };
    return null;
  }

  return null;
}

/** Returns the iframe src for a streaming video media row, or null if not streaming. */
export function getEmbedURL(input: { url?: string | null; mime_type?: string | null }): string | null {
  const provider = getStreamingProvider(input);
  if (!provider || !input.url) return null;
  const detected = detectStreamingVideoFromURL(input.url);
  if (!detected || detected.provider !== provider) return null;
  switch (provider) {
    case 'youtube':
      return `https://www.youtube.com/embed/${detected.videoID}`;
    case 'vimeo': {
      const [id, hash] = detected.videoID.split('/');
      return hash ? `https://player.vimeo.com/video/${id}?h=${hash}` : `https://player.vimeo.com/video/${id}`;
    }
    case 'wistia':
      return `https://fast.wistia.net/embed/iframe/${detected.videoID}`;
  }
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
