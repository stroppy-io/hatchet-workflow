import { createRegistry, toJson, type Message, type Registry } from "@bufbuild/protobuf";
import type { GenFile } from "@bufbuild/protobuf/codegenv2";
import { anyUnpack, type Any } from "@bufbuild/protobuf/wkt";

// A registry of EVERY generated proto file. Built from the `file_*` descriptor
// each `*_pb.ts` exports, collected via a Vite glob so new protos are picked up
// automatically. Needed to unpack google.protobuf.Any payloads (e.g. DAG task
// in/out) — the wire is binary+opaque, so decoding to a real message requires
// the concrete type to be registered. See [[project_connect_binary_format]].
const modules = import.meta.glob("./proto/**/*_pb.ts", { eager: true }) as Record<string, Record<string, unknown>>;

const files: GenFile[] = [];
for (const mod of Object.values(modules)) {
  for (const [name, value] of Object.entries(mod)) {
    if (name.startsWith("file_") && value) files.push(value as GenFile);
  }
}

export const registry: Registry = createRegistry(...files);

export type DecodedAny = {
  // Fully-qualified message type, e.g. "cloud.v1.deployment.Deployment".
  typeName: string;
  // The unpacked concrete message (typed access), or null when unknown to the registry.
  message: Message | null;
  // toJson of the unpacked message, or null when the type isn't registered.
  json: unknown | null;
  bytes: number;
};

// decodeAny unpacks an Any into its concrete message (+ JSON form). Returns null
// when the Any is empty; message/json are null when the type isn't registered.
export function decodeAny(any?: Any): DecodedAny | null {
  if (!any?.typeUrl) return null;
  const typeName = any.typeUrl.slice(any.typeUrl.lastIndexOf("/") + 1);
  const bytes = any.value?.length ?? 0;
  const message = anyUnpack(any, registry);
  if (!message) return { typeName, message: null, json: null, bytes };
  const schema = registry.getMessage(message.$typeName);
  if (!schema) return { typeName, message, json: null, bytes };
  try {
    return { typeName, message, json: toJson(schema, message, { registry }), bytes };
  } catch {
    return { typeName, message, json: null, bytes };
  }
}
