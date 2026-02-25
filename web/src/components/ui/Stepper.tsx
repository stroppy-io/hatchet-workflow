import { cn } from '../../App'
import { motion } from 'framer-motion'
import { Check } from 'lucide-react'

interface StepperProps {
    steps: string[]
    currentStep: number
    onStepClick: (step: number) => void
}

export const Stepper = ({ steps, currentStep, onStepClick }: StepperProps) => {
    return (
        <div className="flex items-center justify-between w-full max-w-2xl mx-auto mb-8 relative">
            {/* Background Line */}
            <div className="absolute top-1/2 left-0 w-full h-0.5 bg-border -z-10" />

            {/* Progress Line */}
            <motion.div
                className="absolute top-1/2 left-0 h-0.5 bg-primary -z-10"
                initial={{ width: "0%" }}
                animate={{ width: `${(currentStep / (steps.length - 1)) * 100}%` }}
                transition={{ type: "spring", stiffness: 300, damping: 30 }}
            />

            {steps.map((step, index) => {
                const isActive = index === currentStep
                const isCompleted = index < currentStep

                return (
                    <div key={index} className="flex flex-col items-center gap-2 cursor-pointer group" onClick={() => onStepClick(index)}>
                        <div className={cn(
                            "w-8 h-8 flex items-center justify-center border-2 transition-all duration-300 z-10 bg-background",
                            isActive ? "border-primary text-primary scale-110 shadow-[0_0_15px_rgba(var(--primary),0.5)]" :
                                isCompleted ? "border-primary bg-primary text-primary-foreground" :
                                    "border-muted-foreground text-muted-foreground group-hover:border-foreground group-hover:text-foreground"
                        )}>
                            {isCompleted ? <Check className="w-4 h-4" /> : <span className="text-xs font-black">{index + 1}</span>}
                        </div>
                        <span className={cn(
                            "text-[10px] uppercase font-black tracking-widest transition-colors absolute -bottom-6 w-32 text-center",
                            isActive ? "text-primary" : isCompleted ? "text-foreground" : "text-muted-foreground"
                        )}>
                            {step}
                        </span>
                    </div>
                )
            })}
        </div>
    )
}
