/**
 * Transcript Parser for Claude Code
 *
 * Parses JSONL transcript files to extract session state.
 */

import * as fs from "fs";
import * as path from "path";

export interface TodoItem {
  content: string;
  status: "pending" | "in_progress" | "completed";
  activeForm?: string;
}

export interface ToolCall {
  tool: string;
  success: boolean;
  filePath?: string;
  command?: string;
  timestamp?: string;
}

export interface TranscriptState {
  lastTodos: TodoItem[];
  recentToolCalls: ToolCall[];
  filesModified: string[];
  errorsEncountered: string[];
  lastAssistantMessage: string;
  sessionId?: string;
}

interface TranscriptEntry {
  type?: string;
  message?: {
    role?: string;
    content?: unknown;
  };
  tool_use?: {
    name?: string;
    input?: Record<string, unknown>;
  };
  tool_result?: {
    is_error?: boolean;
    content?: string;
  };
  todos?: TodoItem[];
}

/**
 * Find the most recent transcript file in the Claude cache directory.
 */
export function findTranscriptPath(): string | null {
  const cacheDir = path.join(
    process.env.HOME || "",
    ".claude",
    "projects"
  );

  if (!fs.existsSync(cacheDir)) {
    return null;
  }

  // Find project directories
  const projectDirs = fs.readdirSync(cacheDir).filter((name) => {
    const fullPath = path.join(cacheDir, name);
    return fs.statSync(fullPath).isDirectory();
  });

  let mostRecentTranscript: string | null = null;
  let mostRecentTime = 0;

  for (const projectDir of projectDirs) {
    const transcriptDir = path.join(cacheDir, projectDir);
    const files = fs.readdirSync(transcriptDir).filter((f) =>
      f.endsWith(".jsonl")
    );

    for (const file of files) {
      const filePath = path.join(transcriptDir, file);
      const stat = fs.statSync(filePath);
      if (stat.mtimeMs > mostRecentTime) {
        mostRecentTime = stat.mtimeMs;
        mostRecentTranscript = filePath;
      }
    }
  }

  return mostRecentTranscript;
}

/**
 * Parse a JSONL transcript file.
 */
export function parseTranscript(jsonlPath: string): TranscriptState {
  const result: TranscriptState = {
    lastTodos: [],
    recentToolCalls: [],
    filesModified: [],
    errorsEncountered: [],
    lastAssistantMessage: "",
  };

  if (!fs.existsSync(jsonlPath)) {
    return result;
  }

  const content = fs.readFileSync(jsonlPath, "utf-8");
  const lines = content.trim().split("\n");

  const toolCalls: ToolCall[] = [];
  const filesModified = new Set<string>();
  const errors: string[] = [];
  let lastAssistantMessage = "";
  let lastTodos: TodoItem[] = [];

  for (const line of lines) {
    if (!line.trim()) continue;

    try {
      const entry = JSON.parse(line) as TranscriptEntry;

      // Track TodoWrite state
      if (entry.todos) {
        lastTodos = entry.todos;
      }

      // Track assistant messages
      if (entry.message?.role === "assistant") {
        const content = entry.message.content;
        if (typeof content === "string") {
          lastAssistantMessage = content;
        } else if (Array.isArray(content)) {
          const textParts = content
            .filter((c): c is { type: "text"; text: string } => c.type === "text")
            .map((c) => c.text);
          if (textParts.length > 0) {
            lastAssistantMessage = textParts.join("\n");
          }
        }
      }

      // Track tool uses
      if (entry.tool_use) {
        const tool = entry.tool_use.name || "unknown";
        const input = entry.tool_use.input || {};

        const toolCall: ToolCall = {
          tool,
          success: true,
          timestamp: new Date().toISOString(),
        };

        // Track file modifications
        if (tool === "Write" || tool === "Edit") {
          const filePath = (input.file_path || input.path) as string;
          if (filePath) {
            toolCall.filePath = filePath;
            filesModified.add(filePath);
          }
        }

        // Track bash commands
        if (tool === "Bash") {
          toolCall.command = input.command as string;
        }

        toolCalls.push(toolCall);
      }

      // Track tool errors
      if (entry.tool_result?.is_error) {
        const errorContent = entry.tool_result.content || "Unknown error";
        errors.push(
          errorContent.length > 200
            ? errorContent.substring(0, 200) + "..."
            : errorContent
        );

        // Mark last tool call as failed
        if (toolCalls.length > 0) {
          toolCalls[toolCalls.length - 1].success = false;
        }
      }
    } catch {
      // Skip malformed lines
      continue;
    }
  }

  result.lastTodos = lastTodos;
  result.recentToolCalls = toolCalls.slice(-5); // Last 5 tool calls
  result.filesModified = Array.from(filesModified);
  result.errorsEncountered = errors.slice(-5); // Last 5 errors
  result.lastAssistantMessage = lastAssistantMessage;

  return result;
}

/**
 * Get the current transcript state.
 */
export function getCurrentTranscriptState(): TranscriptState {
  const transcriptPath = findTranscriptPath();
  if (!transcriptPath) {
    return {
      lastTodos: [],
      recentToolCalls: [],
      filesModified: [],
      errorsEncountered: [],
      lastAssistantMessage: "",
    };
  }
  return parseTranscript(transcriptPath);
}
