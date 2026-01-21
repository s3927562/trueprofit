import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { isAuthed } from './services/auth'
import AskView from './views/AskView'
import CallbackView from './views/CallbackView'
import HomeView from './views/HomeView'
import LoginView from './views/LoginView'
import ShopifyView from './views/ShopifyView'
import SummaryView from './views/SummaryView'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  if (!isAuthed()) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<ProtectedRoute><HomeView /></ProtectedRoute>} />
        <Route path="/login" element={<LoginView />} />
        <Route path="/callback" element={<CallbackView />} />
        <Route path="/summary" element={<ProtectedRoute><SummaryView /></ProtectedRoute>} />
        <Route path="/shopify" element={<ProtectedRoute><ShopifyView /></ProtectedRoute>} />
        <Route path="/ask" element={<ProtectedRoute><AskView /></ProtectedRoute>} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
