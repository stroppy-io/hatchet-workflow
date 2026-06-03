// TEMPORARY mock — MUST be deleted before real API wiring; do not build on this.
//
// Canned auth provider so the shell renders without a backend. Supplies one
// user (username drives the avatar seed) and a few tenants WITH slugs.

import type {
  AuthProvider,
  SessionTenant,
  SessionUser,
} from "@/services/auth";

const MOCK_TENANTS: SessionTenant[] = [
  { id: "t-acme", slug: "acme", name: "Acme Corporation", role: "owner" },
  { id: "t-globex", slug: "globex", name: "Globex Industries", role: "operator" },
  { id: "t-initech", slug: "initech", name: "Initech LLC", role: "viewer" },
];

const MOCK_USER: SessionUser = {
  id: "u-1",
  username: "ada.lovelace",
  displayName: "Ada Lovelace",
  email: "ada@stroppy.cloud",
  isAdmin: true,
  tenants: MOCK_TENANTS,
};

export const mockAuthProvider: AuthProvider = {
  async getSession() {
    return MOCK_USER;
  },
  async logout() {
    // No real session to tear down under the mock.
  },
};
