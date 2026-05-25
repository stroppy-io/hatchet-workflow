// Wrap a scalar id into the proto Id message shape ({ value }) that every
// tenant-scoped ui request expects for tenant_id and bare-id fields.
export function tenantIdMessage(value: string): { value: string } {
  return { value };
}

export function idMessage(value: string | undefined): { value: string } {
  return { value: value ?? "" };
}
