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

/**
 * Returns the iframe src for a streaming video media row, or null if not
 * streaming. When `autoplay` is true, appends provider-specific params for
 * autoplay + muted + loop AND hides every player chrome element (controls,
 * branding, title, related videos, fullscreen button, keyboard shortcuts).
 * The autoplay flag falls back to the row's `video_autoplay` when not passed
 * explicitly.
 */
export function getEmbedURL(
  input: { url?: string | null; mime_type?: string | null; video_autoplay?: boolean | null },
  opts?: { autoplay?: boolean }
): string | null {
  const provider = getStreamingProvider(input);
  if (!provider || !input.url) return null;
  const detected = detectStreamingVideoFromURL(input.url);
  if (!detected || detected.provider !== provider) return null;
  const autoplay = opts?.autoplay ?? input.video_autoplay ?? false;
  switch (provider) {
    case 'youtube': {
      const base = `https://www.youtube.com/embed/${detected.videoID}`;
      if (!autoplay) return base;
      // Loop on YouTube requires playlist={ID} pointing at the same video.
      // controls=0 hides the player bar; modestbranding=1 + rel=0 + iv_load_policy=3
      // strip the YouTube logo, related-videos overlay, and annotations;
      // disablekb=1 + fs=0 disable keyboard shortcuts and the fullscreen button.
      const params = [
        'autoplay=1', 'mute=1', 'loop=1', `playlist=${detected.videoID}`,
        'playsinline=1', 'controls=0', 'modestbranding=1', 'rel=0',
        'iv_load_policy=3', 'disablekb=1', 'fs=0'
      ].join('&');
      return `${base}?${params}`;
    }
    case 'vimeo': {
      const [id, hash] = detected.videoID.split('/');
      const base = hash ? `https://player.vimeo.com/video/${id}?h=${hash}` : `https://player.vimeo.com/video/${id}`;
      if (!autoplay) return base;
      // Vimeo's `background=1` is a single switch that bundles autoplay +
      // muted + loop + no-controls + no-title + no-byline + no-portrait —
      // exactly the chromeless preview we want.
      return `${base}${base.includes('?') ? '&' : '?'}background=1`;
    }
    case 'wistia': {
      const base = `https://fast.wistia.net/embed/iframe/${detected.videoID}`;
      if (!autoplay) return base;
      const params = [
        'autoPlay=true', 'muted=true', 'endVideoBehavior=loop', 'playsinline=true',
        'controlsVisibleOnLoad=false', 'playbar=false', 'playButton=false',
        'smallPlayButton=false', 'fullscreenButton=false', 'volumeControl=false',
        'settingsControl=false', 'playbackRateControl=false', 'qualityControl=false'
      ].join('&');
      return `${base}?${params}`;
    }
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
