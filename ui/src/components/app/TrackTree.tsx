import { ChevronRight } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import type { Track } from '@/api/types'
import {
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
  SidebarMenuSub,
  SidebarMenuSubItem,
  SidebarMenuSubButton,
} from '@/components/ui/sidebar'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'

function branchInitials(branches: string[]): string {
  return branches
    .map(b => b.split(' ').map(w => w[0]?.toUpperCase() ?? '').join(''))
    .join('·')
}

export function TrackTree({ tracks }: { tracks: Track[] }) {
  const { trackId } = useParams<{ trackId: string }>()

  return (
    <SidebarMenu>
      {tracks.map(track => {
        const isActive = track.id === trackId
        const children = track.children ?? []
        const childActive = children.some(c => c.id === trackId)

        if (children.length > 0) {
          return (
            <SidebarMenuItem key={track.id}>
              <Collapsible defaultOpen={isActive || childActive}>
                <div className="flex items-center w-full">
                  <SidebarMenuButton isActive={isActive} className="flex-1 min-w-0">
                    <Link to={`/tracks/${track.id}`} className="flex items-center gap-2 min-w-0 w-full">
                      <span className="font-mono text-xs truncate">{track.id}</span>
                      <span className="text-muted-foreground text-xs ml-auto shrink-0">
                        {branchInitials(track.major_branches ?? [])}
                      </span>
                    </Link>
                  </SidebarMenuButton>
                  <CollapsibleTrigger className="p-1 hover:bg-muted rounded shrink-0">
                    <ChevronRight size={12} className="transition-transform duration-200 data-[state=open]:rotate-90" />
                  </CollapsibleTrigger>
                </div>
                <CollapsibleContent>
                  <SidebarMenuSub>
                    {children.map(child => (
                      <SidebarMenuSubItem key={child.id}>
                        <SidebarMenuSubButton isActive={child.id === trackId}>
                          <Link to={`/tracks/${child.id}`} className="font-mono text-xs">
                            {child.id}
                          </Link>
                        </SidebarMenuSubButton>
                      </SidebarMenuSubItem>
                    ))}
                  </SidebarMenuSub>
                </CollapsibleContent>
              </Collapsible>
            </SidebarMenuItem>
          )
        }

        return (
          <SidebarMenuItem key={track.id}>
            <SidebarMenuButton isActive={isActive}>
              <Link to={`/tracks/${track.id}`} className="flex items-center gap-2 min-w-0 w-full">
                <span className="font-mono text-xs truncate">{track.id}</span>
                <span className="text-muted-foreground text-xs ml-auto shrink-0">
                  {branchInitials(track.major_branches ?? [])}
                </span>
              </Link>
            </SidebarMenuButton>
          </SidebarMenuItem>
        )
      })}
    </SidebarMenu>
  )
}
