import createClient from "openapi-fetch";
import type { paths } from "./schema";
import { getAccessToken } from "../auth/tokenProvider";

export const apiClient = createClient<paths>({ baseUrl: "" });

apiClient.use({
  async onRequest({ request }) {
    const token = await getAccessToken();
    if (token) request.headers.set("Authorization", `Bearer ${token}`);

    return request;
  },
});

export function apiErrorMessage(error: unknown): string {
  if (error && typeof error === "object" && "message" in error) {
    return String(error.message);
  }

  return "request failed";
}
