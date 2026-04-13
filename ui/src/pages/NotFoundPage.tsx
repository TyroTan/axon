import { Link } from 'react-router-dom'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function NotFoundPage() {
  return (
    <div className="mt-24 text-center">
      <p className="text-4xl font-bold mb-4">404</p>
      <p className="text-muted-foreground mb-6">Page not found.</p>
      <Link to="/" className={cn(buttonVariants({ variant: 'outline' }))}>
        Go home
      </Link>
    </div>
  )
}
