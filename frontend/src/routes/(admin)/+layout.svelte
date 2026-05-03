<script lang="ts">
  import '../../app.css';
  import { onMount } from 'svelte';
  import { page, navigating } from '$app/stores';
  import { slide } from 'svelte/transition';
  import { cubicOut } from 'svelte/easing';
  import Notifications from '$lib/components/Notifications.svelte';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';

  let { data, children } = $props();

  const isLoginPage = $derived($page.url.pathname === '/admin/login');
  let drawerOpen = $state(false);

  onMount(() => {
    if (!data.token || isLoginPage) return;
    const es = new EventSource(`/api/v1/admin/events?token=${encodeURIComponent(data.token)}`);
    es.addEventListener('new_order', (e) => {
      try {
        const o = JSON.parse((e as MessageEvent).data) as {
          order_id: string;
          order_number: string;
          customer_name: string;
          total: number;
        };
        const total = `HK$${o.total.toFixed(2)}`;
        const who = o.customer_name?.trim() || m.admin_event_guest_label();
        notify.info(
          m.admin_event_new_order_title(),
          m.admin_event_new_order_body({ orderNumber: o.order_number, customer: who, total }),
          0,
          `/admin/orders/${o.order_id}`
        );
      } catch {
        /* ignore malformed events */
      }
    });
    return () => es.close();
  });

  type NavChild = { href: string; label: string };
  type NavLink = { href: string; label: string; icon: string; children?: NavChild[] };
  type NavGroup = { label: string; links: NavLink[] };

  const navGroups: NavGroup[] = $derived([
    {
      label: m.admin_nav_section_main(),
      links: [
        {
          href: '/admin/dashboard',
          label: m.admin_nav_dashboard(),
          icon: 'M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 0 1 3 19.875v-6.75ZM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V8.625ZM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 0 1-1.125-1.125V4.125Z'
        },
        {
          href: '/admin/products',
          label: m.admin_nav_products(),
          icon: 'M20.25 7.5l-.625 10.632a2.25 2.25 0 0 1-2.247 2.118H6.622a2.25 2.25 0 0 1-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125Z',
          children: [
            { href: '/admin/products/categories', label: m.admin_nav_products_categories() },
          ]
        },
        {
          href: '/admin/orders',
          label: m.admin_nav_orders(),
          icon: 'M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0ZM3.75 12h.007v.008H3.75V12Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Zm-.375 5.25h.007v.008H3.75v-.008Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z'
        },
      ]
    },
    {
      label: m.admin_nav_section_cms(),
      links: [
        {
          href: '/admin/cms/pages',
          label: m.admin_nav_pages(),
          icon: 'M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z'
        },
        {
          href: '/admin/cms/posts',
          label: m.admin_nav_posts(),
          icon: 'M12 7.5h1.5m-1.5 3h1.5m-7.5 3h7.5m-7.5 3h7.5m3-9h3.375c.621 0 1.125.504 1.125 1.125V18a2.25 2.25 0 0 1-2.25 2.25M16.5 7.5V18a2.25 2.25 0 0 0 2.25 2.25M16.5 7.5V4.875c0-.621-.504-1.125-1.125-1.125H4.125C3.504 3.75 3 4.254 3 4.875V18a2.25 2.25 0 0 0 2.25 2.25h13.5M6 7.5h3v3H6v-3Z',
          children: [
            { href: '/admin/cms/posts/categories', label: m.admin_nav_posts_categories() },
          ]
        },
        {
          href: '/admin/cms/navigation',
          label: m.admin_nav_navigation(),
          icon: 'M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25H12'
        },
        {
          href: '/admin/media',
          label: m.admin_nav_media(),
          icon: 'M2.25 15.75l5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z'
        },
      ]
    },
    {
      label: m.admin_nav_section_store(),
      links: [
        {
          href: '/admin/customers',
          label: m.admin_nav_customers(),
          icon: 'M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z'
        },
        {
          href: '/admin/settings',
          label: m.admin_nav_settings(),
          icon: 'M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z'
        },
        {
          href: '/admin/users',
          label: m.admin_nav_users(),
          icon: 'M17.982 18.725A7.488 7.488 0 0 0 12 15.75a7.488 7.488 0 0 0-5.982 2.975m11.963 0a9 9 0 1 0-11.963 0m11.963 0A8.966 8.966 0 0 1 12 21a8.966 8.966 0 0 1-5.982-2.275M15 9.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z'
        },
        {
          href: '/admin/import',
          label: m.admin_nav_import(),
          icon: 'M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5'
        },
      ]
    }
  ]);

  function isActive(href: string) {
    return $page.url.pathname.startsWith(href);
  }

  function isChildActive(href: string, siblings: NavChild[]) {
    const pathname = $page.url.pathname;
    const moreSpecific = siblings.filter(s => s.href !== href && s.href.startsWith(href));
    if (moreSpecific.some(s => pathname.startsWith(s.href))) return false;
    return pathname.startsWith(href);
  }

  function isParentActive(link: NavLink) {
    const pathname = $page.url.pathname;
    if (!pathname.startsWith(link.href)) return false;
    return !(link.children ?? []).some(c => pathname.startsWith(c.href));
  }

  function isGroupExpanded(link: NavLink) {
    const pathname = $page.url.pathname;
    return pathname.startsWith(link.href);
  }

  function isNavigatingTo(href: string) {
    const target = $navigating?.to?.url.pathname;
    return !!target && target !== $page.url.pathname && target.startsWith(href);
  }

  // ── Sidebar magnetic spotlight ──────────────────────────────────
  let navEl = $state<HTMLElement | undefined>();
  let spotlight = $state({ visible: false, top: 0, left: 0, width: 0, height: 0 });
  let footerEl = $state<HTMLElement | undefined>();
  let footerSpotlight = $state({ visible: false, top: 0, left: 0, width: 0, height: 0, danger: false });

  function moveSpotlightTo(item: Element | null) {
    if (!item || !navEl || !navEl.contains(item)) {
      spotlight.visible = false;
      return;
    }
    const navRect = navEl.getBoundingClientRect();
    const itemRect = item.getBoundingClientRect();
    spotlight = {
      visible: true,
      top: itemRect.top - navRect.top + navEl.scrollTop,
      left: itemRect.left - navRect.left + navEl.scrollLeft,
      width: itemRect.width,
      height: itemRect.height
    };
  }

  function onNavMouseMove(e: MouseEvent) {
    moveSpotlightTo((e.target as HTMLElement | null)?.closest('.js-nav-item') ?? null);
  }

  function onNavMouseLeave() {
    spotlight.visible = false;
  }

  function moveFooterSpotlightTo(item: Element | null) {
    if (!item || !footerEl || !footerEl.contains(item)) {
      footerSpotlight.visible = false;
      return;
    }
    const containerRect = footerEl.getBoundingClientRect();
    const itemRect = item.getBoundingClientRect();
    footerSpotlight = {
      visible: true,
      top: itemRect.top - containerRect.top,
      left: itemRect.left - containerRect.left,
      width: itemRect.width,
      height: itemRect.height,
      danger: (item as HTMLElement).classList.contains('js-footer-item--danger')
    };
  }

  function onFooterMouseMove(e: MouseEvent) {
    moveFooterSpotlightTo((e.target as HTMLElement | null)?.closest('.js-footer-item') ?? null);
  }

  function onFooterMouseLeave() {
    footerSpotlight.visible = false;
  }
