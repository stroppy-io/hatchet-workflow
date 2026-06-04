// Registration-request API: the closed-signup access-request flow.
//
// - submitRegistrationRequest is PUBLIC (the /register page calls it when
//   self-registration is disabled), so it lives outside the admin provider.
// - listRegistrationRequests / markHandled are admin-only, used by the admin
//   console.
//
// All three ride IamService (iamClient) — no separate client needed.

import { toJson } from "@bufbuild/protobuf";
import {
  RegistrationRequestSchema,
  RegistrationRequestStatus,
  type RegistrationRequest,
  type RegistrationRequestJson,
} from "@/lib/proto/cloud/v1/api/iam_pb";
import { iamClient } from "@/services/client";

export type RegistrationRequestStatusName = "pending" | "handled" | "unknown";

export interface RegistrationRequestVM {
  id: string;
  email: string;
  message: string;
  status: RegistrationRequestStatusName;
  handledByAccountId: string;
  createdAt?: string;
  updatedAt?: string;
}

function statusName(s: RegistrationRequestStatus): RegistrationRequestStatusName {
  switch (s) {
    case RegistrationRequestStatus.PENDING:
      return "pending";
    case RegistrationRequestStatus.HANDLED:
      return "handled";
    default:
      return "unknown";
  }
}

function toVM(r: RegistrationRequest): RegistrationRequestVM {
  const j = toJson(RegistrationRequestSchema, r) as RegistrationRequestJson;
  return {
    id: j.id ?? "",
    email: j.email ?? "",
    message: j.message ?? "",
    status: statusName(r.status),
    handledByAccountId: j.handledByAccountId ?? "",
    createdAt: j.createdAt,
    updatedAt: j.updatedAt,
  };
}

/** Public: leave an access request while self-signup is closed. */
export async function submitRegistrationRequest(
  email: string,
  message: string,
): Promise<void> {
  await iamClient.submitRegistrationRequest({ email, message });
}

/** Admin: list access requests (newest first); optionally filter by status. */
export async function listRegistrationRequests(
  status?: RegistrationRequestStatus,
): Promise<RegistrationRequestVM[]> {
  const { requests } = await iamClient.listRegistrationRequests({
    status: status ?? RegistrationRequestStatus.UNSPECIFIED,
  });
  return requests.map(toVM);
}

/** Admin: flip a request to HANDLED. */
export async function markRegistrationRequestHandled(
  id: string,
): Promise<RegistrationRequestVM | null> {
  const { request } = await iamClient.markRegistrationRequestHandled({ id });
  return request ? toVM(request) : null;
}
