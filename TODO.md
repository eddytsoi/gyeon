# Gyeon — TODO

## Backend

### Auth
- [x] JWT authentication + middleware
- [x] Multi-user admin auth with roles (super_admin / admin / editor)
- [x] Customer auth (registration, login, JWT)

### CMS
- [x] Pages (CRUD)
- [x] Posts (CRUD)
- [x] Post categories
- [x] Navigation / menus

### eShop
- [x] Products (CRUD)
- [x] Product categories
- [x] Product variants (full CRUD + stock adjust)
- [x] Product images (full CRUD, set primary)
- [x] Pricing rules (discount campaigns, coupon codes)

### Orders
- [x] Cart (add / remove / view)
- [x] Checkout (order creation)
- [x] Order management (list, detail)
- [x] Fulfillment & cancellation

### Customers
- [x] Registration & login
- [x] Profile management
- [x] Addresses (CRUD)
- [x] Purchase history

### Admin
- [x] Dashboard stats
- [x] Admin user management (create / edit / delete with roles)

### Platform
- [x] Site settings API (key-value store)
- [x] Media library (file upload + management)
- [x] i18n support for content (CMS pages, posts, products)

### Database
- [x] Migration 001 — eShop (products, categories, variants, images)
- [x] Migration 002 — Orders (orders, order items, cart)
- [x] Migration 003 — CMS (pages, posts)
- [x] Migration 004 — Navigation
- [x] Migration 005 — Customer auth (accounts, addresses)
- [x] Migration 006 — Settings & media
- [x] Migration 007 — Admin users
- [x] Migration 008 — Pricing (campaigns, coupon codes)
- [x] Migration 009 — i18n

### SQL Queries (sqlc)
- [x] Products
- [x] Product categories
- [x] Product variants
- [x] Product images
- [x] CMS pages
- [x] CMS posts

---

## Frontend (SvelteKit)

### Admin
- [x] Login / logout (email + password)
- [x] Dashboard (stats overview)
- [x] Products list
- [x] Product create / edit
- [x] Product variants management (add / edit / delete / stock adjust)
- [x] Product images management (add / set primary / delete)
- [x] Orders list
- [x] Order detail (fulfillment, cancellation)
- [x] Customer list
- [x] Customer detail (with order history)
- [x] CMS pages (list + create/edit)
- [x] CMS posts (list + create/edit)
- [x] Post categories
- [x] Navigation management
- [x] Site settings page
- [x] Admin user management (roles & permissions)

### Storefront
- [x] Home page
- [x] Products catalog
- [x] Product detail
- [x] Blog listing
- [x] Blog post detail
- [x] Static CMS pages
- [x] Shopping cart
- [ ] Checkout page & flow
- [x] Customer account: register, login, logout
- [x] Customer account: profile
- [x] Customer account: addresses (list / add / edit)
- [x] Customer account: order history + order detail
- [ ] Localization / language switcher

---

## Infrastructure & Docs

- [x] Docker Compose setup (PostgreSQL)
- [ ] Document environment variables in `CLAUDE.md`
- [ ] Getting Started / Running Locally guide
- [ ] Testing setup (backend: Go tests, frontend: Playwright or Vitest)
- [ ] CI/CD pipeline
- [ ] Architecture documentation (define module boundaries)
