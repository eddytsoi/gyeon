import {
  adminGetForm,
  adminCreateForm,
  adminUpdateForm,
  type AdminForm,
  type FormParseError,
  type UpsertFormBody
} from '$lib/api/admin';
import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ params, cookies }) => {
  const token = cookies.get('admin_token') ?? '';
  if (params.id === 'new') {
    return { form: null as AdminForm | null };
  }
  const form = await adminGetForm(token, params.id).catch(() => null);
  return { form };
};

export const actions: Actions = {
  save: async ({ request, params, cookies }) => {
    const token = cookies.get('admin_token') ?? '';
    const data = await request.formData();

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
      recaptcha_action: ((data.get('recaptcha_action') as string) || 'contact_form').trim()
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
