import { fail, redirect } from '@sveltejs/kit';
import type { PageServerLoad, Actions } from './$types';
import { adminCreateCustomer, adminSendResetPasswordEmail } from '$lib/api/admin';
import type { CustomerRole } from '$lib/types';

export const load: PageServerLoad = async ({ parent }) => {
  const { token } = await parent();
  if (!token) throw redirect(303, '/admin/login');
  return {};
};

export const actions: Actions = {
  default: async ({ request, cookies }) => {
    const token = cookies.get('admin_token');
    if (!token) throw redirect(303, '/admin/login');

    const fd = await request.formData();
    const first_name = String(fd.get('first_name') ?? '').trim();
    const last_name = String(fd.get('last_name') ?? '').trim();
    const email = String(fd.get('email') ?? '')
      .trim()
      .toLowerCase();
    const phone = String(fd.get('phone') ?? '').trim();
    const roleRaw = String(fd.get('role') ?? 'customer');
    const role: CustomerRole =
      roleRaw === 'installer' || roleRaw === 'installer_v2' ? roleRaw : 'customer';
    const sendSetupEmail = fd.get('send_setup_email') === 'on';

    if (!email || !email.includes('@')) {
      return fail(400, { error: 'invalid_email' });
    }
    if (!first_name) {
      return fail(400, { error: 'missing_first_name' });
    }

    let createdId: string;
    try {
      const created = await adminCreateCustomer(token, {
        first_name,
        last_name: last_name || undefined,
        email,
        phone: phone || undefined,
        role
      });
      createdId = created.id;
    } catch (err) {
      const msg = err instanceof Error ? err.message : '';
      if (msg.includes('409')) return fail(409, { error: 'email_taken' });
      return fail(502, { error: 'create_failed' });
    }

    // Optional, best-effort: fire the password-setup email so the customer can
    // pick their own password. A send failure must not roll back the (already
    // created) customer — surface it via a query flag and let the admin retry
    // from the detail page's "send reset password email" button.
    let emailFlag = '';
    if (sendSetupEmail) {
      try {
        await adminSendResetPasswordEmail(token, createdId);
        emailFlag = '&email_sent=1';
      } catch {
        emailFlag = '&email_failed=1';
      }
    }

    throw redirect(303, `/admin/customers/${createdId}?created=1${emailFlag}`);
  }
};
