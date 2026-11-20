import { cva } from 'class-variance-authority'

export { default as Button } from './Button.vue'

export const buttonVariants = cva(
  'inline-flex shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-md text-sm font-medium transition-[color,background-color,border-color,box-shadow,transform] outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/30 disabled:pointer-events-none disabled:opacity-45 active:scale-[0.97] aria-invalid:border-destructive aria-invalid:ring-destructive/20 [&_svg]:pointer-events-none [&_svg:not([class*="size-"])]:size-4 [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default: 'border border-primary bg-primary text-primary-foreground shadow-none hover:bg-primary/88',
        destructive: 'border border-destructive bg-destructive text-destructive-foreground shadow-none hover:bg-destructive/88 focus-visible:ring-destructive/20',
        outline: 'border border-input bg-card text-foreground shadow-none hover:border-foreground/20 hover:bg-accent hover:text-accent-foreground',
        secondary: 'border border-transparent bg-secondary text-secondary-foreground shadow-none hover:bg-secondary/75',
        ghost: 'border border-transparent hover:bg-accent hover:text-accent-foreground',
        link: 'text-foreground underline-offset-4 hover:underline'
      },
      size: {
        default: 'h-9 px-4 py-2 has-[>svg]:px-3',
        sm: 'h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5',
        lg: 'h-10 rounded-md px-6 has-[>svg]:px-4',
        icon: 'size-9',
        'icon-sm': 'size-8',
        'icon-lg': 'size-10'
      }
    },
    defaultVariants: { variant: 'default', size: 'default' }
  }
)
