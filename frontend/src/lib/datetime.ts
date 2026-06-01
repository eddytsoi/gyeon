/** Order timestamp → "2026年6月1日 14:30" (Hong Kong time, 24-hour, no seconds). */
export function formatOrderDateTime(iso: string): string {
  const d = new Date(iso);
  const date = d.toLocaleDateString('zh-Hant', {
    timeZone: 'Asia/Hong_Kong',
    year: 'numeric',
    month: 'long',
    day: 'numeric'
  });
  const time = d.toLocaleTimeString('zh-Hant', {
    timeZone: 'Asia/Hong_Kong',
    hour12: false,
    hour: '2-digit',
    minute: '2-digit'
  });
  return `${date} ${time}`;
}
