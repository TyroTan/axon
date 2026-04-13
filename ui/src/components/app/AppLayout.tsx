import { Outlet, useLocation, useParams } from 'react-router-dom'
import { AppSidebar } from './AppSidebar'
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from '@/components/ui/sidebar'
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Separator } from '@/components/ui/separator'
import { TooltipProvider } from '@/components/ui/tooltip'

interface Crumb {
  label: string
  to?: string
}

function useBreadcrumbs(): Crumb[] {
  const location = useLocation()
  const { trackId, sessionId } = useParams<{ trackId?: string; sessionId?: string }>()
  const crumbs: Crumb[] = [{ label: 'Home', to: '/' }]

  if (trackId) {
    crumbs.push({ label: trackId, to: sessionId ? `/tracks/${trackId}` : undefined })
  }
  if (sessionId) {
    crumbs.push({ label: `Session ${sessionId}` })
  }

  // New track / new session
  if (location.pathname.endsWith('/new') && !sessionId) {
    crumbs.push({ label: 'New Track' })
  }

  return crumbs
}

export function AppLayout() {
  const crumbs = useBreadcrumbs()

  return (
    <TooltipProvider>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>
          {/* Topbar */}
          <header className="flex h-12 shrink-0 items-center gap-2 border-b px-4">
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-2 h-4" />
            <Breadcrumb>
              <BreadcrumbList>
                {crumbs.map((crumb, i) => (
                  <span key={i} className="flex items-center gap-1.5">
                    {i > 0 && <BreadcrumbSeparator />}
                    <BreadcrumbItem>
                      {crumb.to ? (
                        <BreadcrumbLink href={crumb.to}>
                          {crumb.label}
                        </BreadcrumbLink>
                      ) : (
                        <BreadcrumbPage>{crumb.label}</BreadcrumbPage>
                      )}
                    </BreadcrumbItem>
                  </span>
                ))}
              </BreadcrumbList>
            </Breadcrumb>
          </header>

          {/* Page content */}
          <div className="flex flex-1 flex-col gap-4 p-6 overflow-y-auto">
            <Outlet />
          </div>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}
