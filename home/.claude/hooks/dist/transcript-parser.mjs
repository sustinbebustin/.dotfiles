// src/transcript-parser.ts
import * as fs from "fs";
import * as path from "path";
function findTranscriptPath() {
  const cacheDir = path.join(
    process.env.HOME || "",
    ".claude",
    "projects"
  );
  if (!fs.existsSync(cacheDir)) {
    return null;
  }
  const projectDirs = fs.readdirSync(cacheDir).filter((name) => {
    const fullPath = path.join(cacheDir, name);
    return fs.statSync(fullPath).isDirectory();
  });
  let mostRecentTranscript = null;
  let mostRecentTime = 0;
  for (const projectDir of projectDirs) {
    const transcriptDir = path.join(cacheDir, projectDir);
    const files = fs.readdirSync(transcriptDir).filter(
      (f) => f.endsWith(".jsonl")
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
function parseTranscript(jsonlPath) {
  const result = {
    lastTodos: [],
    recentToolCalls: [],
    filesModified: [],
    errorsEncountered: [],
    lastAssistantMessage: ""
  };
  if (!fs.existsSync(jsonlPath)) {
    return result;
  }
  const content = fs.readFileSync(jsonlPath, "utf-8");
  const lines = content.trim().split("\n");
  const toolCalls = [];
  const filesModified = /* @__PURE__ */ new Set();
  const errors = [];
  let lastAssistantMessage = "";
  let lastTodos = [];
  for (const line of lines) {
    if (!line.trim()) continue;
    try {
      const entry = JSON.parse(line);
      if (entry.todos) {
        lastTodos = entry.todos;
      }
      if (entry.message?.role === "assistant") {
        const content2 = entry.message.content;
        if (typeof content2 === "string") {
          lastAssistantMessage = content2;
        } else if (Array.isArray(content2)) {
          const textParts = content2.filter((c) => c.type === "text").map((c) => c.text);
          if (textParts.length > 0) {
            lastAssistantMessage = textParts.join("\n");
          }
        }
      }
      if (entry.tool_use) {
        const tool = entry.tool_use.name || "unknown";
        const input = entry.tool_use.input || {};
        const toolCall = {
          tool,
          success: true,
          timestamp: (/* @__PURE__ */ new Date()).toISOString()
        };
        if (tool === "Write" || tool === "Edit") {
          const filePath = input.file_path || input.path;
          if (filePath) {
            toolCall.filePath = filePath;
            filesModified.add(filePath);
          }
        }
        if (tool === "Bash") {
          toolCall.command = input.command;
        }
        toolCalls.push(toolCall);
      }
      if (entry.tool_result?.is_error) {
        const errorContent = entry.tool_result.content || "Unknown error";
        errors.push(
          errorContent.length > 200 ? errorContent.substring(0, 200) + "..." : errorContent
        );
        if (toolCalls.length > 0) {
          toolCalls[toolCalls.length - 1].success = false;
        }
      }
    } catch {
      continue;
    }
  }
  result.lastTodos = lastTodos;
  result.recentToolCalls = toolCalls.slice(-5);
  result.filesModified = Array.from(filesModified);
  result.errorsEncountered = errors.slice(-5);
  result.lastAssistantMessage = lastAssistantMessage;
  return result;
}
function getCurrentTranscriptState() {
  const transcriptPath = findTranscriptPath();
  if (!transcriptPath) {
    return {
      lastTodos: [],
      recentToolCalls: [],
      filesModified: [],
      errorsEncountered: [],
      lastAssistantMessage: ""
    };
  }
  return parseTranscript(transcriptPath);
}
export {
  findTranscriptPath,
  getCurrentTranscriptState,
  parseTranscript
};
