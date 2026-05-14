<script lang="ts">
  import { enhance } from '$app/forms';
  import type { PageData, ActionData } from './$types';
  import { notify } from '$lib/stores/notifications.svelte';
  import * as m from '$lib/paraglide/messages';
  import CF7TagToolbar from '$lib/components/admin/CF7TagToolbar.svelte';

  let { data, form: actionData }: { data: PageData; form: ActionData } = $props();

  const isNew = !data.form;
  const initial = data.form;

  // Restore the values the admin just submitted if validation failed; otherwise
  // hydrate from the DB row (edit) or defaults (new).
  const submitted = (actionData as { values?: Record<string, string | boolean> } | null)?.values;
  const get = (key: string, fallback: string) =>
    typeof submitted?.[key] === 'string' ? (submitted[key] as string) : fallback;

  let title = $state(get('title', initial?.title ?? ''));
  let slug = $state(get('slug', initial?.slug ?? ''));
  let markup = $state(get('markup', initial?.markup ?? defaultMarkup()));
  let markupTextareaEl = $state<HTMLTextAreaElement | null>(null);

  let mailTo = $state(get('mail_to', initial?.mail_to ?? ''));
  let mailFrom = $state(get('mail_from', initial?.mail_from ?? ''));
  let mailSubject = $state(get('mail_subject', initial?.mail_subject ?? defaultSubject()));
  let mailBody = $state(get('mail_body', initial?.mail_body ?? defaultBody()));
  let mailReplyTo = $state(get('mail_reply_to', initial?.mail_reply_to ?? '[your-email]'));

  let replyEnabled = $state<boolean>(
    typeof submitted?.reply_enabled === 'boolean'
      ? (submitted.reply_enabled as boolean)
      : (initial?.reply_enabled ?? false)
  );
  let replyToField = $state(get('reply_to_field', initial?.reply_to_field ?? 'your-email'));
  let replyFrom = $state(get('reply_from', initial?.reply_from ?? ''));
  let replySubject = $state(get('reply_subject', initial?.reply_subject ?? 'Thank you for your message'));
  let replyBody = $state(get('reply_body', initial?.reply_body ?? defaultReplyBody()));

  let successMessage = $state(get('success_message', initial?.success_message ?? 'Thank you for your message.'));
  let errorMessage = $state(
    get('error_message', initial?.error_message ?? 'There was an error. Please try again.')
  );
  let recaptchaAction = $state(get('recaptcha_action', initial?.recaptcha_action ?? 'contact_form'));

  // Auto-generate slug from title for new forms.
  function onTitleInput() {
    if (isNew) {
      slug = title
        .toLowerCase()
        .replace(/[^a-z0-9\s-]/g, '')
        .replace(/\s+/g, '-')
        .replace(/-+/g, '-')
        .replace(/^-|-$/g, '');
    }
  }

  // Show server-returned parse errors + field errors inline.
  const parseErrors = $derived(
    (actionData as { parseErrors?: { position: number; tag?: string; message: string }[] } | null)
      ?.parseErrors ?? []
  );
  const fieldErrors = $derived(
    ((actionData as { fields?: Record<string, string> } | null)?.fields) ?? {}
  );

  function defaultMarkup() {
    return `[text* your-name placeholder "Your name"]
[email* your-email placeholder "you@example.com"]
[textarea* your-message placeholder "How can we help?"]
[submit "Send message"]`;
  }
  function defaultSubject() {
    return 'New contact form submission';
  }
  function defaultBody() {
    return `New submission from your website:

Name:    [your-name]
Email:   [your-email]
Message:
[your-message]
`;
  }
  function defaultReplyBody() {
    return `Hi [your-name],

Thanks for reaching out — we've received your message and will get back to you soon.

— Gyeon
`;
  }
</script>

<svelte:head><title>{isNew ? m.admin_forms_new_title() : m.admin_forms_edit_title({ title: initial?.title ?? '' })}</title></svelte:head>

