<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';

  let { data, form }: { data: PageData; form: ActionData } = $props();
  let saving = $state(false);

  const TOGGLE_KEYS = new Set(['maintenance_mode']);

  const textSettings = $derived(data.settings.filter((s) => !TOGGLE_KEYS.has(s.key)));
  const maintenanceSetting = $derived(data.settings.find((s) => s.key === 'maintenance_mode'));
  let maintenanceOn = $state(maintenanceSetting?.value === 'true');
</script>

<svelte:head><title>Settings — Gyeon Admin</title></svelte:head>

<div class="max-w-2xl">
  <div class="flex items-center justify-between mb-8">
    <h1 class="text-2xl font-bold text-gray-900">Site Settings</h1>
  </div>

  {#if form?.success}
    <div class="bg-green-50 border border-green-100 text-green-700 text-sm rounded-xl px-4 py-3 mb-6">
      Settings saved successfully.
    </div>
  {/if}
  {#if form?.error}
    <div class="bg-red-50 border border-red-100 text-red-600 text-sm rounded-xl px-4 py-3 mb-6">
      {form.error}
    </div>
  {/if}

  <form method="POST" action="?/save"
        use:enhance={() => {
          saving = true;
          return async ({ update }) => { await update(); saving = false; };
        }}>

    <!-- Maintenance Mode -->
    {#if maintenanceSetting}
      <div class="bg-white rounded-2xl border border-gray-100 p-6 mb-4">
        <div class="flex items-center justify-between gap-4">
          <div>
            <p class="text-sm font-semibold text-gray-900">Maintenance Mode</p>
            {#if maintenanceSetting.description}
              <p class="text-xs text-gray-400 mt-0.5">{maintenanceSetting.description}</p>
            {/if}
          </div>
          <button type="button"
                  onclick={() => (maintenanceOn = !maintenanceOn)}
                  class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer rounded-full border-2 border-transparent
                         transition-colors duration-200 focus:outline-none focus:ring-2 focus:ring-gray-900 focus:ring-offset-2
                         {maintenanceOn ? 'bg-red-500' : 'bg-gray-200'}"
                  role="switch"
                  aria-checked={maintenanceOn}>
            <span class="pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform
                         transition duration-200 {maintenanceOn ? 'translate-x-5' : 'translate-x-0'}"></span>
          </button>
          <input type="hidden" name="maintenance_mode" value={maintenanceOn ? 'true' : 'false'} />
        </div>
        {#if maintenanceOn}
          <p class="mt-3 text-xs text-red-600 font-medium">
            ⚠ Site is in maintenance mode — non-admin visitors are redirected to the maintenance page.
          </p>
        {/if}
      </div>
    {/if}

    <!-- Other Settings -->
    <div class="bg-white rounded-2xl border border-gray-100 p-6">
      <div class="flex flex-col gap-5">
        {#each textSettings as setting}
          <div class="flex flex-col gap-1.5">
            <label for={setting.key}
                   class="text-xs font-semibold text-gray-500 uppercase tracking-wide">
              {setting.key.replace(/_/g, ' ')}
            </label>
            {#if setting.description}
              <p class="text-xs text-gray-400 -mt-0.5">{setting.description}</p>
            {/if}
            <input id={setting.key} name={setting.key} value={setting.value}
                   class="border border-gray-200 rounded-xl px-3 py-2.5 text-sm
                          focus:outline-none focus:ring-2 focus:ring-gray-900" />
          </div>
        {:else}
          <p class="text-sm text-gray-400">No settings found.</p>
        {/each}
      </div>

      <div class="mt-6 pt-5 border-t border-gray-100">
        <button type="submit" disabled={saving}
                class="px-5 py-2.5 bg-gray-900 text-white text-sm font-medium rounded-xl
                       hover:bg-gray-700 transition-colors disabled:opacity-50">
          {saving ? 'Saving…' : 'Save Settings'}
        </button>
      </div>
    </div>
  </form>
</div>
