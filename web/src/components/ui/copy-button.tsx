import { useState } from "react";
import { Check, Copy } from "lucide-react";

import { Button } from "@/components/ui/button";
import { notifyError, notifySuccess } from "@/lib/toast";

type CopyButtonProps = {
  value: string;
  label?: string;
  toastLabel?: string;
  className?: string;
};

// Copy-to-clipboard with a transient check icon + toast confirmation.
// Used for ids, share links, and one-time plaintext tokens.
export function CopyButton({ value, label, toastLabel = "Copied", className }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      notifySuccess(toastLabel);
      setTimeout(() => setCopied(false), 1500);
    } catch (error) {
      notifyError(error, "Could not copy");
    }
  };

  return (
    <Button
      type="button"
      variant="ghost"
      size={label ? "sm" : "icon-sm"}
      className={className}
      onClick={copy}
    >
      {copied ? <Check className="size-3.5" /> : <Copy className="size-3.5" />}
      {label}
    </Button>
  );
}
