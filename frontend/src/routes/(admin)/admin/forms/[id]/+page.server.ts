import {
  adminGetForm,
  adminCreateForm,
  adminUpdateForm,
  adminGetPages,
  type AdminForm,
  type FormParseError,
  type UpsertFormBody
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  // Pages drive the success/error redirect dropdowns. 200 is generous —
  // sites with more than that should switch to a search-driven picker,
  // but for now a flat list keeps the UI dead simple.
  const pagesPromise = adminGetPages(token, 200, 0).catch(() => ({ items: [], total: 0 }));
  if (params.id === 'new') {
    const pages = await pagesPromise;
    return { form: null as AdminForm | null, pages: pages.items };
  }
  const [form, pages] = await Promise.all([
    adminGetForm(token, params.id).catch(() => null),
    pagesPromise
  ]);
  return { form, pages: pages.items };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

    const successMode = (data.get('success_mode') as string) === 'redirect' ? 'redirect' : 'message';
    const errorMode = (data.get('error_mode') as string) === 'redirect' ? 'redirect' : 'message';
    const rawSuccessPageID = ((data.get('success_page_id') as string) || '').trim();
    const rawErrorPageID = ((data.get('error_page_id') as string) || '').trim();

    const body: UpsertFormBody = {
      slug: ((data.get('slug') as string) || '').trim(),
      title: ((data.get('title') as string) || '').trim(),
      markup: (data.get('markup') as string) || '',

      mail_to: ((data.get('mail_to') as string) || '').trim(),
      mail_from: ((data.get('mail_from') as string) || '').trim(),
      mail_subject: ((data.get('mail_subject') as string) || '').trim(),
      mail_body: (data.get('mail_body') as string) || '',
      mail_reply_to: ((data.get('mail_reply_to') as string) || '').trim(),

      reply_enabled: data.get('reply_enabled') === 'true',
      reply_to_field: ((data.get('reply_to_field') as string) || '').trim(),
      reply_from: ((data.get('reply_from') as string) || '').trim(),
      reply_subject: ((data.get('reply_subject') as string) || '').trim(),
      reply_body: (data.get('reply_body') as string) || '',

      success_message: ((data.get('success_message') as string) || '').trim(),
      error_message: ((data.get('error_message') as string) || '').trim(),
      recaptcha_action: ((data.get('recaptcha_action') as string) || 'contact_form').trim(),

      success_mode: successMode,
      error_mode: errorMode,
      success_page_id: successMode === 'redirect' && rawSuccessPageID ? rawSuccessPageID : null,
      error_page_id: errorMode === 'redirect' && rawErrorPageID ? rawErrorPageID : null
    };

    const result =
      params.id === 'new'
        ? await adminCreateForm(token, body)
        : await adminUpdateForm(token, params.id, body);

    if (!result.ok) {
      return fail(422, {
        error: result.error ?? 'Save failed',
        parseErrors: (result.parseErrors ?? []) as FormParseError[],
        fields: result.fields ?? {},
        values: body
      });
    }

    redirect(303, '/admin/forms');
  }
};
