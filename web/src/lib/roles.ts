// Shared role-level helpers for role-gating across the shell.
//
// Roles are ordered viewer < operator < owner. `roleLevel` maps each to a
// numeric level; `roleAtLeast` is the gate used by the sidebar, org pages, and
// any authoring affordance.

import type { TenantRole } from "@/services/auth";

export const roleLevel: Record<TenantRole, number> = {
  viewer: 1,
  operator: 2,
  owner: 3,
};

/** True when `role` is at least the required role. */
export function roleAtLeast(role: TenantRole, min: TenantRole): boolean {
  return roleLevel[role] >= roleLevel[min];
}

/** Human-facing label for a role. */
export function roleLabel(role: TenantRole): string {
  return role.charAt(0).toUpperCase() + role.slice(1);
}
