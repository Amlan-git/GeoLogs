import type { Category } from "./api";

export const categories: Record<Category, { label: string; color: string }> = {
  delivery_zone: { label: "Delivery zone", color: "#2166ce" },
  restricted_zone: { label: "Restricted zone", color: "#c73a47" },
  toll_zone: { label: "Toll zone", color: "#a96512" },
  customer_area: { label: "Customer area", color: "#14816a" },
};
