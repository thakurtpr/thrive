/**
 * Thrive validation plugin — pre/post tool hooks.
 *
 * - tool.execute.before: block destructive shell ops and secret leaks.
 * - session.created: log session start for audit trail.
 */

const BLOCKED_PATTERNS: RegExp[] = [
  /\brm\s+-rf\s+(\/|\/\*|~)/, // rm -rf on / or $HOME
  /\bmkfs\b/,
  /\bdd\s+.*of=\/dev\//, // dd onto devices
  />\s*\/dev\/sd[a-z]/, // redirect onto disks
  /--privileged/, // no privileged container runs from the agent
];

const SECRET_PATTERNS: RegExp[] = [
  /AKIA[0-9A-Z]{16}/, // AWS access key
  /ghp_[A-Za-z0-9]{36,}/, // GitHub token
  /-----BEGIN (RSA |OPENSSH |EC )?PRIVATE KEY-----/, // private keys
];

type ToolEvent = {
  tool: string;
  input?: Record<string, unknown>;
};

export async function onToolExecuteBefore(event: ToolEvent): Promise<{ allow: boolean; reason?: string }> {
  if (event.tool !== "bash") return { allow: true };

  const command = String(event.input?.command ?? "");
  for (const pattern of BLOCKED_PATTERNS) {
    if (pattern.test(command)) {
      return { allow: false, reason: `Blocked destructive command pattern: ${pattern}` };
    }
  }
  for (const pattern of SECRET_PATTERNS) {
    if (pattern.test(command)) {
      return { allow: false, reason: "Blocked: command appears to contain a secret. Use env vars or the secret store." };
    }
  }
  return { allow: true };
}

export async function onSessionCreated(session: { id?: string }): Promise<void> {
  // Audit trail only — never print secrets here.
  console.log(`[thrive-validate] session started: ${session.id ?? "unknown"}`);
}
