#!/usr/bin/env node
import { readFileSync, existsSync, readdirSync } from 'fs';
import { join } from 'path';

interface HookInput {
    session_id: string;
    transcript_path: string;
    cwd: string;
    permission_mode: string;
    prompt: string;
}

interface PromptTriggers {
    keywords?: string[];
    intentPatterns?: string[];
}

interface SkillRule {
    type: 'guardrail' | 'domain';
    enforcement: 'block' | 'suggest' | 'warn';
    priority: 'critical' | 'high' | 'medium' | 'low';
    promptTriggers?: PromptTriggers;
}

interface SkillRules {
    version: string;
    skills: Record<string, SkillRule>;
    agents?: Record<string, SkillRule>;
}

interface MatchedSkill {
    name: string;
    matchType: 'keyword' | 'intent';
    config: SkillRule;
    isAgent?: boolean;
}

interface ClaudeSettings {
    enabledPlugins?: Record<string, boolean>;
}

interface ParsedPlugin {
    name: string;
    source: string;
}

/**
 * Parse plugin identifier (e.g., "meta@sustinbebustin-plugins") into name and source
 */
function parsePluginId(pluginId: string): ParsedPlugin | null {
    const parts = pluginId.split('@');
    if (parts.length !== 2) return null;
    return { name: parts[0], source: parts[1] };
}

/**
 * Compare semver strings. Returns positive if a > b, negative if a < b, 0 if equal.
 */
function compareSemver(a: string, b: string): number {
    const parseVersion = (v: string) => v.split('.').map(n => parseInt(n, 10) || 0);
    const va = parseVersion(a);
    const vb = parseVersion(b);
    for (let i = 0; i < Math.max(va.length, vb.length); i++) {
        const na = va[i] || 0;
        const nb = vb[i] || 0;
        if (na !== nb) return na - nb;
    }
    return 0;
}

/**
 * Find the highest version directory in a plugin path
 */
function findHighestVersion(pluginPath: string): string | null {
    if (!existsSync(pluginPath)) return null;
    try {
        const entries = readdirSync(pluginPath, { withFileTypes: true });
        const versions = entries
            .filter(e => e.isDirectory() && /^\d+\.\d+/.test(e.name))
            .map(e => e.name)
            .sort(compareSemver);
        return versions.length > 0 ? versions[versions.length - 1] : null;
    } catch {
        return null;
    }
}

/**
 * Load enabled plugins from settings.json files (global and project)
 */
function loadEnabledPlugins(homeDir: string, projectDir: string): ParsedPlugin[] {
    const plugins: ParsedPlugin[] = [];
    const seen = new Set<string>();

    const settingsPaths = [
        join(homeDir, '.claude', 'settings.json'),
        join(projectDir, '.claude', 'settings.json'),
        join(projectDir, '.claude', 'settings.local.json')
    ];

    for (const settingsPath of settingsPaths) {
        if (!existsSync(settingsPath)) continue;
        try {
            const settings: ClaudeSettings = JSON.parse(readFileSync(settingsPath, 'utf-8'));
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
            // Invalid JSON, skip
        }
    }

    return plugins;
}

/**
 * Load skill-rules.json from a plugin's cache directory
 */
function loadPluginSkillRules(homeDir: string, plugin: ParsedPlugin): SkillRules | null {
    const pluginBasePath = join(homeDir, '.claude', 'plugins', 'cache', plugin.source, plugin.name);
    const highestVersion = findHighestVersion(pluginBasePath);
    if (!highestVersion) return null;

    const skillRulesPath = join(pluginBasePath, highestVersion, 'skills', 'skill-rules.json');
    if (!existsSync(skillRulesPath)) return null;

    try {
        return JSON.parse(readFileSync(skillRulesPath, 'utf-8'));
    } catch {
        return null;
    }
}

/**
 * Merge multiple SkillRules objects. Later sources override earlier ones.
 */
function mergeSkillRules(...rulesSets: SkillRules[]): SkillRules {
    const merged: SkillRules = { version: '1.0', skills: {}, agents: {} };
    for (const rules of rulesSets) {
        if (rules.version) merged.version = rules.version;
        merged.skills = { ...merged.skills, ...rules.skills };
        merged.agents = { ...(merged.agents || {}), ...(rules.agents || {}) };
    }
    return merged;
}

