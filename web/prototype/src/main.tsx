import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { AppErrorBoundary } from './components/AppErrorBoundary';
import { PrototypeProvider } from './prototype-context';
import App from './App';
import './styles.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <AppErrorBoundary>
      <BrowserRouter>
        <PrototypeProvider>
          <App />
        </PrototypeProvider>
      </BrowserRouter>
    </AppErrorBoundary>
  </StrictMode>,
);
