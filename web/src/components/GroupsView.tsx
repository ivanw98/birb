import { useState } from "react";
import { useGroups } from "@/hooks/useGroups";
import { apiErrorMessage } from "@/api/client";
import type { Group } from "@/api/groups";
import { StatusBanner } from "./StatusBanner";

interface GroupsViewProps {
  onBack: () => void;
}

// Wire code is 8 chars, no separators; the hyphen is display-only.
function displayCode(code: string): string {
  return `${code.slice(0, 4)}-${code.slice(4)}`;
}

export function GroupsView({ onBack }: GroupsViewProps) {
  const { groups, create, join, leave, remove, removeGroupMember } =
    useGroups();
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [creating, setCreating] = useState(false);
  const [joining, setJoining] = useState(false);

  const handleCreate = async () => {
    setCreating(true);
    setError(null);
    try {
      await create(name.trim());
      setName("");
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const handleJoin = async () => {
    setJoining(true);
    setError(null);
    try {
      await join(code.trim());
      setCode("");
    } catch (err) {
      setError(apiErrorMessage(err));
    } finally {
      setJoining(false);
    }
  };

  return (
    <div className="flex w-full max-w-md flex-col gap-4 self-center">
      <button
        type="button"
        onClick={onBack}
        className="h-12 rounded-md border border-slate-300 bg-white text-lg font-medium text-ink"
      >
        Back to feed
      </button>

      {error && (
        <StatusBanner tone="danger" onDismiss={() => setError(null)}>
          {error}
        </StatusBanner>
      )}

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void handleCreate();
        }}
        className="flex flex-col gap-2"
      >
        <label htmlFor="group-name" className="text-lg text-ink">
          Start a new group
        </label>
        <div className="flex gap-3">
          <input
            id="group-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="h-12 min-w-0 flex-1 rounded-md border border-slate-300 px-3 text-lg text-ink"
          />
          <button
            type="submit"
            disabled={creating || name.trim() === ""}
            className="h-12 rounded-md bg-primary px-4 text-lg font-medium text-white disabled:opacity-50"
          >
            {creating ? "Creating…" : "Create"}
          </button>
        </div>
      </form>

      <form
        onSubmit={(e) => {
          e.preventDefault();
          void handleJoin();
        }}
        className="flex flex-col gap-2"
      >
        <label htmlFor="join-code" className="text-lg text-ink">
          Join with a code
        </label>
        <div className="flex gap-3">
          <input
            id="join-code"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            autoCapitalize="characters"
            autoComplete="off"
            className="h-12 min-w-0 flex-1 rounded-md border border-slate-300 px-3 text-lg tracking-widest text-ink"
          />
          <button
            type="submit"
            disabled={joining || code.trim() === ""}
            className="h-12 rounded-md bg-primary px-4 text-lg font-medium text-white disabled:opacity-50"
          >
            {joining ? "Joining…" : "Join"}
          </button>
        </div>
      </form>

      {groups.status === "loading" && (
        <p className="p-4 text-muted">Loading your groups...</p>
      )}
      {groups.status === "error" && (
        <StatusBanner tone="danger">{groups.message}</StatusBanner>
      )}
      {groups.status === "success" &&
        (groups.data.length === 0 ? (
          <p className="p-4 text-muted">
            No groups yet. Create one, or join with a code from a friend.
          </p>
        ) : (
          <ul className="flex flex-col gap-3">
            {groups.data.map((group) => (
              <GroupCard
                key={group.id}
                group={group}
                onLeave={leave}
                onDelete={remove}
                onRemoveMember={removeGroupMember}
                onError={setError}
              />
            ))}
          </ul>
        ))}
    </div>
  );
}

type Confirming = { kind: "delete" } | { kind: "member"; userId: string };

interface GroupCardProps {
  group: Group;
  onLeave: (id: string) => Promise<void>;
  onDelete: (id: string) => Promise<void>;
  onRemoveMember: (groupId: string, userId: string) => Promise<void>;
  onError: (message: string) => void;
}

