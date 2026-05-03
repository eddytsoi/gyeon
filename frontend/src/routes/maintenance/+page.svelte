<script lang="ts">
  import '../../app.css';
  import * as m from '$lib/paraglide/messages';
</script>

<svelte:head><title>{m.maintenance_title()}</title></svelte:head>

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
  <div class="max-w-md w-full text-center">
    <div class="mb-6 text-5xl select-none">🔧</div>
    <h1 class="text-3xl font-bold text-gray-900 mb-3">{m.maintenance_heading()}</h1>
    <p class="text-gray-500 text-base leading-relaxed">
      {m.maintenance_body()}<br />
      {m.maintenance_thanks()}
    </p>
  </div>
</div>
