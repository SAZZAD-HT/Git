import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { Layout } from './app/Layout';
import { RequireAuth } from './app/RequireAuth';
import { SessionProvider } from './app/session';
import { applyTheme, loadTheme } from './app/theme';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ExplorerPage } from './features/explorer/ExplorerPage';
import { LoginPage } from './features/auth/LoginPage';
import { RegisterPage } from './features/auth/RegisterPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { ProfileSection } from './features/settings/ProfileSection';
import { PasswordSection } from './features/settings/PasswordSection';
import { TokensSection } from './features/settings/TokensSection';
import { SessionsSection } from './features/settings/SessionsSection';
import { AdminUsersPage } from './features/admin/AdminUsersPage';
import { NewRepoPage } from './features/repo/NewRepoPage';
import { RepoLayout } from './features/repo/RepoLayout';
import { CodePage } from './features/repo/CodePage';
import { BlobPage } from './features/repo/BlobPage';
import { CommitsPage } from './features/repo/CommitsPage';
import { RepoSettingsPage } from './features/repo/RepoSettingsPage';
import { CommitPage } from './features/repo/CommitPage';
import { ComparePage } from './features/repo/ComparePage';
import { ProfilePage } from './features/users/ProfilePage';
import { NotFoundPage } from './components/NotFoundPage';
import './styles/tokens.css';
import './styles/globals.css';
import './styles/repo.css';

applyTheme(loadTheme());

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: 1, refetchOnWindowFocus: false } },
});

// Routes mirror GitHub's URLs (docs/reference/09 §2). Feature routes are added phase by phase.
const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { path: '/', element: <DashboardPage /> },
      { path: '/login', element: <LoginPage /> },
      { path: '/register', element: <RegisterPage /> },
      {
        path: '/settings',
        element: <RequireAuth><SettingsPage /></RequireAuth>,
        children: [
          { index: true, element: <ProfileSection /> },
          { path: 'password', element: <PasswordSection /> },
          { path: 'tokens', element: <TokensSection /> },
          { path: 'sessions', element: <SessionsSection /> },
        ],
      },
      { path: '/admin/users', element: <RequireAuth admin><AdminUsersPage /></RequireAuth> },
      { path: '/explorer', element: <ExplorerPage /> },
      { path: '/new', element: <RequireAuth><NewRepoPage /></RequireAuth> },
      { path: '/:owner', element: <ProfilePage /> },
      {
        path: '/:owner/:repo',
        element: <RepoLayout />,
        children: [
          { index: true, element: <CodePage /> },
          { path: 'tree/*', element: <CodePage /> },
          { path: 'blob/*', element: <BlobPage /> },
          { path: 'commits', element: <CommitsPage /> },
          { path: 'commit/:sha', element: <CommitPage /> },
          { path: 'compare/*', element: <ComparePage /> },
          { path: 'settings', element: <RepoSettingsPage /> },
        ],
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <SessionProvider>
        <RouterProvider router={router} />
      </SessionProvider>
    </QueryClientProvider>
  </StrictMode>,
);