<div class="max-w-4xl mx-auto space-y-6">
  <div class="flex items-center gap-4">
    <a
      href="/admin/forms"
      class="p-2 rounded-xl text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
      aria-label={m.admin_forms_back_aria()}
    >
      ←
    </a>
    <h2 class="text-xl font-bold text-gray-900">{isNew ? m.admin_forms_new_heading() : m.admin_forms_edit_heading()}</h2>
  </div>

  {#if actionData && 'error' in actionData && actionData.error}
    <div class="rounded-xl border border-red-100 bg-red-50 px-4 py-3 text-sm text-red-700">
      {actionData.error}
    </div>
  {/if}

  <form
    method="POST"
    action="?/save"
    use:enhance={() => {
      return async ({ result, update }) => {
        if (result.type === 'redirect') notify.success(m.admin_forms_save_success());
        else if (result.type === 'failure') notify.error(m.admin_forms_save_failure(), m.admin_forms_save_failure_detail());
        await update();
      };
    }}
    class="space-y-6"
  >
    <!-- Basic info -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6 space-y-4">
      <h3 class="text-sm font-semibold text-gray-900 uppercase tracking-wide">{m.admin_forms_section_form()}</h3>

      <div>
        <label for="title" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_title()}</label>
        <input
          id="title"
          name="title"
          required
          bind:value={title}
          oninput={onTitleInput}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
        {#if fieldErrors.title}
          <p class="text-xs text-red-500 mt-1">{fieldErrors.title}</p>
        {/if}
      </div>

      <div>
        <label for="slug" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_slug()}</label>
        <input
          id="slug"
          name="slug"
          required
          bind:value={slug}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
        <p class="text-xs text-gray-400 mt-1">{m.admin_forms_slug_hint_pre()} <code class="font-mono">[contact-form id="{slug || 'slug'}"]</code></p>
        {#if fieldErrors.slug}
          <p class="text-xs text-red-500 mt-1">{fieldErrors.slug}</p>
        {/if}
      </div>

      <div>
        <label for="markup" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_markup()}</label>
        <div class="mt-1">
          <CF7TagToolbar bind:value={markup} textarea={markupTextareaEl} />
        </div>
        <textarea
          id="markup"
          name="markup"
          rows={10}
          bind:value={markup}
          bind:this={markupTextareaEl}
          class="block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        ></textarea>
        <p class="text-xs text-gray-400 mt-1">{m.admin_forms_markup_hint()}</p>
        {#if parseErrors.length > 0}
          <div class="mt-2 rounded-xl border border-red-100 bg-red-50 px-3 py-2 text-sm text-red-700 space-y-1">
            <p class="font-semibold">{m.admin_forms_parser_errors()}</p>
            <ul class="list-disc ml-5">
              {#each parseErrors as pe}
                <li>
                  <span class="font-mono text-xs">{pe.tag || '?'}</span> — {pe.message}
                </li>
              {/each}
            </ul>
          </div>
        {/if}
      </div>
    </section>

    <!-- Admin notification mail -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6 space-y-4">
      <h3 class="text-sm font-semibold text-gray-900 uppercase tracking-wide">{m.admin_forms_section_notification()}</h3>

      <div>
        <label for="mail_to" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_to()}</label>
        <input
          id="mail_to"
          name="mail_to"
          required
          type="email"
          bind:value={mailTo}
          placeholder="admin@example.com"
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
        {#if fieldErrors.mail_to}
          <p class="text-xs text-red-500 mt-1">{fieldErrors.mail_to}</p>
        {/if}
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <label for="mail_from" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_from()} <span class="text-gray-400">{m.admin_forms_label_optional()}</span></label>
          <input
            id="mail_from"
            name="mail_from"
            type="email"
            bind:value={mailFrom}
            placeholder={m.admin_forms_mail_from_placeholder()}
            class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
          />
        </div>
        <div>
          <label for="mail_reply_to" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_reply_to()}</label>
          <input
            id="mail_reply_to"
            name="mail_reply_to"
            bind:value={mailReplyTo}
            placeholder="[your-email]"
            class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
          />
        </div>
      </div>

      <div>
        <label for="mail_subject" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_subject()}</label>
        <input
          id="mail_subject"
          name="mail_subject"
          required
          bind:value={mailSubject}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
      </div>

      <div>
        <label for="mail_body" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_body()}</label>
        <textarea
          id="mail_body"
          name="mail_body"
          required
          rows={8}
          bind:value={mailBody}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        ></textarea>
        <p class="text-xs text-gray-400 mt-1">
          {m.admin_forms_mail_body_hint()}
        </p>
      </div>
    </section>

    <!-- Auto-reply -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6 space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="text-sm font-semibold text-gray-900 uppercase tracking-wide">{m.admin_forms_section_autoreply()}</h3>
        <label class="inline-flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            name="reply_enabled"
            value="true"
            bind:checked={replyEnabled}
            class="rounded"
          />
          <span>{m.admin_forms_reply_enabled()}</span>
        </label>
      </div>

      {#if replyEnabled}
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label for="reply_to_field" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_reply_field()}</label>
            <input
              id="reply_to_field"
              name="reply_to_field"
              bind:value={replyToField}
              placeholder="your-email"
              class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
            />
            <p class="text-xs text-gray-400 mt-1">{m.admin_forms_reply_field_hint()}</p>
          </div>
          <div>
            <label for="reply_from" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_from()} <span class="text-gray-400">{m.admin_forms_label_optional()}</span></label>
            <input
              id="reply_from"
              name="reply_from"
              type="email"
              bind:value={replyFrom}
              placeholder={m.admin_forms_mail_from_placeholder()}
              class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
            />
          </div>
        </div>
        <div>
          <label for="reply_subject" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_subject()}</label>
          <input
            id="reply_subject"
            name="reply_subject"
            bind:value={replySubject}
            class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
          />
        </div>
        <div>
          <label for="reply_body" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_mail_body()}</label>
          <textarea
            id="reply_body"
            name="reply_body"
            rows={8}
            bind:value={replyBody}
            class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
          ></textarea>
        </div>
      {/if}
    </section>

    <!-- Messages + recaptcha action -->
    <section class="bg-white rounded-2xl border border-gray-100 p-6 space-y-4">
      <h3 class="text-sm font-semibold text-gray-900 uppercase tracking-wide">{m.admin_forms_section_messages()}</h3>
      <div>
        <label for="success_message" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_success_message()}</label>
        <input
          id="success_message"
          name="success_message"
          bind:value={successMessage}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
      </div>
      <div>
        <label for="error_message" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_error_message()}</label>
        <input
          id="error_message"
          name="error_message"
          bind:value={errorMessage}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
      </div>
      <div>
        <label for="recaptcha_action" class="block text-sm font-medium text-gray-700">{m.admin_forms_label_recaptcha_action()}</label>
        <input
          id="recaptcha_action"
          name="recaptcha_action"
          bind:value={recaptchaAction}
          class="mt-1 block w-full rounded-xl border border-gray-200 px-3.5 py-2.5 font-mono text-sm focus:border-gray-900 focus:outline-none focus:ring-1 focus:ring-gray-900"
        />
      </div>
    </section>

    <div class="flex justify-end gap-3 pt-2">
      <a href="/admin/forms" class="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">{m.admin_forms_cancel()}</a>
      <button type="submit" class="rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-semibold text-white hover:bg-gray-800">
        {isNew ? m.admin_forms_create_button() : m.admin_forms_save_button()}
      </button>
    </div>
  </form>
</div>
