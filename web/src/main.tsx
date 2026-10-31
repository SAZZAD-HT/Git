import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { createBrowserRouter, RouterProvider } from 'react-router-dom';
import { Layout } from './app/Layout';
import { applyTheme, loadTheme } from './app/theme';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { ExplorerPage } from './features/explorer/ExplorerPage';
import { NotFoundPage } from './components/NotFoundPage';
import './styles/tokens.css';
import './styles/globals.css';

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
      { path: '/explorer', element: <ExplorerPage /> },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
);
