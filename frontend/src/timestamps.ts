export function formatUTC(timestamp: string): string {
  const date = new Date(timestamp);
  return Number.isFinite(date.getTime())
    ? date.toISOString().slice(0, 19).replace("T", " ") + " UTC"
    : "Unknown time";
}
