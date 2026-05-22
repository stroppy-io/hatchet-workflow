import { Panel } from "@/components/panel";

export function PlaceholderPage({ title }: { title: string }) {
  return (
    <div className="p-5">
      <Panel title={title} description="This section is reserved in the RBAC-aware navigation scope.">
        <div className="border border-dashed p-6 text-sm text-muted-foreground">Implementation will follow the feature slice for this section.</div>
      </Panel>
    </div>
  );
}
