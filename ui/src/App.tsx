import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppLayout } from '@/components/app/AppLayout'
import { HomePage } from '@/pages/HomePage'
import { TrackPage } from '@/pages/TrackPage'
import { ContextEditorPage } from '@/pages/ContextEditorPage'
import { NotFoundPage } from '@/pages/NotFoundPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppLayout />}>
          <Route index element={<HomePage />} />
          <Route path="tracks/:trackId" element={<TrackPage />} />
          <Route path="tracks/:trackId/context" element={<ContextEditorPage />} />
          <Route path="*" element={<NotFoundPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
