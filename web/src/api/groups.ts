import { apiClient, apiErrorMessage } from "./client";
import type { components } from "./schema";

export type Group = components["schemas"]["Group"];
export type GroupMember = components["schemas"]["GroupMember"];

export async function listGroups(): Promise<Group[]> {
  const { data, error } = await apiClient.GET("/api/groups");
  if (!data) throw new Error(apiErrorMessage(error));

  return data;
}

export async function createGroup(name: string): Promise<Group> {
  const { data, error } = await apiClient.POST("/api/groups", {
    body: { name },
  });
  if (!data) throw new Error(apiErrorMessage(error));

  return data;
}

export async function joinGroup(code: string): Promise<Group> {
  const { data, error } = await apiClient.POST("/api/groups/join", {
    body: { code },
  });
  if (!data) throw new Error(apiErrorMessage(error));

  return data;
}

// The 204 endpoints have no body, so success is response.ok, not data.
export async function leaveGroup(id: string): Promise<void> {
  const { error, response } = await apiClient.POST("/api/groups/{id}/leave", {
    params: { path: { id } },
  });
  if (!response.ok) throw new Error(apiErrorMessage(error));
}

export async function deleteGroup(id: string): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/groups/{id}", {
    params: { path: { id } },
  });
  if (!response.ok) throw new Error(apiErrorMessage(error));
}

export async function removeMember(
  groupId: string,
  userId: string,
): Promise<void> {
  const { error, response } = await apiClient.DELETE(
    "/api/groups/{id}/members/{userId}",
    {
      params: { path: { id: groupId, userId } },
    },
  );
  if (!response.ok) throw new Error(apiErrorMessage(error));
}
