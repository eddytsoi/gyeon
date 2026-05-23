<script lang="ts">
  import type { NavItem, SocialMediaEntry } from '$lib/types';
  import * as m from '$lib/paraglide/messages';
  import SocialIcon from './SocialIcon.svelte';
  import { SOCIAL_ICONS, CUSTOM_ICON_KEY } from './social-icons';

  let {
    navItems = [],
    socials = [],
    companyLogoFooterUrl = '',
    companyLogoFooterHeight = 40,
    slogan = ''
  }: {
    navItems?: NavItem[];
    socials?: SocialMediaEntry[];
    companyLogoFooterUrl?: string;
    companyLogoFooterHeight?: number;
    slogan?: string;
  } = $props();

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

  function socialLabel(entry: SocialMediaEntry): string {
    if (entry.label) return entry.label;
    if (entry.icon === CUSTOM_ICON_KEY) return m.footer_social_aria_label();
    return SOCIAL_ICONS[entry.icon]?.label ?? entry.icon;
  }

  let visibleSocials = $derived(
    socials.filter((s) => s.url && safeNavUrl(s.url) !== '#')
  );
</script>

<footer class="bg-navy-900 text-white/70 mt-24">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-14 md:py-20">
    <div class="flex flex-col md:flex-row md:items-start md:justify-between gap-10 md:gap-8">

      <!-- Left: brand + footer nav -->
      <div class="flex-1 min-w-0">
        <a href="/" class="flex items-center" aria-label={m.footer_logo()}>
          {#if companyLogoFooterUrl}
            <img src={companyLogoFooterUrl} alt={m.footer_logo()}
                 style="height: {companyLogoFooterHeight}px; width: auto;"
                 class="object-contain" />
          {:else}
            <span class="font-display text-xl font-bold tracking-[0.18em] uppercase text-white">
              {m.footer_logo()}
            </span>
          {/if}
        </a>
        <p class="mt-3 text-sm font-body leading-relaxed text-white/60 max-w-md">
          {slogan || m.footer_tagline()}
        </p>

        {#if navItems.length > 0}
          <nav class="mt-6">
            <ul class="flex flex-col sm:flex-row sm:flex-wrap gap-x-6 gap-y-2.5 text-sm font-body">
              {#each navItems as item}
                <li>
                  <a
                    href={safeNavUrl(item.url)}
                    target={item.target ?? '_self'}
                    rel={item.target === '_blank' ? 'noopener noreferrer' : undefined}
                    class="hover:text-white transition-colors"
                  >
                    {item.label}
                  </a>
                </li>
              {/each}
            </ul>
          </nav>
        {/if}
      </div>

      <!-- Right: social icons -->
      {#if visibleSocials.length > 0}
        <div class="md:flex-shrink-0">
          <ul class="flex flex-wrap items-center gap-4 md:justify-end">
            {#each visibleSocials as entry}
              <li>
                <a
                  href={safeNavUrl(entry.url)}
                  target="_blank"
                  rel="noopener noreferrer"
                  aria-label={socialLabel(entry)}
                  class="text-white/70 hover:text-white transition-colors inline-flex"
                >
                  <SocialIcon {entry} class="h-5 w-5" />
                </a>
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>

    <!-- Below-footer bar -->
    <div class="mt-12 md:mt-16 pt-6 border-t border-white/10 flex flex-col sm:flex-row items-center justify-center gap-4 text-xs text-white/50">
      <p class="font-body">{m.footer_copyright({ year: new Date().getFullYear() })}</p>
    </div>
  </div>
</footer>
