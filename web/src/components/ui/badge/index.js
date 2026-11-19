import { cva } from 'class-variance-authority'

export { default as Badge } from './Badge.vue'

export const badgeVariants = cva(
  'inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden rounded-full border px-2 py-0.5 text-[0.68rem] font-medium whitespace-nowrap transition-colors focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 [&>svg]:pointer-events-none [&>svg]:size-3',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-transparent bg-secondary text-secondary-foreground',
        destructive: 'border-destructive/20 bg-destructive/10 text-destructive',
        outline: 'border-border bg-transparent text-foreground',
        success: 'border-success/20 bg-success/10 text-success',
        warning: 'border-warning/20 bg-warning/10 text-warning'
      }
    },
    defaultVariants: { variant: 'default' }
  }
)
