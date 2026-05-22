type JsonPanelProps = {
  value: unknown;
  emptyLabel?: string;
};

export function JsonPanel({ value, emptyLabel = "No data yet" }: JsonPanelProps) {
  if (value === undefined || value === null) {
    return <div className="border border-dashed p-4 text-sm text-muted-foreground">{emptyLabel}</div>;
  }

  return (
    <pre className="max-h-[28rem] overflow-auto bg-[#050506] p-3 font-mono text-xs leading-5 text-zinc-200">
      {JSON.stringify(value, null, 2)}
    </pre>
  );
}
