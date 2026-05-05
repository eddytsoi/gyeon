<script lang="ts">
  import type { PageData } from './$types';
  import { page } from '$app/state';
  import ProductCard from '$lib/components/shop/ProductCard.svelte';
  import * as m from '$lib/paraglide/messages';
  import Seo from '$lib/components/Seo.svelte';
  import { siteOrigin } from '$lib/seo';

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

<Seo
  title={m.home_title()}
  description={m.home_meta_description()}
  canonical={homeOrigin}
  jsonLd={homeJsonLd}
/>

<!-- Hero -->
<section class="bg-gray-900 text-white">
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-20 md:py-32 text-center">
    <h1 class="text-4xl sm:text-5xl md:text-6xl font-bold tracking-tight">
      {m.home_hero_heading()}
    </h1>
    <p class="mt-4 text-lg text-gray-300 max-w-xl mx-auto">
      {m.home_hero_subheading()}
    </p>
    <a href="/products"
       class="mt-8 inline-block bg-white text-gray-900 font-semibold px-8 py-3
              rounded-full hover:bg-gray-100 transition-colors">
      {m.home_hero_cta()}
    </a>
  </div>
</section>

<!-- Featured Products -->
<section class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16">
  <h2 class="text-2xl font-bold text-gray-900 mb-8">{m.home_featured_heading()}</h2>

  {#if data.products.length === 0}
    <div class="text-center py-20 text-gray-400">
      <p class="text-lg">{m.home_no_products()}</p>
      <a href="/products" class="mt-2 inline-block text-sm underline">{m.home_browse_all()}</a>
    </div>
  {:else}
    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 md:gap-6">
      {#each data.products as item}
        <ProductCard
          product={item.product}
          image={item.primaryImage}
          variant={item.cheapestVariant}
        />
      {/each}
    </div>
    <div class="mt-10 text-center">
      <a href="/products"
         class="inline-block border border-gray-300 text-gray-700 font-medium px-8 py-3
                rounded-full hover:border-gray-900 hover:text-gray-900 transition-colors">
        {m.home_view_all_products()}
      </a>
    </div>
  {/if}
</section>
