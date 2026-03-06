import { useStore } from "@nanostores/react"
import yaml from "js-yaml"
import { $testSuite } from "@/stores/editor"

export function YamlPreview() {
  const suite = useStore($testSuite)

  let yamlStr: string
  try {
    yamlStr = yaml.dump(suite, { indent: 2, lineWidth: 120, noRefs: true })
  } catch {
    yamlStr = "# Error serializing test suite"
  }

  return (
    <pre className="h-full overflow-auto p-3 bg-background font-mono text-[12px] text-foreground leading-relaxed select-all">
      {yamlStr}
    </pre>
  )
}
