import { useEffect, useState } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { BookOpen, Plus, Activity, MessageSquare } from 'lucide-react'
import { api } from '@/api/client'
import type { Track, Conversation } from '@/api/types'
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
  const [convs, setConvs] = useState<Conversation[]>([])
  const [loading, setLoading] = useState(true)
  const location = useLocation()

  useEffect(() => {
    Promise.all([
      api.listTracks().then(r => setTracks(r.tracks ?? [])),
      api.listConversations().then(r => setConvs(r.conversations ?? [])),
    ]).finally(() => setLoading(false))
  }, [location.pathname])

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
          <SidebarGroupContent>
            <NavLink
              to="/monitoring"
              className={({ isActive }) =>
                `flex items-center gap-2 px-2 py-1.5 text-sm rounded-md transition-colors ${
                  isActive
                    ? 'bg-accent text-accent-foreground font-medium'
                    : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
                }`
              }
            >
              <Activity size={15} className="shrink-0" />
              <span className="group-data-[collapsible=icon]:hidden">Monitoring</span>
            </NavLink>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <div className="flex items-center justify-between pr-2 group-data-[collapsible=icon]:hidden">
            <SidebarGroupLabel>Conversations</SidebarGroupLabel>
            <Link to="/conversations/new" title="New conversation" className="text-muted-foreground hover:text-foreground">
              <Plus size={14} />
            </Link>
          </div>
          <SidebarGroupContent>
            {loading ? (
              <div className="px-2 py-1"><div className="h-5 w-full bg-muted rounded animate-pulse" /></div>
            ) : convs.length === 0 ? (
              <Link
                to="/conversations/new"
                className="flex items-center gap-2 px-2 py-1.5 text-xs text-muted-foreground hover:text-foreground hover:bg-accent/50 rounded-md transition-colors group-data-[collapsible=icon]:hidden"
              >
                <MessageSquare size={13} className="shrink-0" />
                <span>New conversation</span>
              </Link>
            ) : (
              <div className="space-y-px group-data-[collapsible=icon]:hidden">
                {convs.slice(0, 8).map(c => (
                  <NavLink
                    key={c.id}
                    to={`/conversations/${c.id}`}
                    className={({ isActive }) =>
                      `flex items-center gap-2 px-2 py-1 text-xs rounded-md transition-colors truncate ${
                        isActive
                          ? 'bg-accent text-accent-foreground font-medium'
                          : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
                      }`
                    }
                  >
                    <MessageSquare size={11} className="shrink-0" />
                    <span className="truncate">{c.title || c.track_ids.join(', ')}</span>
                  </NavLink>
                ))}
                {convs.length > 8 && (
                  <p className="px-2 text-[10px] text-muted-foreground">+{convs.length - 8} more</p>
                )}
              </div>
            )}
          </SidebarGroupContent>
        </SidebarGroup>

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
