<script lang="ts">
  import type { NavItem } from '$lib/types';
  import * as m from '$lib/paraglide/messages';

  let { navItems = [] }: { navItems?: NavItem[] } = $props();
</script>

<footer class="bg-gray-900 text-gray-400 mt-24">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
    <div class="grid grid-cols-2 md:grid-cols-4 gap-8">
      <div class="col-span-2 md:col-span-1">
        <span class="text-white font-bold text-lg">{m.footer_logo()}</span>
        <p class="mt-2 text-sm leading-relaxed">
          {m.footer_tagline()}
        </p>
      </div>
      <div>
        <h3 class="text-white text-sm font-semibold mb-3">{m.footer_section_shop()}</h3>
        <ul class="space-y-2 text-sm">
          <li><a href="/products" class="hover:text-white transition-colors">{m.footer_link_all_products()}</a></li>
        </ul>
      </div>
      <div>
        <h3 class="text-white text-sm font-semibold mb-3">{m.footer_section_support()}</h3>
        <ul class="space-y-2 text-sm">
          <li><a href="/cart" class="hover:text-white transition-colors">{m.footer_link_my_cart()}</a></li>
        </ul>
      </div>
    </div>

    {#if navItems.length > 0}
      <nav class="mt-10 border-t border-gray-800 pt-6">
        <ul class="flex flex-wrap items-center justify-center gap-x-6 gap-y-2 text-xs">
          {#each navItems as item}
            <li>
              <a href={item.url} target={item.target ?? '_self'}
                 rel={item.target === '_blank' ? 'noopener' : undefined}
                 class="hover:text-white transition-colors">
                {item.label}
              </a>
            </li>
          {/each}
        </ul>
      </nav>
    {/if}

    <div class="{navItems.length > 0 ? 'mt-6' : 'mt-10 border-t border-gray-800 pt-6'} text-xs text-center">
      {m.footer_copyright({ year: new Date().getFullYear() })}
    </div>
  </div>
</footer>
