import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { isAuthed } from './services/auth'
import HomeView from './views/HomeView'
import LoginView from './views/LoginView'
import CallbackView from './views/CallbackView'
import SummaryView from './views/SummaryView'
import ShopifyView from './views/ShopifyView'

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
      </Routes>
    </BrowserRouter>
  )
}

export default App
