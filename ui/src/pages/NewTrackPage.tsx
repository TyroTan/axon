import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { X } from 'lucide-react'

// NewTrackPage — creates a new root track (track_2, track_3, …).
// Root tracks are topic clusters with no parent. You define the major branches here
// and the system allocates the ID automatically.
//
// To create a CHILD track (track_1_2, track_1_2_3, …) — go to the parent track
// and use the Duplicate button. Child tracks inherit the parent's context and concept map.

export function NewTrackPage() {
  const [branches, setBranches] = useState<string[]>([''])
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  function addBranch() {
    setBranches(b => [...b, ''])
  }

  function updateBranch(i: number, val: string) {
    setBranches(b => b.map((v, idx) => idx === i ? val : v))
  }

  function removeBranch(i: number) {
    setBranches(b => b.filter((_, idx) => idx !== i))
  }

  async function handleCreate() {
    const valid = branches.map(b => b.trim()).filter(Boolean)
    if (valid.length === 0) {
      setError('Add at least one branch name.')
      return
    }
    setCreating(true)
    setError(null)
    try {
      const res = await api.createTrack(valid)
      navigate(`/tracks/${res.new_track_id}`)
    } catch (e) {
      setError(String(e))
      setCreating(false)
    }
  }

  const validCount = branches.filter(b => b.trim()).length

  return (
    <div className="max-w-lg space-y-6 mt-8">
      <div>
        <h1 className="text-xl font-bold mb-1">New Root Track</h1>
        <p className="text-sm text-muted-foreground">
          A root track is a new topic cluster with no parent (e.g. <code className="bg-muted px-1 rounded text-xs">track_2</code>).
          Define 2–5 major knowledge branches below — these become the columns of your concept map.
        </p>
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Major Branches</CardTitle>
          <CardDescription>
            Each branch covers a body of knowledge. Examples: "RAG Architecture", "LLM Systems",
            "ML Fundamentals". 3–5 branches recommended.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {branches.map((b, i) => (
            <div key={i} className="flex gap-2 items-center">
              <input
                type="text"
                value={b}
                onChange={e => updateBranch(i, e.target.value)}
                onKeyDown={e => e.key === 'Enter' && addBranch()}
                placeholder={`Branch ${i + 1} — e.g. "RAG Architecture"`}
                className="flex-1 text-sm bg-muted border border-border rounded px-3 py-1.5 focus:outline-none focus:ring-1 focus:ring-primary"
              />
              {branches.length > 1 && (
                <button
                  onClick={() => removeBranch(i)}
                  className="text-muted-foreground hover:text-foreground"
                >
                  <X size={14} />
                </button>
              )}
            </div>
          ))}
          <button
            onClick={addBranch}
            className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'text-xs mt-1')}
          >
            + Add branch
          </button>
        </CardContent>
      </Card>

      {error && <p className="text-sm text-red-500">{error}</p>}

      <button
        onClick={handleCreate}
        disabled={creating || validCount === 0}
        className={cn(
          buttonVariants(),
          (creating || validCount === 0) && 'opacity-60 cursor-not-allowed',
        )}
      >
        {creating ? 'Creating…' : `Create track with ${validCount} branch${validCount !== 1 ? 'es' : ''}`}
      </button>

      <Card className="border-dashed">
        <CardContent className="pt-4 pb-4">
          <p className="text-sm font-medium mb-1">Creating a child track instead?</p>
          <p className="text-sm text-muted-foreground">
            To create <code className="bg-muted px-1 rounded text-xs">track_1_2</code> (a child of{' '}
            <code className="bg-muted px-1 rounded text-xs">track_1</code>), open{' '}
            <code className="bg-muted px-1 rounded text-xs">track_1</code> and click{' '}
            <strong>Duplicate</strong>. Child tracks inherit the parent's context files and concept map.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}
