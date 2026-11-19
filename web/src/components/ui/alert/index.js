import { cva } from 'class-variance-authority'

export { default as Alert } from './Alert.vue'
export { default as AlertTitle } from './AlertTitle.vue'
export { default as AlertDescription } from './AlertDescription.vue'

export const alertVariants = cva(
  'relative grid w-full grid-cols-[0_1fr] items-start gap-y-0.5 rounded-lg border px-4 py-3 text-sm has-[>svg]:grid-cols-[calc(var(--spacing)*4)_1fr] has-[>svg]:gap-x-3 [&>svg]:size-4 [&>svg]:translate-y-0.5',
  {
    variants: {
      variant: {
        default: 'bg-card text-card-foreground',
        destructive: 'border-destructive/25 bg-destructive/8 text-destructive [&>svg]:text-current',
        success: 'border-success/25 bg-success/8 text-success [&>svg]:text-current',
        warning: 'border-warning/25 bg-warning/8 text-warning [&>svg]:text-current'
      }
    },
    defaultVariants: { variant: 'default' }
  }
)
