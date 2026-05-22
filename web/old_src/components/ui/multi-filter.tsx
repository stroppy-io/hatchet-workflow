import { useState } from "react";
import { Check, ChevronDown, X } from "lucide-react";
import { Popover, PopoverContent, PopoverTrigger } from "./popover";
import { cn } from "@/lib/utils";

export interface FilterOption {
  value: string;
  label: string;
  count?: number;
  color?: string;
}

interface MultiFilterProps {
  icon?: React.ReactNode;
  label: string;
  options: FilterOption[];
  selected: Set<string>;
  onChange: (selected: Set<string>) => void;
  className?: string;
}

export function MultiFilter({ icon, label, options, selected, onChange, className }: MultiFilterProps) {
  const [open, setOpen] = useState(false);

  const allSelected = selected.size === 0;
  const summary = allSelected
    ? "All"
    : selected.size === 1
      ? options.find((o) => selected.has(o.value))?.label || "1 selected"
      : `${selected.size} selected`;

  const toggle = (value: string) => {
    const next = new Set(selected);
    if (next.has(value)) {
      next.delete(value);
    } else {
      next.add(value);
    }
    onChange(next);
  };

  const selectAll = () => onChange(new Set());

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className={cn(
            "flex items-center gap-1.5 px-2 py-1 text-[11px] font-mono border border-zinc-800 rounded transition-colors",
            "hover:bg-zinc-900 hover:border-zinc-700",
            !allSelected && "border-primary/40 bg-primary/5",
            className,
          )}
        >
          {icon}
          <span className="text-zinc-500 uppercase text-[9px] font-semibold tracking-wider">{label}</span>
          <span className={cn("truncate max-w-[120px]", allSelected ? "text-zinc-500" : "text-primary")}>
            {summary}
          </span>
          {!allSelected && (
            <X
              className="h-3 w-3 text-zinc-500 hover:text-zinc-300 shrink-0"
              onClick={(e) => {
                e.stopPropagation();
                selectAll();
              }}
            />
          )}
          <ChevronDown className="h-3 w-3 text-zinc-600 shrink-0" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-0 bg-zinc-950 border-zinc-800">
        {/* Select all */}
        <button
          onClick={selectAll}
          className={cn(
            "w-full flex items-center gap-2 px-3 py-1.5 text-[11px] font-mono hover:bg-zinc-900 transition-colors border-b border-zinc-800",
            allSelected && "text-primary",
          )}
        >
          <div className={cn(
            "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
            allSelected && "bg-primary border-primary",
          )}>
            {allSelected && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
          </div>
          All
        </button>

        {/* Options */}
        <div className="max-h-60 overflow-y-auto py-1">
          {options.map((opt) => {
            const isSelected = selected.has(opt.value);
            return (
              <button
                key={opt.value}
                onClick={() => toggle(opt.value)}
                className="w-full flex items-center gap-2 px-3 py-1 text-[11px] font-mono hover:bg-zinc-900 transition-colors"
              >
                <div className={cn(
                  "h-3.5 w-3.5 rounded-sm border border-zinc-700 flex items-center justify-center shrink-0",
                  isSelected && "bg-primary border-primary",
                )}>
                  {isSelected && <Check className="h-2.5 w-2.5 text-primary-foreground" />}
                </div>
                <span className={cn("truncate", opt.color)} title={opt.label}>
                  {opt.label}
                </span>
                {opt.count !== undefined && (
                  <span className="ml-auto text-zinc-600 shrink-0">{opt.count}</span>
                )}
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}
