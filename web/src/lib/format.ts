import type { Timestamp } from "@bufbuild/protobuf/wkt";

import { Provider } from "@/lib/proto/cloud/v1/deployment/deployment_pb.ts";
import { Preset_Kind } from "@/lib/proto/cloud/v1/models/preset_pb.ts";
import { TenantMember_Role } from "@/lib/proto/cloud/v1/models/tenant_pb.ts";
import { Webhook_Event } from "@/lib/proto/cloud/v1/models/webhook_pb.ts";
import { Status } from "@/lib/proto/cloud/v1/runtime/primitive/status_pb.ts";

export function formatId(value?: string) {
  if (!value) return "—";
  if (value.length <= 12) return value;
  return `${value.slice(0, 8)}…${value.slice(-4)}`;
}

export function formatTimestamp(timestamp?: Timestamp) {
  if (!timestamp) return "—";
  const millis = Number(timestamp.seconds) * 1000 + Math.floor(timestamp.nanos / 1_000_000);
  if (!Number.isFinite(millis) || millis <= 0) return "—";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(millis));
}

export function formatBool(value: boolean | undefined, trueLabel = "Yes", falseLabel = "No") {
  if (value === undefined) return "Any";
  return value ? trueLabel : falseLabel;
}

export function presetKindLabel(kind: Preset_Kind) {
  switch (kind) {
    case Preset_Kind.WORKLOAD:
      return "Workload";
    case Preset_Kind.DATABASE:
      return "Database";
    case Preset_Kind.TEST:
      return "Test";
    default:
      return "Unspecified";
  }
}

export function roleLabel(role: TenantMember_Role) {
  switch (role) {
    case TenantMember_Role.VIEWER:
      return "Viewer";
    case TenantMember_Role.ADMIN:
      return "Admin";
    case TenantMember_Role.OWNER:
      return "Owner";
    default:
      return "Unspecified";
  }
}

export function statusLabel(status?: Status) {
  switch (status) {
    case Status.PENDING:
      return "Pending";
    case Status.RUNNING:
      return "Running";
    case Status.RETRY_WAIT:
      return "Retry wait";
    case Status.COMPLETED:
      return "Completed";
    case Status.FAILED:
      return "Failed";
    case Status.SKIPPED:
      return "Skipped";
    default:
      return "Any";
  }
}

export function providerLabel(provider?: Provider) {
  switch (provider) {
    case Provider.DOCKER:
      return "Docker";
    case Provider.YANDEX:
      return "Yandex Cloud";
    default:
      return "Provider";
  }
}

export function webhookEventLabel(event: Webhook_Event) {
  switch (event) {
    case Webhook_Event.RUN_COMPLETED:
      return "Run completed";
    case Webhook_Event.RUN_FAILED:
      return "Run failed";
    case Webhook_Event.SUITE_COMPLETED:
      return "Suite completed";
    case Webhook_Event.SUITE_FAILED:
      return "Suite failed";
    default:
      return "Unspecified";
  }
}
