import React, { useState } from "react"
import { useNavigate } from "react-router"
import { motion } from "framer-motion"
import { Loader2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { login } from "@/stores/auth"

export function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [loading, setLoading] = useState(false)

  async function handleSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError("")
    setLoading(true)

    try {
      await login(username, password)
      navigate("/")
    } catch {
      setError("Invalid credentials")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex h-screen w-screen items-center justify-center bg-background relative overflow-hidden">
      {/* Subtle grid pattern background */}
      <div
        className="absolute inset-0 opacity-[0.03]"
        style={{
          backgroundImage: `linear-gradient(var(--foreground) 1px, transparent 1px), linear-gradient(90deg, var(--foreground) 1px, transparent 1px)`,
          backgroundSize: "40px 40px",
        }}
      />

      {/* Diagonal accent line */}
      <div
        className="absolute top-0 left-0 w-full h-full pointer-events-none"
        style={{
          background: "linear-gradient(135deg, transparent 48%, var(--primary) 49%, var(--primary) 49.5%, transparent 50%)",
          opacity: 0.06,
        }}
      />

      <motion.div
        initial={{ opacity: 0, y: 12 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: [0.25, 0.46, 0.45, 0.94] }}
        className="relative z-10 w-[380px]"
      >
        {/* Welcome header — like IntelliJ's welcome screen */}
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.1, duration: 0.5 }}
          className="mb-8 text-center"
        >
          {/* Logo mark */}
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center border border-border bg-card">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" className="text-primary">
              <path d="M12 2L2 7v10l10 5 10-5V7L12 2z" stroke="currentColor" strokeWidth="1.5" fill="none" />
              <path d="M12 12L2 7M12 12l10-5M12 12v10" stroke="currentColor" strokeWidth="1.5" />
              <circle cx="12" cy="12" r="2" fill="currentColor" opacity="0.6" />
            </svg>
          </div>

          <h1 className="text-[18px] font-semibold tracking-tight text-foreground">
            Stroppy Cloud
          </h1>
          <p className="mt-1 text-[12px] text-muted-foreground">
            Database performance testing platform
          </p>
        </motion.div>

        {/* Login card */}
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, duration: 0.4 }}
          className="border border-border bg-card"
        >
          {/* Tab-like header */}
          <div className="flex items-center border-b border-border bg-secondary/30 px-3 h-8">
            <span className="text-[12px] font-medium text-foreground border-b-2 border-primary -mb-px pb-1.5 pt-1.5">
              Sign In
            </span>
          </div>

          <form onSubmit={handleSubmit} className="p-4 space-y-3">
            <div className="space-y-1">
              <label className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
                Username
              </label>
              <Input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin"
                autoFocus
                className="bg-background"
              />
            </div>

            <div className="space-y-1">
              <label className="text-[11px] font-medium text-muted-foreground uppercase tracking-wider">
                Password
              </label>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Enter password"
                className="bg-background"
              />
            </div>

            {error && (
              <motion.div
                initial={{ opacity: 0, height: 0 }}
                animate={{ opacity: 1, height: "auto" }}
                className="flex items-center gap-2 border border-destructive/30 bg-destructive/5 px-2.5 py-1.5"
              >
                <div className="h-1.5 w-1.5 rounded-full bg-destructive shrink-0" />
                <p className="text-[12px] text-destructive">{error}</p>
              </motion.div>
            )}

            <div className="pt-1">
              <Button type="submit" className="w-full" disabled={loading}>
                {loading && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                Sign in
              </Button>
            </div>
          </form>
        </motion.div>

        {/* Version footer */}
        <motion.p
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.5, duration: 0.5 }}
          className="mt-4 text-center text-[11px] text-muted-foreground/60 font-mono"
        >
          v0.1.0
        </motion.p>
      </motion.div>
    </div>
  )
}
