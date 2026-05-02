<script lang="ts">
  let { class: cls = 'w-4 h-4' }: { class?: string } = $props();
</script>

<svg class="{cls} animate-spin" fill="none" viewBox="0 0 24 24" aria-hidden="true">
  <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" stroke-opacity="0.25" />
  <path d="M22 12a10 10 0 0 0-10-10" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
</svg>
