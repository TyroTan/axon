import { Link } from 'react-router-dom'
import { Card, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

export function HomePage() {
  return (
    <div className="max-w-lg mt-16">
      <h1 className="text-2xl font-bold mb-2">Welcome to Axon</h1>
      <p className="text-muted-foreground mb-8">
        Select a track from the sidebar to begin, or create a new one.
      </p>
      <div className="flex flex-col gap-3">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">What is a track?</CardTitle>
            <CardDescription>
              A track covers 3–5 major knowledge branches (e.g. "RAG Architecture · LLM Systems").
              Each track has a concept map with a prerequisite graph and adaptive quiz sessions.
            </CardDescription>
          </CardHeader>
        </Card>
        <Link to="/tracks/new" className={cn(buttonVariants(), 'w-fit')}>
          + New Track
        </Link>
      </div>
    </div>
  )
}
