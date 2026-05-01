<script lang="ts">
  // Inline SVG used as a prefix on every admin save / confirm button.
  // Heroicons-style check stroke path; sized via class so the icon scales with
  // text and inherits currentColor automatically.
  let { class: cls = 'w-4 h-4' }: { class?: string } = $props();
</script>

<svg class={cls} fill="none" viewBox="0 0 24 24"
     stroke="currentColor" stroke-width="2" aria-hidden="true">
  <path stroke-linecap="round" stroke-linejoin="round" d="m4.5 12.75 6 6 9-13.5" />
</svg>
