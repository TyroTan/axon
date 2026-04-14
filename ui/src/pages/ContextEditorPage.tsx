import { useEffect, useRef, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { GetTrackContextResult } from '@/api/types'
import { buttonVariants } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

type SaveStatus = 'idle' | 'saving' | 'saved' | 'error'

export function ContextEditorPage() {
  const { trackId } = useParams<{ trackId: string }>()

  const [data, setData] = useState<GetTrackContextResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)

  // Editor state
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [draft, setDraft] = useState('')
  const [saveStatus, setSaveStatus] = useState<SaveStatus>('idle')
  const [saveError, setSaveError] = useState<string | null>(null)
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // New-file creation
  const [newFilename, setNewFilename] = useState('')
  const [showNewFile, setShowNewFile] = useState(false)

  useEffect(() => {
    if (!trackId) return
    api.getTrackContext(trackId)
      .then(d => {
        setData(d)
        const files = Object.keys(d.files).sort()
        if (files.length > 0) {
          setSelectedFile(files[0])
          setDraft(d.files[files[0]])
        }
      })
      .catch(e => setLoadError(String(e)))
      .finally(() => setLoading(false))
  }, [trackId])

  function selectFile(name: string) {
    setSelectedFile(name)
    setDraft(data?.files[name] ?? '')
    setSaveStatus('idle')
    setSaveError(null)
  }

  async function saveFile() {
    if (!trackId || !selectedFile) return
    setSaveStatus('saving')
    setSaveError(null)
    try {
      await api.updateContextFile(trackId, selectedFile, draft)
      setData(prev => prev ? {
        ...prev,
        files: { ...prev.files, [selectedFile]: draft },
      } : prev)
      setSaveStatus('saved')
      if (saveTimer.current) clearTimeout(saveTimer.current)
      saveTimer.current = setTimeout(() => setSaveStatus('idle'), 2000)
    } catch (e) {
      setSaveStatus('error')
      setSaveError(String(e))
    }
  }

  async function createFile() {
    const name = newFilename.trim()
    if (!name || !trackId) return
    try {
      await api.updateContextFile(trackId, name, '')
      setData(prev => prev ? {
        ...prev,
        files: { ...prev.files, [name]: '' },
      } : prev)
      setNewFilename('')
      setShowNewFile(false)
      setSelectedFile(name)
      setDraft('')
      setSaveStatus('idle')
    } catch (e) {
      setSaveError(String(e))
    }
  }

  if (loading) return (
    <div className="space-y-3 max-w-5xl">
      <Skeleton className="h-7 w-40" />
      <div className="grid grid-cols-[200px_1fr] gap-4">
        <Skeleton className="h-64 rounded-lg" />
        <Skeleton className="h-96 rounded-lg" />
      </div>
    </div>
  )

  if (loadError) return <p className="text-destructive text-sm">{loadError}</p>
  if (!data) return null

  const fileList = Object.keys(data.files).sort()

  return (
    <div className="max-w-5xl space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-bold font-mono">{trackId} / context</h1>
          <p className="text-xs text-muted-foreground mt-0.5">
            Markdown files injected as context for AI prompt generation
          </p>
        </div>
        <Link
          to={`/tracks/${trackId}`}
          className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
        >
          ← Back to track
        </Link>
      </div>

      {/* Editor layout */}
      <div className="grid grid-cols-[200px_1fr] gap-4 items-start">

        {/* File list */}
        <div className="border rounded-lg overflow-hidden">
          <div className="px-3 py-2 border-b bg-muted/40 flex items-center justify-between">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Files</span>
            <button
              onClick={() => setShowNewFile(v => !v)}
              className="text-xs text-primary hover:underline leading-none"
            >
              + New
            </button>
          </div>

          {showNewFile && (
            <div className="px-2 py-2 border-b flex gap-1">
              <input
                autoFocus
                value={newFilename}
                onChange={e => setNewFilename(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') createFile(); if (e.key === 'Escape') setShowNewFile(false) }}
                placeholder="filename.md"
                className="flex-1 text-xs px-2 py-1 rounded border bg-background focus:outline-none focus:ring-1 focus:ring-ring"
              />
              <button
                onClick={createFile}
                className="text-xs px-2 py-1 rounded bg-primary text-primary-foreground hover:bg-primary/90"
              >
                OK
              </button>
            </div>
          )}

          {fileList.length === 0 && !showNewFile && (
            <p className="text-xs text-muted-foreground px-3 py-4">No files yet.</p>
          )}

          <ul>
            {fileList.map(name => (
              <li key={name}>
                <button
                  onClick={() => selectFile(name)}
                  className={cn(
                    'w-full text-left px-3 py-2 text-xs truncate transition-colors',
                    selectedFile === name
                      ? 'bg-accent text-accent-foreground font-medium'
                      : 'hover:bg-muted/50 text-muted-foreground'
                  )}
                >
                  {name}
                </button>
              </li>
            ))}
          </ul>
        </div>

        {/* Editor panel */}
        <div className="border rounded-lg overflow-hidden flex flex-col">
          {selectedFile ? (
            <>
              <div className="px-4 py-2 border-b bg-muted/40 flex items-center justify-between">
                <span className="text-xs font-mono text-muted-foreground">{selectedFile}</span>
                <div className="flex items-center gap-3">
                  {saveStatus === 'saved' && (
                    <span className="text-xs text-green-500">Saved</span>
                  )}
                  {saveStatus === 'error' && (
                    <span className="text-xs text-destructive" title={saveError ?? ''}>Save failed</span>
                  )}
                  <button
                    onClick={saveFile}
                    disabled={saveStatus === 'saving'}
                    className={cn(
                      buttonVariants({ size: 'sm' }),
                      'h-6 text-xs px-3',
                      saveStatus === 'saving' && 'opacity-60 cursor-not-allowed'
                    )}
                  >
                    {saveStatus === 'saving' ? 'Saving…' : 'Save'}
                  </button>
                </div>
              </div>
              <textarea
                value={draft}
                onChange={e => { setDraft(e.target.value); setSaveStatus('idle') }}
                spellCheck={false}
                className="flex-1 resize-none font-mono text-sm p-4 bg-background focus:outline-none min-h-[480px]"
                placeholder="Paste markdown content here…"
              />
            </>
          ) : (
            <div className="flex items-center justify-center h-64 text-sm text-muted-foreground">
              Select a file to edit
            </div>
          )}
        </div>

      </div>
    </div>
  )
}
