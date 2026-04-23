<script lang="ts">
  import type { LayoutData } from './$types';
  import { page } from '$app/stores';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const isPublicPage = $derived(
    $page.url.pathname === '/account/login' || $page.url.pathname === '/account/register'
  );

  const navLinks = [
    { href: '/account', label: 'Overview' },
    { href: '/account/profile', label: 'Profile' },
    { href: '/account/addresses', label: 'Addresses' },
    { href: '/account/orders', label: 'Orders' }
  ];
</script>

{#if isPublicPage}
  {@render children()}
{:else}
  <div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
    <div class="flex flex-col md:flex-row gap-8">

      <!-- Sidebar -->
      <aside class="md:w-52 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-4">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wider px-3 mb-3">
            My Account
          </p>
          <nav class="flex flex-col gap-1">
            {#each navLinks as link}
              <a
                href={link.href}
                class="px-3 py-2 rounded-lg text-sm font-medium transition-colors
                       {$page.url.pathname === link.href
                         ? 'bg-gray-900 text-white'
                         : 'text-gray-600 hover:bg-gray-50 hover:text-gray-900'}"
              >
                {link.label}
              </a>
            {/each}
          </nav>
          <div class="mt-4 pt-4 border-t border-gray-100">
            <form method="POST" action="/account/logout">
              <button
                type="submit"
                class="w-full px-3 py-2 rounded-lg text-sm font-medium text-left text-red-500 hover:bg-red-50 transition-colors"
              >
                Sign out
              </button>
            </form>
          </div>
        </div>
      </aside>

      <!-- Content -->
      <div class="flex-1 min-w-0">
        {@render children()}
      </div>
    </div>
  </div>
{/if}
