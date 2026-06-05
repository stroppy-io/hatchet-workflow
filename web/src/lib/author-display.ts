import { toJson } from "@bufbuild/protobuf";
import {
  AccountSchema,
  type Account,
  type AccountJson,
} from "@/lib/proto/cloud/v1/iam/account_pb";
import { iamClient } from "@/services/client";

export interface AuthorDisplay {
  label: string;
  title: string;
  avatarName: string;
}

export function fallbackAuthorDisplay(id: string): AuthorDisplay {
  return { label: id, title: id, avatarName: id };
}

export function authorDisplayFromAccount(id: string, account?: Account): AuthorDisplay {
  if (!account) return fallbackAuthorDisplay(id);
  const j = toJson(AccountSchema, account) as AccountJson;
  const nickname = j.nickname ?? "";
  const email = j.email ?? "";
  const label = nickname || email || id;
  const titleParts = [email && email !== label ? email : null, id].filter(Boolean);
  return {
    label,
    title: titleParts.length ? `${label} · ${titleParts.join(" · ")}` : label,
    avatarName: label,
  };
}

export async function resolveAuthorDisplay(id: string): Promise<AuthorDisplay> {
  if (!id) return fallbackAuthorDisplay(id);
  try {
    const { account } = await iamClient.getAccount({ id });
    return authorDisplayFromAccount(id, account);
  } catch {
    return fallbackAuthorDisplay(id);
  }
}
