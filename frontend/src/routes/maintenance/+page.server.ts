import type { PageServerLoad } from './$types';
import { redirect } from '@sveltejs/kit';

const API_BASE = process.env.API_BASE ?? 'http://localhost:8080/api/v1';

async function isMaintenanceMode(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/settings`, { signal: AbortSignal.timeout(2000) });
    if (!res.ok) return false;
    const settings: { key: string; value: string }[] = await res.json();
    return settings.find((s) => s.key === 'maintenance_mode')?.value === 'true';
  } catch {
    return false;
  }
}

export const load: PageServerLoad = async () => {
  if (!(await isMaintenanceMode())) {
    throw redirect(302, '/');
  }
};
