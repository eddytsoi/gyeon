# Gyeon

Fully tailor-made CMS + eShop platform.

## Project Overview

**Gyeon** is a custom-built content management system combined with an e-commerce storefront. It provides both a back-office admin interface for managing content and products, and a customer-facing storefront.

## Architecture

> To be defined as the project evolves.

### Planned Modules

- **CMS** — pages, posts, media library, navigation, localization
- **eShop** — products, categories, inventory, pricing, variants
- **Orders** — cart, checkout, order management, fulfillment
- **Customers** — accounts, addresses, purchase history
- **Admin** — dashboard, user roles & permissions, settings

## Tech Stack

| Layer | Technology |
|---|---|
| Frontend + SSR | SvelteKit |
| Styling | Tailwind CSS |
| Backend API | Go |
| Database | PostgreSQL |
| ORM / Query | sqlc (Go) |

### Design Principles
- Mobile-first responsive layout (Tailwind breakpoints: `sm` / `md` / `lg`)
- SSR for fast initial load, minimal client-side JS
- Go API handles all business logic (CMS content, eShop, orders)
- SvelteKit frontend communicates with Go via REST API

## Development

### Getting Started

```bash
# To be added
```

### Running Locally

```bash
# To be added
```

### Testing

```bash
# To be added
```

## Conventions

- Keep modules decoupled — CMS and eShop can be used independently
- All user-facing content must support localization from day one
- Admin and storefront are separate surfaces (separate routing / apps)

## Environment Variables

> Document required env vars here as they are introduced.
