import { iamClient } from "@/services/client";

const tenantIdBySlug = new Map<string, string>();

export async function resolveTenantId(tenantSlug: string): Promise<string> {
  const slug = tenantSlug.trim();
  if (!slug) throw new Error("tenant slug is required");

  const cached = tenantIdBySlug.get(slug);
  if (cached) return cached;

  const resp = await iamClient.getTenant({
    ref: { case: "slug", value: slug },
  });
  const id = resp.tenant?.id ?? "";
  if (!id) throw new Error(`tenant ${slug} was not resolved`);

  tenantIdBySlug.set(slug, id);
  return id;
}
