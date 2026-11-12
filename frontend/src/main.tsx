import { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { checkReadiness } from "./api";

function App() {
  const [status, setStatus] = useState("Checking API…");

  useEffect(() => {
    const controller = new AbortController();
    checkReadiness(controller.signal).then(
      () => setStatus("API and database are ready."),
      (error: unknown) => {
        if (!controller.signal.aborted) {
          setStatus(error instanceof Error ? error.message : "Unable to reach the API.");
        }
      },
    );
    return () => controller.abort();
  }, []);

  return (
    <main>
      <h1>GeoLog</h1>
      <p role="status">{status}</p>
    </main>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
