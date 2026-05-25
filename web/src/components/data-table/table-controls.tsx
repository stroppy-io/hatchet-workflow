import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

// Toolbar faceted-filter dropdown for table pages (server-backed typed filters).
export function FilterSelect({
  onChange,
  options,
  value,
}: {
  onChange: (value: string) => void;
  options: { label: string; value: string }[];
  value: string;
}) {
  return (
    <Select onValueChange={onChange} value={value}>
      <SelectTrigger className="h-9 w-44">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

// Neutral badge for table cell values (origin, role, enabled, …).
export function DataBadge({ children }: { children: string }) {
  return <Badge variant="secondary">{children}</Badge>;
}
