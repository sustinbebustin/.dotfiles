#!/usr/bin/env node

// src/skill-activation-prompt.ts
import { readFileSync, existsSync, readdirSync } from "fs";
import { join } from "path";
function parsePluginId(pluginId) {
  const parts = pluginId.split("@");
  if (parts.length !== 2) return null;
  return { name: parts[0], source: parts[1] };
}
function compareSemver(a, b) {
  const parseVersion = (v) => v.split(".").map((n) => parseInt(n, 10) || 0);
  const va = parseVersion(a);
  const vb = parseVersion(b);
  for (let i = 0; i < Math.max(va.length, vb.length); i++) {
    const na = va[i] || 0;
    const nb = vb[i] || 0;
    if (na !== nb) return na - nb;
  }
  return 0;
}
function findHighestVersion(pluginPath) {
  if (!existsSync(pluginPath)) return null;
  try {
    const entries = readdirSync(pluginPath, { withFileTypes: true });
    const versions = entries.filter((e) => e.isDirectory() && /^\d+\.\d+/.test(e.name)).map((e) => e.name).sort(compareSemver);
    return versions.length > 0 ? versions[versions.length - 1] : null;
  } catch {
    return null;
  }
}
function loadEnabledPlugins(homeDir, projectDir) {
  const plugins = [];
  const seen = /* @__PURE__ */ new Set();
  const settingsPaths = [
    join(homeDir, ".claude", "settings.json"),
    join(projectDir, ".claude", "settings.json"),
    join(projectDir, ".claude", "settings.local.json")
  ];
  for (const settingsPath of settingsPaths) {
    if (!existsSync(settingsPath)) continue;
    try {
      const settings = JSON.parse(readFileSync(settingsPath, "utf-8"));
      if (settings.enabledPlugins) {
        for (const [pluginId, enabled] of Object.entries(settings.enabledPlugins)) {
          if (!enabled) continue;
          const parsed = parsePluginId(pluginId);
          if (parsed && !seen.has(pluginId)) {
            seen.add(pluginId);
            plugins.push(parsed);
          }
        }
      }
    } catch {
    }
  }
  return plugins;
}
function loadPluginSkillRules(homeDir, plugin) {
  const pluginBasePath = join(homeDir, ".claude", "plugins", "cache", plugin.source, plugin.name);
  const highestVersion = findHighestVersion(pluginBasePath);
  if (!highestVersion) return null;
  const skillRulesPath = join(pluginBasePath, highestVersion, "skills", "skill-rules.json");
  if (!existsSync(skillRulesPath)) return null;
  try {
    return JSON.parse(readFileSync(skillRulesPath, "utf-8"));
  } catch {
    return null;
  }
}
function mergeSkillRules(...rulesSets) {
  const merged = { version: "1.0", skills: {}, agents: {} };
  for (const rules of rulesSets) {
    if (rules.version) merged.version = rules.version;
    merged.skills = { ...merged.skills, ...rules.skills };
    merged.agents = { ...merged.agents || {}, ...rules.agents || {} };
  }
  return merged;
}
async function main() {
  try {
    const input = readFileSync(0, "utf-8");
    const data = JSON.parse(input);
    const prompt = data.prompt.toLowerCase();
    const projectDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();
    const homeDir = process.env.HOME || "";
    const projectRulesPath = join(projectDir, ".claude", "skills", "skill-rules.json");
    const globalRulesPath = join(homeDir, ".claude", "skills", "skill-rules.json");
    let globalRules = { version: "1.0", skills: {}, agents: {} };
    let projectRules = { version: "1.0", skills: {}, agents: {} };
    if (existsSync(globalRulesPath)) {
      try {
        globalRules = JSON.parse(readFileSync(globalRulesPath, "utf-8"));
      } catch (e) {
      }
    }
    if (existsSync(projectRulesPath)) {
      try {
        projectRules = JSON.parse(readFileSync(projectRulesPath, "utf-8"));
      } catch (e) {
      }
    }
    const enabledPlugins = loadEnabledPlugins(homeDir, projectDir);
    const pluginRulesList = [];
    for (const plugin of enabledPlugins) {
      const pluginRules = loadPluginSkillRules(homeDir, plugin);
      if (pluginRules) {
        pluginRulesList.push(pluginRules);
      }
    }
    const hasGlobalRules = existsSync(globalRulesPath);
    const hasProjectRules = existsSync(projectRulesPath);
    const hasPluginRules = pluginRulesList.length > 0;
    if (!hasGlobalRules && !hasProjectRules && !hasPluginRules) {
      process.exit(0);
    }
    const rules = mergeSkillRules(globalRules, ...pluginRulesList, projectRules);
    const matchedSkills = [];
    for (const [skillName, config] of Object.entries(rules.skills)) {
      const triggers = config.promptTriggers;
      if (!triggers) {
        continue;
      }
      if (triggers.keywords) {
        const keywordMatch = triggers.keywords.some(
          (kw) => prompt.includes(kw.toLowerCase())
        );
        if (keywordMatch) {
          matchedSkills.push({ name: skillName, matchType: "keyword", config });
          continue;
        }
      }
      if (triggers.intentPatterns) {
        const intentMatch = triggers.intentPatterns.some((pattern) => {
          const regex = new RegExp(pattern, "i");
          return regex.test(prompt);
        });
        if (intentMatch) {
          matchedSkills.push({ name: skillName, matchType: "intent", config });
        }
      }
    }
    const matchedAgents = [];
    if (rules.agents) {
      for (const [agentName, config] of Object.entries(rules.agents)) {
        const triggers = config.promptTriggers;
        if (!triggers) {
          continue;
        }
        if (triggers.keywords) {
          const keywordMatch = triggers.keywords.some(
            (kw) => prompt.includes(kw.toLowerCase())
          );
          if (keywordMatch) {
            matchedAgents.push({ name: agentName, matchType: "keyword", config, isAgent: true });
            continue;
          }
        }
        if (triggers.intentPatterns) {
          const intentMatch = triggers.intentPatterns.some((pattern) => {
            const regex = new RegExp(pattern, "i");
            return regex.test(prompt);
          });
          if (intentMatch) {
            matchedAgents.push({ name: agentName, matchType: "intent", config, isAgent: true });
          }
        }
      }
    }
    if (matchedSkills.length > 0 || matchedAgents.length > 0) {
      let output = "\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n";
      output += "\u{1F3AF} SKILL ACTIVATION CHECK\n";
      output += "\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n\n";
      const critical = matchedSkills.filter((s) => s.config.priority === "critical");
      const high = matchedSkills.filter((s) => s.config.priority === "high");
      const medium = matchedSkills.filter((s) => s.config.priority === "medium");
      const low = matchedSkills.filter((s) => s.config.priority === "low");
      if (critical.length > 0) {
        output += "\u26A0\uFE0F CRITICAL SKILLS (REQUIRED):\n";
        critical.forEach((s) => output += `  \u2192 ${s.name}
`);
        output += "\n";
      }
      if (high.length > 0) {
        output += "\u{1F4DA} RECOMMENDED SKILLS:\n";
        high.forEach((s) => output += `  \u2192 ${s.name}
`);
        output += "\n";
      }
      if (medium.length > 0) {
        output += "\u{1F4A1} SUGGESTED SKILLS:\n";
        medium.forEach((s) => output += `  \u2192 ${s.name}
`);
        output += "\n";
      }
      if (low.length > 0) {
        output += "\u{1F4CC} OPTIONAL SKILLS:\n";
        low.forEach((s) => output += `  \u2192 ${s.name}
`);
        output += "\n";
      }
      if (matchedAgents.length > 0) {
        output += "\u{1F916} RECOMMENDED AGENTS (token-efficient):\n";
        matchedAgents.forEach((a) => output += `  \u2192 ${a.name}
`);
        output += "\n";
      }
      if (matchedSkills.length > 0) {
        output += "ACTION: Use Skill tool BEFORE responding\n";
      }
      if (matchedAgents.length > 0) {
        output += "ACTION: Use Task tool with agent for exploration\n";
      }
      output += "\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\u2501\n";
      console.log(output);
    }
    process.exit(0);
  } catch (err) {
    console.error("Error in skill-activation-prompt hook:", err);
    process.exit(1);
  }
}
main().catch((err) => {
  console.error("Uncaught error:", err);
  process.exit(1);
});
