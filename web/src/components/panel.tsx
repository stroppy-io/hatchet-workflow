import type { ReactNode } from "react";

type PanelProps = {
  title: string;
  description?: string;
  children: ReactNode;
  action?: ReactNode;
};

export function Panel({ title, description, children, action }: PanelProps) {
  return (
    <section className="border bg-card">
      <div className="flex items-start justify-between gap-4 border-b px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold">{title}</h2>
          {description ? <p className="mt-1 text-xs text-muted-foreground">{description}</p> : null}
        </div>
        {action}
      </div>
      <div className="p-4">{children}</div>
    </section>
  );
}
