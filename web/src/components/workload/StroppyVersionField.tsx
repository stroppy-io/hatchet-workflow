// Shared stroppy-version picker for the preset forms (Workload + Test presets).
//
// domain.Workload.stroppy_version is a real, persisted preset field (release tag
// like "5.1.2", or "commit:<sha7>"). It also GATES script probing: the workload
// editor only probes when a concrete version is set. A blind free-text input
// left the field empty by default, which silently disabled probing and read as
// "the form is broken". This component fixes that:
//
//   - it is a PICKER (release dropdown + commit input), not free text;
//   - on a fresh preset it auto-selects the latest release so probing works on
//     open and the version is saved with the preset (autofillLatest);
//   - "Defer to run" stays available for authors who want an unpinned preset.
//
// Mirrors NewRun's WorkloadVersionPane semantics in a compact, form-friendly
// shape, and reuses the same StroppyProvider + commit helpers.

import { useEffect, useRef, useState } from "react";
import { AlertCircle, GitCommit, Loader2, Tag } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  commitSha,
  commitVersion,
  getStroppyProvider,
  isCommitVersion,
} from "@/services/stroppy";

// Radix Select forbids an empty-string item value, so the "unpinned" choice
// rides a sentinel that maps back to "" on the way out.
const DEFER = "__defer__";

export function StroppyVersionField({
  slug,
  value,
  onChange,
  disabled,
  autofillLatest = false,
}: {
  slug: string;
  value: string;
  onChange: (version: string) => void;
  disabled?: boolean;
  /** When true (create mode), default an empty version to the latest release
   *  once the list loads — so probing works without the author typing one. */
  autofillLatest?: boolean;
}) {
  const [versions, setVersions] = useState<string[] | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const commit = isCommitVersion(value);
  const [mode, setMode] = useState<"release" | "commit">(commit ? "commit" : "release");
  const autofilled = useRef(false);

  useEffect(() => {
    let cancelled = false;
    setVersions(null);
    setErr(null);
    getStroppyProvider()
      .listStroppyVersions(slug)
      .then((v) => !cancelled && setVersions(v))
      .catch((e) => !cancelled && setErr(e instanceof Error ? e.message : String(e)));
    return () => {
      cancelled = true;
    };
  }, [slug]);

  // Fresh presets arrive unpinned (version ""), which would silently disable
  // probing. Default to the latest release once the list loads. Runs once,
  // create-mode only (autofillLatest), and never clobbers an existing version.
  useEffect(() => {
    if (!autofillLatest || autofilled.current) return;
    if (value.trim() || !versions || versions.length === 0) return;
    autofilled.current = true;
    onChange(versions[0]);
  }, [autofillLatest, value, versions, onChange]);

  return (
    <div className="max-w-xs">
      <Label>Stroppy version</Label>

      {/* release | commit toggle */}
      <div className="mb-2 mt-1 inline-flex border border-zinc-800 text-[11px] font-mono">
        <button
          type="button"
          disabled={disabled}
          onClick={() => {
            setMode("release");
            // Leaving commit mode: fall back to the latest release if known,
            // else unpinned — never strand a commit value behind a hidden input.
            if (commit) onChange(versions?.[0] ?? "");
          }}
          className={`flex items-center gap-1.5 px-3 py-1 transition-colors disabled:opacity-60 ${
            mode === "release" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"
          }`}
        >
          <Tag className="h-3 w-3" /> release
        </button>
        <button
          type="button"
          disabled={disabled}
          onClick={() => setMode("commit")}
          className={`flex items-center gap-1.5 border-l border-zinc-800 px-3 py-1 transition-colors disabled:opacity-60 ${
            mode === "commit" ? "bg-primary/[0.08] text-primary" : "text-zinc-500 hover:text-zinc-300"
          }`}
        >
          <GitCommit className="h-3 w-3" /> commit
        </button>
      </div>

      {mode === "release" ? (
        <>
          {err && (
            <div className="flex items-center gap-2 border border-red-900/50 bg-red-950/30 px-3 py-2 text-xs text-red-400">
              <AlertCircle className="h-3.5 w-3.5 shrink-0" /> {err}
            </div>
          )}
          {!versions && !err && (
            <div className="flex items-center gap-2 px-1 py-2 text-xs text-zinc-600">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> Loading versions…
            </div>
          )}
          {versions && (
            <Select
              value={value.trim() ? value : DEFER}
              disabled={disabled}
              onValueChange={(v) => onChange(v === DEFER ? "" : v)}
            >
              <SelectTrigger className="font-mono text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={DEFER}>Defer to run (latest at launch)</SelectItem>
                {versions.map((v) => (
                  <SelectItem key={v} value={v}>
                    {v}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </>
      ) : (
        <Input
          className="font-mono text-xs"
          value={commitSha(value)}
          spellCheck={false}
          disabled={disabled}
          placeholder="short SHA (7+ hex)"
          onChange={(e) => onChange(commitVersion(e.target.value))}
        />
      )}

      <p className="mt-1 text-[11px] text-zinc-600">
        Pins the stroppy build this workload targets and enables script probing.
        Pick “Defer to run” to leave it unpinned.
      </p>
    </div>
  );
}
