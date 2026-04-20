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

  // Compaction
  const [compacting, setCompacting] = useState(false)
  const [compactStream, setCompactStream] = useState('')
  const [compactError, setCompactError] = useState<string | null>(null)

  // Import Job
  const [jobPanelOpen, setJobPanelOpen] = useState(false)
  const [jobRoleLabel, setJobRoleLabel] = useState('')
  const [jobText, setJobText] = useState('')
  const [importingJob, setImportingJob] = useState(false)
  const [importJobStream, setImportJobStream] = useState('')
  const [importJobDone, setImportJobDone] = useState<string | null>(null) // filename written
  const [importJobError, setImportJobError] = useState<string | null>(null)

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
    const content = data?.files[name] ?? data?.inherited_files?.[name] ?? ''
    setDraft(content)
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

  async function compactFile() {
    if (!trackId || !selectedFile || compacting) return
    // Skip _-prefixed system files and already-compacted files.
    if (selectedFile.startsWith('_') || selectedFile.includes('.compact.')) return
    setCompacting(true)
    setCompactStream('')
    setCompactError(null)
    try {
      await api.compactContextFile(trackId, selectedFile, t => setCompactStream(p => p + t))
      // Reload context so the new .compact.md appears in the file list.
      const fresh = await api.getTrackContext(trackId)
      setData(fresh)
    } catch (e) {
      setCompactError(String(e))
    } finally {
      setCompacting(false)
    }
  }

  async function importJob() {
    if (!trackId || importingJob || !jobText.trim()) return
    setImportingJob(true)
    setImportJobStream('')
    setImportJobDone(null)
    setImportJobError(null)
    try {
      const filename = await api.importJob(trackId, jobRoleLabel, jobText, t => setImportJobStream(p => p + t))
      setImportJobDone(filename)
      // Reload context so new .job.md appears in the file list.
      const fresh = await api.getTrackContext(trackId)
      setData(fresh)
      setJobText('')
      setJobRoleLabel('')
    } catch (e) {
      setImportJobError(String(e))
    } finally {
      setImportingJob(false)
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
  const inheritedList = Object.keys(data.inherited_files ?? {}).sort()
  const isInherited = selectedFile != null && !(selectedFile in data.files)

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

      {/* Import Job Description panel */}
      <div className="border rounded-lg overflow-hidden">
        <button
          onClick={() => setJobPanelOpen(v => !v)}
          className="w-full flex items-center justify-between px-4 py-2.5 bg-muted/40 hover:bg-muted/60 transition-colors text-sm font-medium"
        >
          <span>Import Job Description</span>
          <span className="text-muted-foreground text-xs">{jobPanelOpen ? '▲ collapse' : '▼ expand'}</span>
        </button>
        {jobPanelOpen && (
          <div className="p-4 space-y-3">
            <p className="text-xs text-muted-foreground">
              Paste a raw job description. The AI will extract a structured interview prep file
              ({' '}<code className="bg-muted px-1 rounded">role.job.md</code>) and write it to this track's context.
              Question generation will automatically use it to adapt framing, scenarios, and distractors.
            </p>
            <div className="flex gap-2 items-center">
              <label className="text-xs text-muted-foreground shrink-0">Role label (optional):</label>
              <input
                type="text"
                value={jobRoleLabel}
                onChange={e => setJobRoleLabel(e.target.value)}
                placeholder="e.g. ragflow_expert"
                className="flex-1 text-xs px-2 py-1.5 rounded border border-border bg-background font-mono focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>
            <textarea
              value={jobText}
              onChange={e => setJobText(e.target.value)}
              placeholder="Paste job description here…"
              rows={10}
              className="w-full text-sm font-mono p-3 rounded border border-border bg-background focus:outline-none focus:ring-1 focus:ring-primary resize-y"
            />
            {importJobError && <p className="text-xs text-destructive">{importJobError}</p>}
            {importJobDone && (
              <p className="text-xs text-green-400">
                Written to <code className="bg-muted px-1 rounded">{importJobDone}</code> — visible in file list.
              </p>
            )}
            {importJobStream && (
              <pre className="text-xs text-muted-foreground bg-muted rounded p-3 max-h-48 overflow-auto whitespace-pre-wrap">{importJobStream}</pre>
            )}
            <button
              onClick={importJob}
              disabled={importingJob || !jobText.trim()}
              className={cn(
                'px-4 py-1.5 rounded text-sm font-medium bg-primary text-primary-foreground hover:bg-primary/90 transition-colors',
                (importingJob || !jobText.trim()) && 'opacity-60 cursor-not-allowed'
              )}
            >
              {importingJob ? 'Analyzing…' : 'Import & Analyze'}
            </button>
          </div>
        )}
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

          {inheritedList.length > 0 && (
            <>
              <div className="px-3 py-1.5 border-t bg-muted/20 flex items-center gap-1">
                <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">Inherited</span>
              </div>
              <ul>
                {inheritedList.map(name => (
                  <li key={name}>
                    <button
                      onClick={() => selectFile(name)}
                      className={cn(
                        'w-full text-left px-3 py-2 text-xs truncate transition-colors',
                        selectedFile === name
                          ? 'bg-accent text-accent-foreground font-medium'
                          : 'hover:bg-muted/50 text-muted-foreground/60'
                      )}
                    >
                      <span className="truncate">{name}</span>
                    </button>
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>

        {/* Editor panel */}
        <div className="border rounded-lg overflow-hidden flex flex-col">
          {selectedFile ? (
            <>
              <div className="px-4 py-2 border-b bg-muted/40 flex items-center justify-between">
                <span className="text-xs font-mono text-muted-foreground">{selectedFile}</span>
                <div className="flex items-center gap-3">
                  {isInherited ? (
                    <span className="text-xs text-muted-foreground/60 italic">
                      from {data.inherited_from?.[selectedFile]} · read-only
                    </span>
                  ) : (
                    <>
                      {saveStatus === 'saved' && <span className="text-xs text-green-500">Saved</span>}
                      {saveStatus === 'error' && <span className="text-xs text-destructive" title={saveError ?? ''}>Save failed</span>}
                      {compactError && <span className="text-xs text-destructive" title={compactError}>Compact failed</span>}
                      {!selectedFile.startsWith('_') && !selectedFile.includes('.compact.') && (
                        <button
                          onClick={compactFile}
                          disabled={compacting}
                          title="Distil this file to ~50% tokens via LLM — writes a .compact.md version"
                          className={cn(
                            buttonVariants({ variant: 'outline', size: 'sm' }),
                            'h-6 text-xs px-3',
                            compacting && 'opacity-60 cursor-not-allowed'
                          )}
                        >
                          {compacting ? 'Compacting…' : 'Compact'}
                        </button>
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
                    </>
                  )}
                </div>
              </div>
              {compactStream && (
                <pre className="text-xs text-muted-foreground bg-muted/60 px-4 py-2 max-h-24 overflow-auto whitespace-pre-wrap border-b">
                  {compactStream}
                </pre>
              )}
              <textarea
                value={draft}
                onChange={e => { if (!isInherited) { setDraft(e.target.value); setSaveStatus('idle') } }}
                readOnly={isInherited}
                spellCheck={false}
                className={cn(
                  'flex-1 resize-none font-mono text-sm p-4 bg-background focus:outline-none min-h-[480px]',
                  isInherited && 'text-muted-foreground/70 cursor-default'
                )}
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