function GroupCard({
  group,
  onLeave,
  onDelete,
  onRemoveMember,
  onError,
}: GroupCardProps) {
  const [confirming, setConfirming] = useState<Confirming | null>(null);
  const [busy, setBusy] = useState(false);

  const runAction = async (action: () => Promise<void>) => {
    setBusy(true);
    try {
      await action();
    } catch (err) {
      onError(apiErrorMessage(err));
    } finally {
      setBusy(false);
      setConfirming(null);
    }
  };

  const confirmingMember =
    confirming?.kind === "member"
      ? group.members.find((m) => m.id === confirming.userId)
      : undefined;

  return (
    <li className="flex flex-col gap-3 rounded-lg border border-slate-200 p-4">
      <p className="text-lg font-semibold text-ink">{group.name}</p>
      <p className="text-center text-3xl font-semibold tracking-[0.2em] text-ink">
        {displayCode(group.joinCode)}
      </p>
      <p className="text-muted">Share this code to invite someone.</p>

      <ul className="flex flex-col gap-2">
        {group.members.map((member) => (
          <li
            key={member.id}
            className="flex items-center justify-between gap-3"
          >
            <span className="text-lg text-ink">
              {member.name ?? "A group member"}
              {member.isOwner && " (owner)"}
            </span>
            {group.isOwner && !member.isOwner && (
              <button
                type="button"
                onClick={() =>
                  setConfirming({ kind: "member", userId: member.id })
                }
                disabled={busy || confirming !== null}
                className="h-12 rounded-md border border-danger px-4 text-base font-medium text-danger disabled:opacity-50"
              >
                Remove
              </button>
            )}
          </li>
        ))}
      </ul>

      {confirming === null && (
        <div className="flex gap-3">
          {!group.isOwner && (
            <button
              type="button"
              onClick={() => void runAction(() => onLeave(group.id))}
              disabled={busy}
              className="h-12 flex-1 rounded-md border border-slate-300 bg-white text-base font-medium text-ink disabled:opacity-50"
            >
              {busy ? "Leaving…" : "Leave group"}
            </button>
          )}
          {group.isOwner && (
            <button
              type="button"
              onClick={() => setConfirming({ kind: "delete" })}
              disabled={busy}
              className="h-12 flex-1 rounded-md border border-danger text-base font-medium text-danger disabled:opacity-50"
            >
              Delete group
            </button>
          )}
        </div>
      )}

      {confirming?.kind === "delete" && (
        <ConfirmRow
          question={`Delete "${group.name}"? Everyone loses this group's feed.`}
          actionLabel="Delete"
          busy={busy}
          onCancel={() => setConfirming(null)}
          onConfirm={() => void runAction(() => onDelete(group.id))}
        />
      )}
      {confirmingMember && (
        <ConfirmRow
          question={`Remove ${confirmingMember.name ?? "this member"} from "${group.name}"?`}
          actionLabel="Remove"
          busy={busy}
          onCancel={() => setConfirming(null)}
          onConfirm={() =>
            void runAction(() => onRemoveMember(group.id, confirmingMember.id))
          }
        />
      )}
    </li>
  );
}

interface ConfirmRowProps {
  question: string;
  actionLabel: string;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}

function ConfirmRow({
  question,
  actionLabel,
  busy,
  onCancel,
  onConfirm,
}: ConfirmRowProps) {
  return (
    <div className="space-y-3">
      <p className="text-lg text-ink">{question}</p>
      <div className="flex gap-3">
        <button
          type="button"
          onClick={onCancel}
          disabled={busy}
          className="h-12 flex-1 rounded-md border border-slate-300 bg-white text-base font-medium text-ink disabled:opacity-50"
        >
          Keep it
        </button>
        <button
          type="button"
          onClick={onConfirm}
          disabled={busy}
          className="h-12 flex-1 rounded-md bg-danger text-base font-medium text-white disabled:opacity-50"
        >
          {busy ? "Working…" : actionLabel}
        </button>
      </div>
    </div>
  );
}
