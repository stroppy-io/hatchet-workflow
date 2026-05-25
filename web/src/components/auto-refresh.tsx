import { useEffect, useState } from "react";
import { RefreshCw } from "lucide-react";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

const OPTIONS = [
  { label: "No auto-refresh", value: "0" },
  { label: "Every 3s", value: "3000" },
  { label: "Every 5s", value: "5000" },
  { label: "Every 10s", value: "10000" },
  { label: "Every 30s", value: "30000" },
];

// Interval picker that calls onRefresh on a timer. onRefresh must be stable
// (useCallback) or the interval resets each render. Defaults to 5s.
export function AutoRefresh({ onRefresh }: { onRefresh: () => void }) {
  const [interval, setIntervalValue] = useState("5000");

  useEffect(() => {
    const ms = Number(interval);
    if (!ms) return;
    const timer = setInterval(onRefresh, ms);
    return () => clearInterval(timer);
  }, [interval, onRefresh]);

  return (
    <div className="flex items-center gap-1.5">
      <RefreshCw className="size-3.5 text-muted-foreground" />
      <Select value={interval} onValueChange={setIntervalValue}>
        <SelectTrigger className="h-9 w-40">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
