export async function checkReadiness(signal: AbortSignal): Promise<void> {
  const baseURL = import.meta.env.VITE_API_BASE_URL?.replace(/\/+$/, "");
  if (!baseURL) {
    throw new Error("API URL is missing.");
  }

  const response = await fetch(`${baseURL}/healthz`, { signal });
  if (!response.ok) {
    throw new Error(`API returned HTTP ${response.status}.`);
  }

  const data: unknown = await response.json();
  if (
    typeof data !== "object" ||
    data === null ||
    !("status" in data) ||
    data.status !== "ok" ||
    !("time_ns" in data) ||
    typeof data.time_ns !== "string"
  ) {
    throw new Error("Unexpected readiness response.");
  }
}
