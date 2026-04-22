<script lang="ts">
  import '../../app.css';
  import { page } from '$app/stores';

  let { children } = $props();

  const isLoginPage = $derived($page.url.pathname === '/admin/login');
  let drawerOpen = $state(false);

  const navGroups = [
    {
      label: 'Main',
      links: [
        {
          href: '/admin/dashboard',
          label: 'Dashboard',
          icon: 'M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z'
        },
        {
          href: '/admin/products',
          label: 'Products',
          icon: 'M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z'
        },
        {
          href: '/admin/orders',
          label: 'Orders',
          icon: 'M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z'
        },
      ]
    },
    {
      label: 'CMS',
      links: [
        {
          href: '/admin/cms/pages',
          label: 'Pages',
          icon: 'M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z'
        },
        {
          href: '/admin/cms/posts',
          label: 'Posts',
          icon: 'M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 0 1-2.25 2.25M16.5 7.5V18a2.25 2.25 0 0 0 2.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 0 0 2.25 2.25h13.5M6 7.5h3v3H6v-3Z'
        },
        {
          href: '/admin/cms/post-categories',
          label: 'Categories',
          icon: 'M9.568 3H5.25A2.25 2.25 0 0 0 3 5.25v4.318c0 .597.237 1.17.659 1.591l9.581 9.581c.699.699 1.78.872 2.607.33a18.095 18.095 0 0 0 5.223-5.223c.542-.827.369-1.908-.33-2.607L11.16 3.66A2.25 2.25 0 0 0 9.568 3Z M6 6h.008v.008H6V6Z'
        },
        {
          href: '/admin/cms/navigation',
          label: 'Navigation',
          icon: 'M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12'
        },
      ]
    }
  ];

  function isActive(href: string) {
    return $page.url.pathname.startsWith(href);
  }
</script>

{#if isLoginPage}
  <div class="min-h-screen bg-gray-50">
    {@render children()}
  </div>
{:else}
  <!-- Mobile overlay -->
  {#if drawerOpen}
    <div class="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm md:hidden"
         onclick={() => drawerOpen = false}
         role="button" tabindex="-1" aria-label="Close menu"></div>
  {/if}

  <div class="flex h-screen bg-slate-50 overflow-hidden">

    <!-- ── Sidebar ── -->
    <aside class="
      fixed inset-y-0 left-0 z-50 flex flex-col w-64 bg-white border-r border-gray-100
      transition-transform duration-200 ease-in-out
      md:static md:translate-x-0 md:flex-shrink-0
      {drawerOpen ? 'translate-x-0 shadow-2xl' : '-translate-x-full'}
    ">
      <!-- Logo -->
      <div class="flex items-center gap-2.5 px-5 h-16 border-b border-gray-100 flex-shrink-0">
        <div class="w-8 h-8 rounded-lg bg-gray-900 flex items-center justify-center">
          <span class="text-white font-bold text-sm">G</span>
        </div>
        <div>
          <p class="font-bold text-gray-900 text-sm leading-none">Gyeon</p>
          <p class="text-[10px] text-gray-400 mt-0.5 leading-none">Admin Console</p>
        </div>
        <!-- Close btn (mobile only) -->
        <button onclick={() => drawerOpen = false}
                class="ml-auto p-1.5 rounded-lg text-gray-400 hover:text-gray-700
                       hover:bg-gray-100 transition-colors md:hidden">
          <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>

      <!-- Nav -->
      <nav class="flex-1 overflow-y-auto px-3 py-4 space-y-4">
        {#each navGroups as group}
          <div>
            <p class="px-3 mb-1.5 text-[10px] font-semibold text-gray-400 uppercase tracking-widest">
              {group.label}
            </p>
            <div class="space-y-0.5">
              {#each group.links as link}
                {@const active = isActive(link.href)}
                <a href={link.href}
                   onclick={() => drawerOpen = false}
                   class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm font-medium
                          transition-all duration-150 group
                          {active
                            ? 'bg-gray-900 text-white shadow-sm'
                            : 'text-gray-500 hover:text-gray-900 hover:bg-gray-50'}">
                  <svg class="w-4 h-4 flex-shrink-0 transition-colors
                              {active ? 'text-white' : 'text-gray-400 group-hover:text-gray-700'}"
                       fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d={link.icon} />
                  </svg>
                  {link.label}
                </a>
              {/each}
            </div>
          </div>
        {/each}
      </nav>

      <!-- Footer -->
      <div class="px-3 py-4 border-t border-gray-100 flex-shrink-0">
        <a href="/" target="_blank"
           class="flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm text-gray-400
                  hover:text-gray-700 hover:bg-gray-50 transition-colors group mb-1">
          <svg class="w-4 h-4 text-gray-300 group-hover:text-gray-500 transition-colors"
               fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M13.5 6H5.25A2.25 2.25 0 0 0 3 8.25v10.5A2.25 2.25 0 0 0 5.25 21h10.5A2.25
                 2.25 0 0 0 18 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
          </svg>
          View Store
        </a>
        <form method="POST" action="/admin/logout">
          <button type="submit"
                  class="w-full flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm
                         text-gray-400 hover:text-red-600 hover:bg-red-50 transition-colors group">
            <svg class="w-4 h-4 text-gray-300 group-hover:text-red-400 transition-colors"
                 fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25
                   2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75"/>
            </svg>
            Sign Out
          </button>
        </form>
      </div>
    </aside>

    <!-- ── Main area ── -->
    <div class="flex-1 flex flex-col min-w-0 overflow-hidden">

      <!-- Top bar -->
      <header class="h-16 bg-white border-b border-gray-100 flex items-center
                     gap-4 px-4 sm:px-6 flex-shrink-0">
        <!-- Hamburger (mobile) -->
        <button onclick={() => drawerOpen = true}
                class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100
                       transition-colors md:hidden -ml-1">
          <svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
                  d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"/>
          </svg>
        </button>

        <!-- Page title (derived from pathname) -->
        <h1 class="text-base font-semibold text-gray-900">
          {#if $page.url.pathname.includes('dashboard')}Dashboard
          {:else if $page.url.pathname.includes('products')}Products
          {:else if $page.url.pathname.includes('orders')}Orders
          {:else if $page.url.pathname.includes('cms/pages')}CMS · Pages
          {:else if $page.url.pathname.includes('cms/posts')}CMS · Posts
          {:else if $page.url.pathname.includes('cms/post-categories')}CMS · Categories
          {:else if $page.url.pathname.includes('cms/navigation')}CMS · Navigation
          {:else}Admin{/if}
        </h1>

        <div class="ml-auto flex items-center gap-2">
          <div class="w-8 h-8 rounded-full bg-gray-900 flex items-center justify-center">
            <span class="text-white text-xs font-semibold">A</span>
          </div>
        </div>
      </header>

      <!-- Content -->
      <main class="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8">
        {@render children()}
      </main>
    </div>
  </div>
{/if}
