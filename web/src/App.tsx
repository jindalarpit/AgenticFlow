import { lazy, Suspense } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { Layout } from "./components/Layout";
import { ToastProvider } from "./components/Toast";
import Login from "./pages/Login";
import Dashboard from "./pages/Dashboard";
import AgentsPage from "./pages/AgentsPage";
import AgentDetail from "./pages/AgentDetail";
import Settings from "./pages/Settings";
import SkillForm from "./pages/SkillForm";
import SkillList from "./pages/SkillList";

const TaskDetail = lazy(() => import("./pages/TaskDetail"));
const AgentCreatePage = lazy(() => import("./pages/AgentCreatePage"));
const AgentEditPage = lazy(() => import("./pages/AgentEditPage"));

function Placeholder({ title }: { title: string }) {
  return (
    <div className="flex items-center justify-center min-h-[calc(100vh-57px)]">
      <h1 className="text-2xl font-semibold text-gray-700">{title}</h1>
    </div>
  );
}

export default function App() {
  return (
    <ToastProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/"
            element={
              <ProtectedRoute>
                <Layout>
                  <Dashboard />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/tasks/:id"
            element={
              <ProtectedRoute>
                <Layout>
                  <Suspense fallback={<div className="p-8 text-gray-500">Loading...</div>}>
                    <TaskDetail />
                  </Suspense>
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/history"
            element={
              <ProtectedRoute>
                <Layout>
                  <Placeholder title="Task History" />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/agents"
            element={
              <ProtectedRoute>
                <Layout>
                  <AgentsPage />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/agents/new"
            element={
              <ProtectedRoute>
                <Layout>
                  <Suspense fallback={<div className="p-8 text-gray-500">Loading...</div>}>
                    <AgentCreatePage />
                  </Suspense>
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/agents/:id/edit"
            element={
              <ProtectedRoute>
                <Layout>
                  <Suspense fallback={<div className="p-8 text-gray-500">Loading...</div>}>
                    <AgentEditPage />
                  </Suspense>
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/agents/:id"
            element={
              <ProtectedRoute>
                <Layout>
                  <AgentDetail />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/skills"
            element={
              <ProtectedRoute>
                <Layout>
                  <SkillList />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/skills/new"
            element={
              <ProtectedRoute>
                <Layout>
                  <SkillForm />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/skills/:id/edit"
            element={
              <ProtectedRoute>
                <Layout>
                  <SkillForm />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route
            path="/settings"
            element={
              <ProtectedRoute>
                <Layout>
                  <Settings />
                </Layout>
              </ProtectedRoute>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  );
}
