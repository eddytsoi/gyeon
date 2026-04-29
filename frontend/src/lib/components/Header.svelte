<script lang="ts">
  import { cartStore } from '$lib/stores/cart.svelte';
  import type { NavItem, Customer } from '$lib/types';

  let {
    navItems = [],
    customer = null
  }: { navItems?: NavItem[]; customer?: Customer | null } = $props();

  let mobileOpen = $state(false);
  let accountOpen = $state(false);

  // Fallback hardcoded nav when DB has no items yet
  const fallbackLinks = [
    { label: 'Home', url: '/', target: '_self' },
    { label: 'Products', url: '/products', target: '_self' },
    { label: 'Blog', url: '/blog', target: '_self' },
  ];

  const links = $derived(
    navItems.length > 0
      ? navItems.map(i => ({ label: i.label, url: i.url, target: i.target }))
      : fallbackLinks
  );

  function closeAccount(e: MouseEvent) {
    const target = e.target as HTMLElement;
    if (!target.closest('[data-account-menu]')) accountOpen = false;
  }

  $effect(() => {
    if (accountOpen) {
      document.addEventListener('click', closeAccount);
      return () => document.removeEventListener('click', closeAccount);
    }
  });
</script>

<header class="sticky top-0 z-40 bg-white border-b border-gray-100 shadow-sm">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
    <div class="flex items-center justify-between h-16">

      <!-- Logo -->
      <a href="/" class="text-xl font-bold tracking-tight text-gray-900">
        Gyeon
      </a>

      <!-- Desktop nav -->
      <nav class="hidden md:flex items-center gap-8">
        {#each links as link}
          <a href={link.url} target={link.target}
             class="text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors">
            {link.label}
          </a>
        {/each}
      </nav>

      <!-- Account + Cart + mobile toggle -->
      <div class="flex items-center gap-1 sm:gap-2">

        <!-- Account -->
        {#if customer}
          <div class="relative" data-account-menu>
            <button
              type="button"
              onclick={(e) => { e.stopPropagation(); accountOpen = !accountOpen; }}
              class="p-2 text-gray-600 hover:text-gray-900 transition-colors flex items-center"
              aria-label="Account menu"
              aria-expanded={accountOpen}
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none"
                   viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round"
                  d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
              </svg>
            </button>

            {#if accountOpen}
              <div class="absolute right-0 mt-2 w-56 bg-white border border-gray-100 rounded-xl shadow-lg py-1 origin-top-right">
                <div class="px-4 py-3 border-b border-gray-100">
                  <p class="text-sm font-medium text-gray-900 truncate">{customer.first_name} {customer.last_name}</p>
                  <p class="text-xs text-gray-500 truncate">{customer.email}</p>
                </div>
                <a href="/account" onclick={() => accountOpen = false}
                   class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50">
                  <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M2.25 12l8.954-8.955c.44-.439 1.152-.439 1.591 0L21.75 12M4.5 9.75v10.125c0 .621.504 1.125 1.125 1.125H9.75v-4.875c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21h4.125c.621 0 1.125-.504 1.125-1.125V9.75M8.25 21h8.25" />
                  </svg>
                  Overview
                </a>
                <a href="/account/profile" onclick={() => accountOpen = false}
                   class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50">
                  <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
                  </svg>
                  Profile
                </a>
                <a href="/account/orders" onclick={() => accountOpen = false}
                   class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50">
                  <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25ZM6.75 12h.008v.008H6.75V12Zm0 3h.008v.008H6.75V15Zm0 3h.008v.008H6.75V18Z" />
                  </svg>
                  Orders
                </a>
                <a href="/account/addresses" onclick={() => accountOpen = false}
                   class="flex items-center gap-2 px-4 py-2 text-sm text-gray-700 hover:bg-gray-50">
                  <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM19.5 10.5c0 7.142-7.5 11.25-7.5 11.25S4.5 17.642 4.5 10.5a7.5 7.5 0 1 1 15 0Z" />
                  </svg>
                  Addresses
                </a>
                <div class="border-t border-gray-100 mt-1 pt-1">
                  <form method="POST" action="/account/logout">
                    <button type="submit"
                            class="flex items-center gap-2 w-full text-left px-4 py-2 text-sm text-red-500 hover:bg-red-50">
                      <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 9V5.25A2.25 2.25 0 0 1 10.5 3h6a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 16.5 21h-6a2.25 2.25 0 0 1-2.25-2.25V15m-3 0-3-3m0 0 3-3m-3 3H15" />
                      </svg>
                      Sign out
                    </button>
                  </form>
                </div>
              </div>
            {/if}
          </div>
        {:else}
          <a
            href="/account/login"
            class="p-2 text-gray-600 hover:text-gray-900 transition-colors"
            aria-label="Sign in"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none"
                 viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
            </svg>
          </a>
        {/if}

        <!-- Cart -->
        <a href="/cart" class="relative p-2 text-gray-600 hover:text-gray-900 transition-colors" aria-label="Cart">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none"
               viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M2.25 3h1.386c.51 0 .955.343 1.087.835l.383 1.437M7.5 14.25a3 3 0
                 0 0-3 3h15.75m-12.75-3h11.218c1.121-2.3 2.1-4.684 2.924-7.138a60.114
                 60.114 0 0 0-16.536-1.84M7.5 14.25 5.106 5.272M6 20.25a.75.75 0 1
                 1-1.5 0 .75.75 0 0 1 1.5 0Zm12.75 0a.75.75 0 1 1-1.5 0 .75.75 0 0
                 1 1.5 0Z" />
          </svg>
          {#if cartStore.itemCount > 0}
            <span class="absolute -top-0.5 -right-0.5 flex h-4 w-4 items-center
                         justify-center rounded-full bg-gray-900 text-[10px]
                         font-semibold text-white">
              {cartStore.itemCount}
            </span>
          {/if}
        </a>

        <!-- Mobile hamburger -->
        <button class="md:hidden p-2 text-gray-600" onclick={() => mobileOpen = !mobileOpen}
                aria-label="Toggle menu">
          <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none"
               viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            {#if mobileOpen}
              <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
            {:else}
              <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5M3.75 17.25h16.5"/>
            {/if}
          </svg>
        </button>
      </div>
    </div>
  </div>

  <!-- Mobile nav -->
  {#if mobileOpen}
    <nav class="md:hidden border-t border-gray-100 bg-white px-4 py-3 flex flex-col gap-1">
      {#each links as link}
        <a href={link.url} target={link.target}
           onclick={() => mobileOpen = false}
           class="py-2 text-sm font-medium text-gray-700 hover:text-gray-900">
          {link.label}
        </a>
      {/each}
    </nav>
  {/if}
</header>
