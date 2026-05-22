<script lang="ts">
  import type { SocialMediaEntry } from '$lib/types';
  import { SOCIAL_ICONS, CUSTOM_ICON_KEY, sanitizeSvgPath } from './social-icons';

  let {
    entry,
    class: className = 'h-5 w-5'
  }: { entry: SocialMediaEntry; class?: string } = $props();

  let pathData = $derived.by(() => {
    if (entry.icon === CUSTOM_ICON_KEY) return sanitizeSvgPath(entry.customSvgPath);
    return SOCIAL_ICONS[entry.icon]?.path ?? '';
  });
</script>

{#if pathData}
  <svg
    xmlns="http://www.w3.org/2000/svg"
    viewBox="0 0 24 24"
    fill="currentColor"
    class={className}
    aria-hidden="true"
    focusable="false"
  >
    <path d={pathData} />
  </svg>
{/if}
