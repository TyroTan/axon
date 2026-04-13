import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { BookOpen, Plus } from 'lucide-react'
import { api } from '@/api/client'
import type { Track } from '@/api/types'
import { TrackTree } from './TrackTree'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarRail,
} from '@/components/ui/sidebar'
import { Skeleton } from '@/components/ui/skeleton'

export function AppSidebar() {
  const [tracks, setTracks] = useState<Track[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.listTracks()
      .then(r => setTracks(r.tracks ?? []))
      .finally(() => setLoading(false))
  }, [])

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <div className="flex items-center gap-2 px-2 py-1">
          <BookOpen size={18} className="text-primary shrink-0" />
          <span className="font-semibold text-sm tracking-wide group-data-[collapsible=icon]:hidden">Axon</span>
        </div>
      </SidebarHeader>

      <SidebarContent>
        <SidebarGroup>
          <div className="flex items-center justify-between pr-2 group-data-[collapsible=icon]:hidden">
            <SidebarGroupLabel>Tracks</SidebarGroupLabel>
            <Link to="/tracks/new" title="New track" className="text-muted-foreground hover:text-foreground">
              <Plus size={14} />
            </Link>
          </div>
          <SidebarGroupContent>
            {loading ? (
              <div className="space-y-1 px-2 py-1">
                {[1, 2, 3].map(i => <Skeleton key={i} className="h-7 w-full rounded" />)}
              </div>
            ) : (
              <TrackTree tracks={tracks} />
            )}
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter>
        <div className="px-3 py-2 text-xs text-muted-foreground group-data-[collapsible=icon]:hidden">
          Local · file-persisted
        </div>
      </SidebarFooter>

      <SidebarRail />
    </Sidebar>
  )
}
