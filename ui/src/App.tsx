import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppLayout } from '@/components/app/AppLayout'
import { HomePage } from '@/pages/HomePage'
import { TrackPage } from '@/pages/TrackPage'
import { ContextEditorPage } from '@/pages/ContextEditorPage'
import { SessionPage } from '@/pages/SessionPage'
import { MonitoringPage } from '@/pages/MonitoringPage'
import { ConversationPage, NewConversationPage } from '@/pages/ConversationPage'
import { NewTrackPage } from '@/pages/NewTrackPage'
import { NotFoundPage } from '@/pages/NotFoundPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<HomePage />} />
          <Route path="monitoring" element={<MonitoringPage />} />
          <Route path="tracks/new" element={<NewTrackPage />} />
          <Route path="conversations/new" element={<NewConversationPage />} />
          <Route path="conversations/:convId" element={<ConversationPage />} />
          <Route path="tracks/:trackId" element={<TrackPage />} />
          <Route path="tracks/:trackId/context" element={<ContextEditorPage />} />
          <Route path="tracks/:trackId/sessions/:sessionNum" element={<SessionPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
