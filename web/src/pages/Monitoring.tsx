import { motion } from "framer-motion"

const GRAFANA_BASE_URL = window.location.protocol + "//" + window.location.hostname + ":3000"

export function MonitoringPage() {
  const grafanaUrl = `${GRAFANA_BASE_URL}/d/stroppy-overview?orgId=1&kiosk&theme=dark&var-node=All`

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.2 }}
      className="flex flex-col h-full"
    >
      <div className="flex items-center justify-between border-b border-border bg-secondary/20 px-4 py-2">
        <h1 className="text-[14px] font-semibold">Monitoring</h1>
        <a
          href={`${GRAFANA_BASE_URL}`}
          target="_blank"
          rel="noopener noreferrer"
          className="text-[11px] text-primary hover:underline"
        >
          Open Grafana
        </a>
      </div>
      <iframe
        src={grafanaUrl}
        className="flex-1 w-full border-0"
        title="Grafana Monitoring"
        allow="fullscreen"
      />
    </motion.div>
  )
}
