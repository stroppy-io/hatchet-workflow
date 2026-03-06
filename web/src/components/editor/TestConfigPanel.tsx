import React, { useState } from "react"
import { useStore } from "@nanostores/react"
import {
  FileText,
  Terminal,
  HardDrive,
  Variable,
  Plus,
  X,
} from "lucide-react"
import { Input } from "@/components/ui/input"
import { Button } from "@/components/ui/button"
import {
  $currentTest,
  $currentTestIndex,
  updateTest,
  type Test,
  type StroppyCli,
  type Hardware,
} from "@/stores/editor"

function FieldGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <label className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
        {label}
      </label>
      {children}
    </div>
  )
}

function SectionCard({
  title,
  icon: Icon,
  children,
}: {
  title: string
  icon: React.ComponentType<{ className?: string }>
  children: React.ReactNode
}) {
  return (
    <div className="border border-border bg-card">
      <div className="flex items-center gap-2 border-b border-border bg-secondary/20 px-3 py-1.5">
        <Icon className="h-3.5 w-3.5 text-primary" />
        <span className="text-[12px] font-medium">{title}</span>
      </div>
      <div className="p-3 space-y-2.5">{children}</div>
    </div>
  )
}

function EnvEditor({
  env,
  onChange,
}: {
  env: Record<string, string>
  onChange: (env: Record<string, string>) => void
}) {
  const [newKey, setNewKey] = useState("")
  const [newVal, setNewVal] = useState("")
  const entries = Object.entries(env)

  function addEntry() {
    if (!newKey.trim()) return
    onChange({ ...env, [newKey.trim()]: newVal })
    setNewKey("")
    setNewVal("")
  }

  function removeEntry(key: string) {
    const next = { ...env }
    delete next[key]
    onChange(next)
  }

  return (
    <div className="space-y-1.5">
      {entries.map(([k, v]) => (
        <div key={k} className="flex items-center gap-1">
          <span className="text-[11px] font-mono text-muted-foreground truncate w-20">{k}</span>
          <span className="text-[11px] text-muted-foreground">=</span>
          <span className="text-[11px] font-mono truncate flex-1">{v}</span>
          <button
            type="button"
            onClick={() => removeEntry(k)}
            className="text-muted-foreground hover:text-destructive p-0.5"
          >
            <X className="h-3 w-3" />
          </button>
        </div>
      ))}
      <div className="flex items-center gap-1">
        <Input
          value={newKey}
          onChange={(e) => setNewKey(e.target.value)}
          placeholder="KEY"
          className="bg-background h-6 text-[11px] font-mono flex-1"
        />
        <Input
          value={newVal}
          onChange={(e) => setNewVal(e.target.value)}
          placeholder="value"
          className="bg-background h-6 text-[11px] font-mono flex-1"
        />
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-6 w-6"
          onClick={addEntry}
        >
          <Plus className="h-3 w-3" />
        </Button>
      </div>
    </div>
  )
}

export function TestConfigPanel() {
  const test = useStore($currentTest)
  const index = useStore($currentTestIndex)

  if (!test) return null

  function patchTest(patch: Partial<Test>) {
    updateTest(index, patch)
  }

  function patchCli(patch: Partial<StroppyCli>) {
    patchTest({ stroppyCli: { ...test.stroppyCli, ...patch } })
  }

  function patchHardware(patch: Partial<Hardware>) {
    patchTest({ stroppyHardware: { ...test.stroppyHardware, ...patch } })
  }

  return (
    <div className="flex flex-col gap-3 p-2 overflow-y-auto h-full">
      <SectionCard title="Test Info" icon={FileText}>
        <FieldGroup label="Name">
          <Input
            value={test.name}
            onChange={(e) => patchTest({ name: e.target.value })}
            className="bg-background h-7 text-[12px]"
          />
        </FieldGroup>
        <FieldGroup label="Description">
          <textarea
            value={test.description}
            onChange={(e) => patchTest({ description: e.target.value })}
            rows={2}
            className="flex w-full border border-input bg-background px-2 py-1 text-[12px] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring resize-none"
          />
        </FieldGroup>
      </SectionCard>

      <SectionCard title="Stroppy CLI" icon={Terminal}>
        <FieldGroup label="Workload">
          <select
            value={test.stroppyCli.workload}
            onChange={(e) => patchCli({ workload: e.target.value as "TPCC" | "TPCB" })}
            className="flex h-7 w-full border border-input bg-background px-2 py-1 text-[12px] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
          >
            <option value="TPCC">TPC-C</option>
            <option value="TPCB">TPC-B</option>
          </select>
        </FieldGroup>
        <div className="grid grid-cols-2 gap-2">
          <FieldGroup label="Scale Factor">
            <Input
              type="number"
              value={test.stroppyCli.scaleFactor}
              onChange={(e) => patchCli({ scaleFactor: parseInt(e.target.value) || 0 })}
              className="bg-background h-7 text-[12px]"
            />
          </FieldGroup>
          <FieldGroup label="Duration">
            <Input
              value={test.stroppyCli.duration}
              onChange={(e) => patchCli({ duration: e.target.value })}
              placeholder="5m"
              className="bg-background h-7 text-[12px]"
            />
          </FieldGroup>
        </div>
        <FieldGroup label="Version">
          <Input
            value={test.stroppyCli.version}
            onChange={(e) => patchCli({ version: e.target.value })}
            className="bg-background h-7 text-[12px]"
          />
        </FieldGroup>
        <FieldGroup label="Binary Path">
          <Input
            value={test.stroppyCli.binaryPath}
            onChange={(e) => patchCli({ binaryPath: e.target.value })}
            className="bg-background h-7 text-[12px] font-mono"
          />
        </FieldGroup>
        <FieldGroup label="Workdir">
          <Input
            value={test.stroppyCli.workdir}
            onChange={(e) => patchCli({ workdir: e.target.value })}
            className="bg-background h-7 text-[12px] font-mono"
          />
        </FieldGroup>
      </SectionCard>

      <SectionCard title="Stroppy Hardware" icon={HardDrive}>
        <div className="grid grid-cols-3 gap-2">
          <FieldGroup label="Cores">
            <Input
              type="number"
              value={test.stroppyHardware.cores}
              onChange={(e) => patchHardware({ cores: parseInt(e.target.value) || 0 })}
              className="bg-background h-7 text-[12px]"
            />
          </FieldGroup>
          <FieldGroup label="Memory (GB)">
            <Input
              type="number"
              value={test.stroppyHardware.memory}
              onChange={(e) => patchHardware({ memory: parseInt(e.target.value) || 0 })}
              className="bg-background h-7 text-[12px]"
            />
          </FieldGroup>
          <FieldGroup label="Disk (GB)">
            <Input
              type="number"
              value={test.stroppyHardware.disk}
              onChange={(e) => patchHardware({ disk: parseInt(e.target.value) || 0 })}
              className="bg-background h-7 text-[12px]"
            />
          </FieldGroup>
        </div>
      </SectionCard>

      <SectionCard title="Environment" icon={Variable}>
        <EnvEditor
          env={test.stroppyCli.stroppyEnv}
          onChange={(stroppyEnv) => patchCli({ stroppyEnv })}
        />
      </SectionCard>
    </div>
  )
}
