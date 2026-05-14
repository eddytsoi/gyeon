<script lang="ts">
  import type { PageData } from './$types';
  import { page } from '$app/state';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import SectionHead from '$lib/components/shop/SectionHead.svelte';
  import Eyebrow from '$lib/components/shop/Eyebrow.svelte';
  import Section from '$lib/components/shop/Section.svelte';
  import MarkdownContent from '$lib/components/MarkdownContent.svelte';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin, snippet } from '$lib/seo';

  let { data }: { data: PageData } = $props();

  const homeOrigin = $derived(siteOrigin(page.data.publicSettings));
  const homeJsonLd = $derived({
    '@context': 'https://schema.org',
    '@type': 'WebSite',
    name: 'Gyeon',
    url: homeOrigin,
    potentialAction: {
      '@type': 'SearchAction',
      target: `${homeOrigin}/products?q={search_term_string}`,
      'query-input': 'required name=search_term_string'
    }
  });
</script>

{#if data.mode === 'page'}
  <Seo
    title={`${data.page.meta_title ?? data.page.title} — Gyeon`}
    description={data.page.meta_desc ?? snippet(data.page.content)}
    canonical={homeOrigin}
  />

  <div class="max-w-3xl mx-auto px-4 {data.page.content_padded === false ? '' : 'py-12 sm:py-16'}">
    {#if data.page.show_title}
      <h1 class="text-3xl sm:text-4xl font-bold text-gray-900 tracking-tight mb-8">
        {data.page.title}
      </h1>
    {/if}
    <div class="text-gray-700 text-base leading-relaxed">
      <MarkdownContent content={data.page.content} refs={data.shortcodeRefs} />
    </div>
  </div>
{:else}
  <Seo
    title={m.home_title()}
    description={m.home_meta_description()}
    canonical={homeOrigin}
    jsonLd={homeJsonLd}
  />

  <!--
    Hero — editorial split (gyeon-project-design-system §2.2)
    Mobile (< md): figure stacks above text. Desktop (md+): 7/5 grid, text left.
    Hero image asset goes in the <figure> on the right; until then a paper/cream
    panel keeps the rhythm.
  -->
  <Section bg="paper" layout="hero" padding="md">
    <!-- Text column -->
    <div class="md:col-span-7 order-2 md:order-1">
      <Eyebrow class="mb-4">New Season</Eyebrow>
      <h1 class="font-display font-bold tracking-tight leading-[1.02] text-ink-900
                 text-4xl sm:text-5xl md:text-6xl lg:text-7xl">
        {m.home_hero_heading()}
      </h1>
      <p class="mt-5 md:mt-6 text-base md:text-lg text-ink-900/70 max-w-md font-body leading-relaxed">
        {m.home_hero_subheading()}
      </p>
      <div class="mt-8 flex flex-wrap items-center gap-4">
        <a href="/products"
           class="inline-flex items-center gap-2 bg-navy-500 hover:bg-navy-700 text-white
                  font-display font-bold tracking-wide uppercase
                  px-7 py-3.5 rounded-md transition-colors">
          {m.home_hero_cta()}
          <svg class="w-4 h-4" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M13 5l7 7-7 7"/>
          </svg>
        </a>
        <a href="/products"
           class="inline-flex items-center gap-1.5 text-navy-500 font-display font-semibold
                  tracking-wide uppercase underline-offset-4 hover:underline">
          Discover
          <svg class="w-3.5 h-3.5" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
          </svg>
        </a>
      </div>
    </div>

    <!-- Visual column -->
    <figure class="md:col-span-5 order-1 md:order-2 aspect-[4/5] w-full overflow-hidden rounded-lg
                   bg-gradient-to-br from-navy-900 via-navy-700 to-navy-500 relative">
      <!--
        Placeholder until a hero asset is wired. Geometry is composed in SVG so
        it scales clean at any breakpoint and avoids fetching an image asset.
      -->
      <svg class="absolute inset-0 w-full h-full" viewBox="0 0 400 500" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
        <defs>
          <radialGradient id="heroGlow" cx="30%" cy="20%" r="80%">
            <stop offset="0%" stop-color="#FED022" stop-opacity="0.18"/>
            <stop offset="60%" stop-color="#FED022" stop-opacity="0"/>
          </radialGradient>
        </defs>
        <rect width="400" height="500" fill="url(#heroGlow)"/>
        <circle cx="280" cy="180" r="120" fill="none" stroke="rgba(255,255,255,0.12)" stroke-width="1"/>
        <circle cx="280" cy="180" r="80"  fill="none" stroke="rgba(255,255,255,0.18)" stroke-width="1"/>
        <circle cx="280" cy="180" r="40"  fill="rgba(248,153,93,0.4)"/>
      </svg>
      <figcaption class="absolute bottom-5 left-5 right-5 text-white/80
                          font-display text-[11px] uppercase tracking-[0.18em] flex justify-between">
        <span>GYEON · 量身定製</span>
        <span>HK</span>
      </figcaption>
    </figure>
  </Section>

  <!-- Featured Products -->
  <section class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-16 md:pt-24 pb-12 md:pb-16">
    <SectionHead eyebrow="Featured" title={m.home_featured_heading()} />

    {#if data.products.length === 0}
      <div class="text-center py-20 text-ink-500">
        <p class="text-lg">{m.home_no_products()}</p>
        <a href="/products" class="mt-2 inline-block text-sm underline underline-offset-4 hover:text-navy-500">
          {m.home_browse_all()}
        </a>
      </div>
    {:else}
      <ul class="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-x-4 gap-y-10 md:gap-x-6 md:gap-y-12">
        {#each data.products as item (item.product.id)}
          <li>
            <ProductCard
              product={item.product}
              image={item.primaryImage}
              variant={item.cheapestVariant}
            />
          </li>
        {/each}
      </ul>
      <div class="mt-12 md:mt-16 text-center">
        <a href="/products"
           class="inline-block font-display font-bold tracking-wide uppercase
                  px-8 py-3 border-2 border-navy-500 text-navy-500
                  hover:bg-navy-500 hover:text-white rounded-md transition-colors">
          {m.home_view_all_products()}
        </a>
      </div>
    {/if}
  </section>

  <!--
    Editorial break — gyeon-project-design-system §2.4
    Full-bleed bg-cream strip, two-column on md+, image stacks above text on mobile.
    Figure is an SVG composition until a real campaign asset lands.
  -->
  <Section bg="cream" layout="split" padding="lg">
    <figure class="aspect-square overflow-hidden rounded-lg bg-navy-900 relative order-1">
      <svg class="absolute inset-0 w-full h-full" viewBox="0 0 500 500" preserveAspectRatio="xMidYMid slice" aria-hidden="true">
        <defs>
          <linearGradient id="craftGrad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%"  stop-color="#21314D"/>
            <stop offset="100%" stop-color="#19253F"/>
          </linearGradient>
          <radialGradient id="craftGlow" cx="70%" cy="30%" r="50%">
            <stop offset="0%" stop-color="#3692C0" stop-opacity="0.5"/>
            <stop offset="100%" stop-color="#3692C0" stop-opacity="0"/>
          </radialGradient>
        </defs>
        <rect width="500" height="500" fill="url(#craftGrad)"/>
        <rect width="500" height="500" fill="url(#craftGlow)"/>
        <g transform="translate(250 250)">
          <circle r="170" fill="none" stroke="rgba(255,255,255,0.08)" stroke-width="1"/>
          <circle r="120" fill="none" stroke="rgba(255,255,255,0.12)" stroke-width="1"/>
          <circle r="70"  fill="none" stroke="rgba(255,255,255,0.18)" stroke-width="1"/>
          <circle r="30"  fill="rgba(248,153,93,0.35)"/>
        </g>
      </svg>
      <figcaption class="absolute bottom-5 left-5 right-5 flex items-center justify-between
                          font-display text-[11px] uppercase tracking-[0.18em] text-white/70">
        <span>Q² CERAMIC</span>
        <span>SINCE 2011</span>
      </figcaption>
    </figure>

    <div class="order-2">
      <Eyebrow class="mb-3">Our Craft</Eyebrow>
      <h3 class="font-display text-3xl md:text-4xl lg:text-5xl font-bold tracking-tight leading-tight text-ink-900 mb-4 md:mb-5">
        為高端漆面而生的鍍膜配方
      </h3>
      <p class="font-body text-base md:text-lg leading-[1.75] text-ink-900/75 max-w-md">
        每一支 GYEON 都是在波蘭原廠手工調配，
        從原料到成品經過 14 道品質檢測，
        為每一道漆面提供最穩定可靠的長效保護。
      </p>
      <a href="/products"
         class="mt-6 md:mt-8 inline-flex items-center gap-2 text-navy-500 hover:text-navy-700
                font-display font-semibold uppercase tracking-wide
                underline-offset-4 hover:underline transition-colors">
        關於 GYEON
        <svg class="w-3.5 h-3.5" aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7"/>
        </svg>
      </a>
    </div>
  </Section>
{/if}
