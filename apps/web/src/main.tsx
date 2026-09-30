import { lazy, StrictMode, Suspense } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import App from "./App";

// Loaded only for the visual check (it pulls in the server renderer).
const RenderPage = lazy(() => import("./pages/Render"));
import "./styles.css";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    {location.pathname === "/render" ? <Suspense><RenderPage /></Suspense> : (
      <BrowserRouter>
        <App />
      </BrowserRouter>
    )}
  </StrictMode>,
);
