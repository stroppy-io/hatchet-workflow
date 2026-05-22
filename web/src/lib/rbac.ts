import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";

export type TenantRole = TenantMember_Role;

export function roleLabel(role: TenantRole) {
  switch (role) {
    case TenantMember_Role.OWNER:
      return "OWNER";
    case TenantMember_Role.ADMIN:
      return "ADMIN";
    case TenantMember_Role.VIEWER:
      return "VIEWER";
    default:
      return "NO ROLE";
  }
}

export function canAccess(role: TenantRole, minimum: TenantRole) {
  return role >= minimum;
}

export const ROLE_VIEWER = TenantMember_Role.VIEWER;
export const ROLE_ADMIN = TenantMember_Role.ADMIN;
export const ROLE_OWNER = TenantMember_Role.OWNER;