async function main() {
    try {
        // Read input from stdin
        const input = readFileSync(0, 'utf-8');
        const data: HookInput = JSON.parse(input);
        const prompt = data.prompt.toLowerCase();

        // Load skill rules (merge global + plugins + project)
        const projectDir = process.env.CLAUDE_PROJECT_DIR || process.cwd();
        const homeDir = process.env.HOME || '';
        const projectRulesPath = join(projectDir, '.claude', 'skills', 'skill-rules.json');
        const globalRulesPath = join(homeDir, '.claude', 'skills', 'skill-rules.json');

        let globalRules: SkillRules = { version: '1.0', skills: {}, agents: {} };
        let projectRules: SkillRules = { version: '1.0', skills: {}, agents: {} };

        // Load global rules if exists
        if (existsSync(globalRulesPath)) {
            try {
                globalRules = JSON.parse(readFileSync(globalRulesPath, 'utf-8'));
            } catch (e) {
                // Invalid JSON, use empty
            }
        }

        // Load project rules if exists
        if (existsSync(projectRulesPath)) {
            try {
                projectRules = JSON.parse(readFileSync(projectRulesPath, 'utf-8'));
            } catch (e) {
                // Invalid JSON, use empty
            }
        }

        // Load enabled plugins and their skill-rules
        const enabledPlugins = loadEnabledPlugins(homeDir, projectDir);
        const pluginRulesList: SkillRules[] = [];
        for (const plugin of enabledPlugins) {
            const pluginRules = loadPluginSkillRules(homeDir, plugin);
            if (pluginRules) {
                pluginRulesList.push(pluginRules);
            }
        }

        // Exit if no rules sources exist
        const hasGlobalRules = existsSync(globalRulesPath);
        const hasProjectRules = existsSync(projectRulesPath);
        const hasPluginRules = pluginRulesList.length > 0;
        if (!hasGlobalRules && !hasProjectRules && !hasPluginRules) {
            process.exit(0);
        }

        // Merge: global -> plugins (in order) -> project (project overrides all)
        const rules = mergeSkillRules(globalRules, ...pluginRulesList, projectRules);

        const matchedSkills: MatchedSkill[] = [];

        // Check each skill for matches
        for (const [skillName, config] of Object.entries(rules.skills)) {
            const triggers = config.promptTriggers;
            if (!triggers) {
                continue;
            }

            // Keyword matching
            if (triggers.keywords) {
                const keywordMatch = triggers.keywords.some(kw =>
                    prompt.includes(kw.toLowerCase())
                );
                if (keywordMatch) {
                    matchedSkills.push({ name: skillName, matchType: 'keyword', config });
                    continue;
                }
            }

            // Intent pattern matching
            if (triggers.intentPatterns) {
                const intentMatch = triggers.intentPatterns.some(pattern => {
                    const regex = new RegExp(pattern, 'i');
                    return regex.test(prompt);
                });
                if (intentMatch) {
                    matchedSkills.push({ name: skillName, matchType: 'intent', config });
                }
            }
        }

        // Check each agent for matches
        const matchedAgents: MatchedSkill[] = [];
        if (rules.agents) {
            for (const [agentName, config] of Object.entries(rules.agents)) {
                const triggers = config.promptTriggers;
                if (!triggers) {
                    continue;
                }

                // Keyword matching
                if (triggers.keywords) {
                    const keywordMatch = triggers.keywords.some(kw =>
                        prompt.includes(kw.toLowerCase())
                    );
                    if (keywordMatch) {
                        matchedAgents.push({ name: agentName, matchType: 'keyword', config, isAgent: true });
                        continue;
                    }
                }

                // Intent pattern matching
                if (triggers.intentPatterns) {
                    const intentMatch = triggers.intentPatterns.some(pattern => {
                        const regex = new RegExp(pattern, 'i');
                        return regex.test(prompt);
                    });
                    if (intentMatch) {
                        matchedAgents.push({ name: agentName, matchType: 'intent', config, isAgent: true });
                    }
                }
            }
        }

        // Generate output if matches found
        if (matchedSkills.length > 0 || matchedAgents.length > 0) {
            let output = '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n';
            output += '🎯 SKILL ACTIVATION CHECK\n';
            output += '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n';

            // Group skills by priority
            const critical = matchedSkills.filter(s => s.config.priority === 'critical');
            const high = matchedSkills.filter(s => s.config.priority === 'high');
            const medium = matchedSkills.filter(s => s.config.priority === 'medium');
            const low = matchedSkills.filter(s => s.config.priority === 'low');

            if (critical.length > 0) {
                output += '⚠️ CRITICAL SKILLS (REQUIRED):\n';
                critical.forEach(s => output += `  → ${s.name}\n`);
                output += '\n';
            }

            if (high.length > 0) {
                output += '📚 RECOMMENDED SKILLS:\n';
                high.forEach(s => output += `  → ${s.name}\n`);
                output += '\n';
            }

            if (medium.length > 0) {
                output += '💡 SUGGESTED SKILLS:\n';
                medium.forEach(s => output += `  → ${s.name}\n`);
                output += '\n';
            }

            if (low.length > 0) {
                output += '📌 OPTIONAL SKILLS:\n';
                low.forEach(s => output += `  → ${s.name}\n`);
                output += '\n';
            }

            // Add matched agents
            if (matchedAgents.length > 0) {
                output += '🤖 RECOMMENDED AGENTS (token-efficient):\n';
                matchedAgents.forEach(a => output += `  → ${a.name}\n`);
                output += '\n';
            }

            if (matchedSkills.length > 0) {
                output += 'ACTION: Use Skill tool BEFORE responding\n';
            }
            if (matchedAgents.length > 0) {
                output += 'ACTION: Use Task tool with agent for exploration\n';
            }
            output += '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n';

            console.log(output);
        }

        process.exit(0);
    } catch (err) {
        console.error('Error in skill-activation-prompt hook:', err);
        process.exit(1);
    }
}

main().catch(err => {
    console.error('Uncaught error:', err);
    process.exit(1);
});
