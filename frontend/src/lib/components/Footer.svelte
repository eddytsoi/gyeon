<script lang="ts">
  import type { NavItem } from '$lib/types';
  import * as m from '$lib/paraglide/messages';

  let { navItems = [] }: { navItems?: NavItem[] } = $props();

  // CMS nav URLs are admin-authored, but we still reject anything that isn't
  // a same-origin relative path or an http(s) absolute URL. Blocks
  // javascript:, data:, vbscript:, file: smuggled through a compromised
  // admin account or stored-XSS into the nav table.
  function safeNavUrl(url: string): string {
    if (!url) return '#';
    if (url.startsWith('/') && !url.startsWith('//')) return url;
    if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('mailto:') || url.startsWith('tel:')) return url;
    return '#';
  }
</script>

<footer class="bg-navy-900 text-white/70 mt-24">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-14 md:py-20">
    <div class="grid grid-cols-2 md:grid-cols-4 gap-10 md:gap-8">

      <!-- Brand column -->
      <div class="col-span-2 md:col-span-1">
        <a href="/" class="font-display text-xl font-bold tracking-[0.18em] uppercase text-white">
          {m.footer_logo()}
        </a>
        <p class="mt-4 text-sm font-body leading-relaxed text-white/60">
          {m.footer_tagline()}
        </p>
      </div>

      <!-- Shop -->
      <div>
        <h3 class="text-[11px] uppercase tracking-[0.18em] font-display font-semibold text-white mb-4">
          {m.footer_section_shop()}
        </h3>
        <ul class="space-y-2.5 text-sm font-body">
          <li><a href="/products" class="hover:text-white transition-colors">{m.footer_link_all_products()}</a></li>
        </ul>
      </div>

      <!-- Support -->
      <div>
        <h3 class="text-[11px] uppercase tracking-[0.18em] font-display font-semibold text-white mb-4">
          {m.footer_section_support()}
        </h3>
        <ul class="space-y-2.5 text-sm font-body">
          <li><a href="/cart" class="hover:text-white transition-colors">{m.footer_link_my_cart()}</a></li>
        </ul>
      </div>

      <!-- Contact / about (CMS-driven nav fallback) -->
      {#if navItems.length > 0}
        <nav>
          <h3 class="text-[11px] uppercase tracking-[0.18em] font-display font-semibold text-white mb-4">
            About
          </h3>
          <ul class="space-y-2.5 text-sm font-body">
            {#each navItems as item}
              <li>
                <a href={safeNavUrl(item.url)} target={item.target ?? '_self'}
                   rel={item.target === '_blank' ? 'noopener noreferrer' : undefined}
                   class="hover:text-white transition-colors">
                  {item.label}
                </a>
              </li>
            {/each}
          </ul>
        </nav>
      {/if}
    </div>

    <!-- Below-footer bar -->
    <div class="mt-12 md:mt-16 pt-6 border-t border-white/10 flex flex-col sm:flex-row items-center justify-between gap-4 text-xs text-white/50">
      <p class="font-body">{m.footer_copyright({ year: new Date().getFullYear() })}</p>
      <p class="font-display uppercase tracking-[0.18em]">HK · ZH-HANT</p>
    </div>
  </div>
</footer>
