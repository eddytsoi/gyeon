<script lang="ts">
  import type { LayoutData } from './$types';
  import { page } from '$app/stores';
  import * as m from '$lib/paraglide/messages';

  let { children, data }: { children: any; data: LayoutData } = $props();

  const isPublicPage = $derived(
    $page.url.pathname === '/account/login' ||
    $page.url.pathname === '/account/register' ||
    $page.url.pathname === '/account/setup-password' ||
    $page.url.pathname === '/account/reset-password'
  );

  // 訂單 leads the nav — /account lands on orders, so it's first in both the
  // Classic sidebar and the Modern tab bar.
  const navLinks = $derived([
    { href: '/account/orders', label: m.account_nav_orders(), icon: 'M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15c1.012 0 1.867.668 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25ZM6.75 12h.008v.008H6.75V12Zm0 3h.008v.008H6.75V15Zm0 3h.008v.008H6.75V18Z' },
    { href: '/account/profile', label: m.account_nav_profile(), icon: 'M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z' },
    { href: '/account/addresses', label: m.account_nav_addresses(), icon: 'M15 10.5a3 3 0 1 1-6 0 3 3 0 0 1 6 0ZM19.5 10.5c0 7.142-7.5 11.25-7.5 11.25S4.5 17.642 4.5 10.5a7.5 7.5 0 1 1 15 0Z' }
  ]);

  // ── Sidebar magnetic spotlight ──────────────────────────────────
  let navEl = $state<HTMLElement | undefined>();
  let spotlight = $state({ visible: false, top: 0, left: 0, width: 0, height: 0, danger: false });

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
      height: itemRect.height,
      danger: (item as HTMLElement).classList.contains('js-nav-item--danger')
    };
  }

  function onNavMouseMove(e: MouseEvent) {
    moveSpotlightTo((e.target as HTMLElement | null)?.closest('.js-nav-item') ?? null);
  }

  function onNavMouseLeave() {
    spotlight.visible = false;
  }
</script>

{#if isPublicPage}
  {@render children()}
{:else if data.accountPageLayout === 'modern'}
  <!-- Modern shell — single column with a top tab bar (Concept A). -->
  <div class="max-w-4xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
    <!-- Heading row — 登出 sits here (not in the scrollable tab bar) so it stays
         visible on mobile where the tabs can scroll off-screen. -->
    <div class="flex items-center justify-between gap-4 mb-5">
      <h1 class="text-2xl font-bold text-navy-900">{m.account_my_account_label()}</h1>
      <form method="POST" action="/account/logout" class="shrink-0">
        <button
          type="submit"
          class="inline-flex items-center gap-2 text-sm font-medium text-ink-500 transition-colors hover:text-alert"
        >
          <svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 9V5.25A2.25 2.25 0 0 1 10.5 3h6a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 16.5 21h-6a2.25 2.25 0 0 1-2.25-2.25V15m-3 0-3-3m0 0 3-3m-3 3H15" />
          </svg>
          {m.account_nav_sign_out()}
        </button>
      </form>
    </div>

    <!-- Top tab bar — horizontal-scrolls on small screens so content stays at the top. -->
    <nav class="flex items-stretch gap-1 mb-8 border-b border-gray-200 overflow-x-auto
                [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
      {#each navLinks as link}
        {@const active = $page.url.pathname.startsWith(link.href)}
        <a
          href={link.href}
          aria-current={active ? 'page' : undefined}
          class="inline-flex items-center gap-2 shrink-0 -mb-px border-b-2 px-4 py-3 text-sm font-medium transition-colors
                 {active
                   ? 'border-navy-500 text-navy-900'
                   : 'border-transparent text-ink-500 hover:text-navy-900'}"
        >
          <svg class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
            <path stroke-linecap="round" stroke-linejoin="round" d={link.icon} />
          </svg>
          {link.label}
        </a>
      {/each}
    </nav>

    <!-- Content -->
    <div class="min-w-0">
      {@render children()}
    </div>
  </div>
{:else}
  <!-- Classic shell (default) — sidebar + content. -->
  <div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
    <div class="flex flex-col md:flex-row gap-8">

      <!-- Sidebar -->
      <aside class="md:w-52 flex-shrink-0">
        <div class="bg-white rounded-2xl border border-gray-100 p-4">
          <p class="text-xs font-semibold text-gray-400 uppercase tracking-wider px-3 mb-3">
            {m.account_my_account_label()}
          </p>
          <div bind:this={navEl}
               onmousemove={onNavMouseMove}
               onmouseleave={onNavMouseLeave}
               class="relative">
            <!-- Magnetic spotlight: glides under the cursor and snaps to the hovered item -->
            <div aria-hidden="true"
                 class="pointer-events-none absolute z-0 rounded-lg
                        transition-[transform,width,height,opacity,background-color] duration-[80ms] ease-out
                        {spotlight.danger ? 'bg-red-50' : 'bg-gray-100'}
                        {spotlight.visible ? 'opacity-100' : 'opacity-0'}"
                 style="top: 0; left: 0; transform: translate3d({spotlight.left}px, {spotlight.top}px, 0); width: {spotlight.width}px; height: {spotlight.height}px;">
            </div>

            <nav class="relative z-10 flex flex-col gap-1">
              {#each navLinks as link}
                <a
                  href={link.href}
                  class="js-nav-item relative z-10 flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium transition-colors
                         {$page.url.pathname === link.href
                           ? 'bg-gray-900 text-white'
                           : 'text-gray-600 hover:text-gray-900'}"
                >
                  <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d={link.icon} />
                  </svg>
                  {link.label}
                </a>
              {/each}
            </nav>
            <div class="relative z-10 mt-4 pt-4 border-t border-gray-100">
              <form method="POST" action="/account/logout">
                <button
                  type="submit"
                  class="js-nav-item js-nav-item--danger relative z-10 flex items-center gap-2.5 w-full px-3 py-2 rounded-lg text-sm font-medium text-left text-red-500 transition-colors"
                >
                  <svg class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M8.25 9V5.25A2.25 2.25 0 0 1 10.5 3h6a2.25 2.25 0 0 1 2.25 2.25v13.5A2.25 2.25 0 0 1 16.5 21h-6a2.25 2.25 0 0 1-2.25-2.25V15m-3 0-3-3m0 0 3-3m-3 3H15" />
                  </svg>
                  {m.account_nav_sign_out()}
                </button>
              </form>
            </div>
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
