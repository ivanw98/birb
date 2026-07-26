import type { components } from "@/api/schema";
import { useApiQuery } from "./useApiQuery";
import { apiClient, apiErrorMessage } from "@/api/client";

export type Me = components["schemas"]["Me"];

export function useMe() {
  const fetcher = async () => {
    const { data, error } = await apiClient.GET("/api/me");
    if (!data) throw new Error(apiErrorMessage(error));

    return data;
  };
  return useApiQuery<Me>(fetcher);
}
