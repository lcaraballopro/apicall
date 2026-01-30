import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Layout } from '@/components/layout';
import {
  LoginPage,
  DashboardPage,
  ProyectosPage,
  TroncalesPage,
  TestCallPage,
  ReportsPage,
  AudiosPage,
  UsersPage,
  CampaignsPage,
  ConfigPage,
} from '@/pages';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30000,
      retry: 1,
    },
  },
});

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route element={<Layout />}>
            <Route path="/" element={<DashboardPage />} />
            <Route path="/proyectos" element={<ProyectosPage />} />
            <Route path="/campanas" element={<CampaignsPage />} />
            <Route path="/troncales" element={<TroncalesPage />} />
            <Route path="/testcall" element={<TestCallPage />} />
            <Route path="/reportes" element={<ReportsPage />} />
            <Route path="/audios" element={<AudiosPage />} />
            <Route path="/usuarios" element={<UsersPage />} />
            <Route path="/configuracion" element={<ConfigPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

export default App;
