export function tenantPath(tenantId: string, path = "") {
  const suffix = path.startsWith("/") ? path : `/${path}`;
  return `/t/${tenantId}${suffix === "/" ? "" : suffix}`;
}
