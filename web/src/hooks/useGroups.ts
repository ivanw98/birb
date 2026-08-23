import { useState } from "react";
import { useApiQuery } from "./useApiQuery";
import {
  createGroup,
  deleteGroup,
  joinGroup,
  leaveGroup,
  listGroups,
  removeMember,
  type Group,
} from "@/api/groups";

export function useGroups() {
  const [nonce, setNonce] = useState(0);
  const groups = useApiQuery<Group[]>(listGroups, ["groups", nonce]);
  const reload = () => setNonce((n) => n + 1);

  // reload() in finally, not on success: every endpoint but create is
  // idempotent, and a rejected call may still have landed on the server.
  const withReload = async (run: () => Promise<unknown>): Promise<void> => {
    try {
      await run();
    } finally {
      reload();
    }
  };

  return {
    groups,
    reload,
    create: (name: string) => withReload(() => createGroup(name)),
    join: (code: string) => withReload(() => joinGroup(code)),
    leave: (id: string) => withReload(() => leaveGroup(id)),
    remove: (id: string) => withReload(() => deleteGroup(id)),
    removeGroupMember: (groupId: string, userId: string) =>
      withReload(() => removeMember(groupId, userId)),
  };
}
