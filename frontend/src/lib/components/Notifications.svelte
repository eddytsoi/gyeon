<script lang="ts">
  import { fly, fade } from 'svelte/transition';
  import { notify, type NotificationType } from '$lib/stores/notifications.svelte';

  const STYLES: Record<NotificationType, { bg: string; border: string; iconBg: string; iconColor: string }> = {
    success: { bg: 'bg-white', border: 'border-green-100',  iconBg: 'bg-green-50',  iconColor: 'text-green-600'  },
    error:   { bg: 'bg-white', border: 'border-red-100',    iconBg: 'bg-red-50',    iconColor: 'text-red-600'    },
    warning: { bg: 'bg-white', border: 'border-orange-100', iconBg: 'bg-orange-50', iconColor: 'text-orange-600' },
    info:    { bg: 'bg-white', border: 'border-blue-100',   iconBg: 'bg-blue-50',   iconColor: 'text-blue-600'   }
  };
</script>

<div class="fixed top-4 right-4 z-[100] flex flex-col gap-3 w-[22rem] max-w-[calc(100vw-2rem)] pointer-events-none">
  {#each notify.items as n (n.id)}
    {@const s = STYLES[n.type]}
    <div in:fly={{ x: 320, duration: 250 }}
         out:fade={{ duration: 200 }}
         class="pointer-events-auto {s.bg} {s.border} border rounded-2xl shadow-lg shadow-gray-900/5
                p-4 flex items-start gap-3">
      <div class="w-7 h-7 rounded-full {s.iconBg} {s.iconColor} flex items-center justify-center flex-shrink-0 mt-0.5">
        {#if n.type === 'success'}
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5"/>
          </svg>
        {:else if n.type === 'error'}
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        {:else if n.type === 'warning'}
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round"
                  d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874
                     1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"/>
          </svg>
        {:else}
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
            <path stroke-linecap="round" stroke-linejoin="round"
                  d="M11.25 11.25l.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21
                     12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z"/>
          </svg>
        {/if}
      </div>

      <div class="flex-1 min-w-0">
        <p class="text-sm font-semibold text-gray-900 leading-snug">{n.title}</p>
        {#if n.message}
          <p class="text-xs text-gray-500 mt-1 leading-relaxed break-words">{n.message}</p>
        {/if}
        {#if n.link}
          <a href={n.link}
             onclick={() => notify.dismiss(n.id)}
             class="inline-block mt-2 text-xs font-medium text-gray-900 underline underline-offset-2
                    hover:text-gray-700 transition-colors">
            View details →
          </a>
        {/if}
      </div>

      <button type="button"
              onclick={() => notify.dismiss(n.id)}
              aria-label="Close"
              class="text-gray-300 hover:text-gray-600 transition-colors flex-shrink-0 -mr-1 -mt-1 p-1">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>
  {/each}
</div>