</script>

<Notifications />

{#if isLoginPage}
  <div class="min-h-screen bg-gray-50">
    {@render children()}
  </div>
{:else}
  <!-- Mobile overlay -->
  {#if drawerOpen}
    <div class="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm md:hidden"
         onclick={() => drawerOpen = false}
         role="button" tabindex="-1" aria-label={m.admin_aria_close_menu()}></div>
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
          <p class="text-[10px] text-gray-400 mt-0.5 leading-none">{m.admin_console()}</p>
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
      <nav bind:this={navEl}
           onmousemove={onNavMouseMove}
           onmouseleave={onNavMouseLeave}
           class="relative flex-1 overflow-y-auto px-3 py-4 space-y-4">
        <!-- Magnetic spotlight: glides under the cursor and snaps to the hovered item -->
        <div aria-hidden="true"
             class="pointer-events-none absolute z-0 rounded-lg bg-gray-100
                    transition-[transform,width,height,opacity] duration-[80ms] ease-out
                    {spotlight.visible ? 'opacity-100' : 'opacity-0'}"
             style="top: 0; left: 0; transform: translate3d({spotlight.left}px, {spotlight.top}px, 0); width: {spotlight.width}px; height: {spotlight.height}px;">
        </div>

        {#each navGroups as group}
          <div>
            <p class="px-3 mb-1.5 text-[10px] font-semibold text-gray-400 uppercase tracking-widest">
              {group.label}
            </p>
            <div class="space-y-0.5">
              {#each group.links as link}
                {#if link.children}
                  {@const parentActive = isParentActive(link)}
                  {@const expanded = isGroupExpanded(link)}
                  {@const loadingParent = isNavigatingTo(link.href) && !expanded}
                  <div>
                    <a href={link.href}
                       onclick={() => drawerOpen = false}
                       class="js-nav-item relative z-10 flex items-center gap-3 pl-4 pr-3 py-2 rounded-lg text-sm
                              transition-colors duration-150 group
                              {parentActive
                                ? 'text-gray-900 font-semibold'
                                : 'text-gray-500 hover:text-gray-900 font-medium'}">
                      {#if parentActive}
                        <span class="absolute left-0 top-1.5 bottom-1.5 w-0.5 bg-gray-900 rounded-r-full"></span>
                      {/if}
                      <svg class="w-4 h-4 flex-shrink-0 transition-colors
                                  {parentActive ? 'text-gray-900' : 'text-gray-400 group-hover:text-gray-700'}"
                           fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                        <path stroke-linecap="round" stroke-linejoin="round" d={link.icon} />
                      </svg>
                      <span class="flex-1">{link.label}</span>
                      {#if loadingParent}
                        <span class="w-1.5 h-1.5 rounded-full bg-current opacity-50 animate-pulse"></span>
                      {/if}
                    </a>
                    {#if expanded}
                      <div transition:slide={{ duration: 180, easing: cubicOut }}
                           class="overflow-hidden">
                        <div class="ml-4 mt-0.5 pl-3 border-l border-gray-100 space-y-0.5">
                          {#each link.children as child}
                            {@const childActive = isChildActive(child.href, link.children)}
                            {@const loadingChild = isNavigatingTo(child.href)}
                            <a href={child.href}
                               onclick={() => drawerOpen = false}
                               class="js-nav-item relative z-10 flex items-center gap-2.5 pl-3 pr-3 py-1.5 rounded-lg text-sm
                                      transition-colors duration-150
                                      {childActive
                                        ? 'text-gray-900 font-semibold'
                                        : 'text-gray-500 hover:text-gray-900 font-medium'}">
                              {#if childActive}
                                <span class="absolute left-0 top-1.5 bottom-1.5 w-0.5 bg-gray-900 rounded-r-full"></span>
                              {/if}
                              <span class="w-1 h-1 rounded-full flex-shrink-0 transition-colors
                                           {childActive ? 'bg-gray-900' : 'bg-gray-300'}"></span>
                              <span class="flex-1">{child.label}</span>
                              {#if loadingChild}
                                <span class="w-1.5 h-1.5 rounded-full bg-current opacity-50 animate-pulse"></span>
                              {/if}
                            </a>
                          {/each}
                        </div>
                      </div>
                    {/if}
                  </div>
                {:else}
                  {@const active = isActive(link.href)}
                  {@const loading = isNavigatingTo(link.href)}
                  <a href={link.href}
                     onclick={() => drawerOpen = false}
                     class="js-nav-item relative z-10 flex items-center gap-3 pl-4 pr-3 py-2 rounded-lg text-sm
                            transition-colors duration-150 group
                            {active
                              ? 'text-gray-900 font-semibold'
                              : 'text-gray-500 hover:text-gray-900 font-medium'}">
                    {#if active}
                      <span class="absolute left-0 top-1.5 bottom-1.5 w-0.5 bg-gray-900 rounded-r-full"></span>
                    {/if}
                    <svg class="w-4 h-4 flex-shrink-0 transition-colors
                                {active ? 'text-gray-900' : 'text-gray-400 group-hover:text-gray-700'}"
                         fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                      <path stroke-linecap="round" stroke-linejoin="round" d={link.icon} />
                    </svg>
                    <span class="flex-1">{link.label}</span>
                    {#if loading}
                      <span class="w-1.5 h-1.5 rounded-full bg-current opacity-50 animate-pulse"></span>
                    {/if}
                  </a>
                {/if}
              {/each}
            </div>
          </div>
        {/each}
      </nav>

      <!-- Footer -->
      <div bind:this={footerEl}
           onmousemove={onFooterMouseMove}
           onmouseleave={onFooterMouseLeave}
           class="relative px-3 py-4 border-t border-gray-100 flex-shrink-0">
        <!-- Footer magnetic spotlight -->
        <div aria-hidden="true"
             class="pointer-events-none absolute z-0 rounded-xl
                    transition-[transform,width,height,opacity,background-color] duration-[80ms] ease-out
                    {footerSpotlight.danger ? 'bg-red-50' : 'bg-gray-100'}
                    {footerSpotlight.visible ? 'opacity-100' : 'opacity-0'}"
             style="top: 0; left: 0; transform: translate3d({footerSpotlight.left}px, {footerSpotlight.top}px, 0); width: {footerSpotlight.width}px; height: {footerSpotlight.height}px;">
        </div>
        <a href="/" target="_blank"
           class="js-footer-item relative z-10 flex items-center gap-3 px-3 py-2.5 rounded-xl text-sm
                  text-gray-400 hover:text-gray-700 transition-colors group mb-1">
          <svg class="w-4 h-4 text-gray-300 group-hover:text-gray-500 transition-colors"
               fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
            <path stroke-linecap="round" stroke-linejoin="round"
              d="M13.5 6H5.25A2.25 2.25 0 0 0 3 8.25v10.5A2.25 2.25 0 0 0 5.25 21h10.5A2.25
                 2.25 0 0 0 18 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
          </svg>
          {m.admin_footer_view_store()}
        </a>
        <form method="POST" action="/admin/logout">
          <button type="submit"
                  class="js-footer-item js-footer-item--danger relative z-10 w-full flex items-center gap-3 px-3 py-2.5 rounded-xl
                         text-sm text-gray-400 hover:text-red-600 transition-colors group">
            <svg class="w-4 h-4 text-gray-300 group-hover:text-red-400 transition-colors"
                 fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
              <path stroke-linecap="round" stroke-linejoin="round"
                d="M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25
                   2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15M12 9l-3 3m0 0 3 3m-3-3h12.75"/>
            </svg>
            {m.admin_footer_sign_out()}
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
          {#if $page.url.pathname.includes('dashboard')}{m.admin_top_dashboard()}
          {:else if $page.url.pathname.includes('products/categories')}{m.admin_top_products_categories()}
          {:else if $page.url.pathname.includes('products')}{m.admin_top_products()}
          {:else if $page.url.pathname.includes('orders')}{m.admin_top_orders()}
          {:else if $page.url.pathname.includes('customers')}{m.admin_top_customers()}
          {:else if $page.url.pathname.includes('settings')}{m.admin_top_settings()}
          {:else if $page.url.pathname.includes('users')}{m.admin_top_users()}
          {:else if $page.url.pathname.includes('cms/pages')}{m.admin_top_cms_pages()}
          {:else if $page.url.pathname.includes('cms/posts/categories')}{m.admin_top_cms_posts_categories()}
          {:else if $page.url.pathname.includes('cms/posts')}{m.admin_top_cms_posts()}
          {:else if $page.url.pathname.includes('cms/navigation')}{m.admin_top_cms_navigation()}
          {:else if $page.url.pathname.includes('media')}{m.admin_top_media()}
          {:else}{m.admin_top_admin()}{/if}
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
