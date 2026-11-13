import type { Coordinates } from "./api";

const decimal = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:e[+-]?\d+)?$/i;

export function parseCoordinateRing(text: string): Coordinates[] {
  const ring: Coordinates[] = [];
  const unique = new Set<string>();
  for (const [index, line] of text.split(/\r?\n/).entries()) {
    if (!line.trim()) continue;
    const fields = line.split(",").map((part) => part.trim());
    if (fields.length !== 2 || fields.some((part) => !decimal.test(part))) {
      throw new Error(`Line ${index + 1}: enter two numbers separated by a comma.`);
    }
    const [lat, lon] = fields.map(Number);
    if (Math.abs(lat) > 90 || Math.abs(lon) > 180) {
      throw new Error(`Line ${index + 1}: latitude must be between −90 and 90 and longitude between −180 and 180.`);
    }
    ring.push([lat, lon]);
    unique.add(`${lat},${lon}`);
  }
  if (unique.size < 3) throw new Error("Enter at least three distinct coordinate pairs.");
  const first = ring[0];
  const last = ring[ring.length - 1];
  if (first[0] !== last[0] || first[1] !== last[1]) ring.push(first);
  return ring;
}
