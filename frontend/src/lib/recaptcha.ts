// Browser helper for Google reCAPTCHA v3. Injects the script lazily on the
// first getRecaptchaToken() call so visitors who never interact with a form
// don't pay the network/JS cost. Returns null when no site key is configured
// — the contact-form submit handler treats null as "skip captcha" and lets
// the backend's `recaptcha_enabled = false` path take over.

let scriptPromise: Promise<void> | null = null;
let currentSiteKey: string | null = null;

declare global {
  interface Window {
    grecaptcha?: {
      ready: (cb: () => void) => void;
      execute: (siteKey: string, opts: { action: string }) => Promise<string>;
    };
  }
}

function loadScript(siteKey: string): Promise<void> {
  if (scriptPromise && currentSiteKey === siteKey) return scriptPromise;
  currentSiteKey = siteKey;
  scriptPromise = new Promise((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(
      `script[data-grecaptcha-key="${siteKey}"]`
    );
    if (existing) {
      if (window.grecaptcha) resolve();
      else existing.addEventListener('load', () => resolve(), { once: true });
      return;
    }
    const s = document.createElement('script');
    s.src = `https://www.google.com/recaptcha/api.js?render=${encodeURIComponent(siteKey)}`;
    s.async = true;
    s.defer = true;
    s.dataset.grecaptchaKey = siteKey;
    s.addEventListener('load', () => resolve(), { once: true });
    s.addEventListener('error', () => reject(new Error('failed to load grecaptcha')), { once: true });
    document.head.appendChild(s);
  });
  return scriptPromise;
}

export async function getRecaptchaToken(
  siteKey: string | null | undefined,
  action: string
): Promise<string> {
  if (!siteKey) return '';
  if (typeof window === 'undefined') return '';
  try {
    await loadScript(siteKey);
  } catch {
    return '';
  }
  if (!window.grecaptcha) return '';
  return new Promise<string>((resolve) => {
    window.grecaptcha!.ready(async () => {
      try {
        const token = await window.grecaptcha!.execute(siteKey, { action });
        resolve(token ?? '');
      } catch {
        resolve('');
      }
    });
  });
}
