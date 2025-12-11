const yaml = require('js-yaml');
const fs = require('fs');
const path = require('path');
const os = require('os');

/**
 * Load and parse config.yaml
 * Expands ~ paths to home directory
 * Supports client parameter for remote deployments
 */
function loadConfig(configPath) {
  if (!configPath) {
    configPath = path.join(os.homedir(), '.config', 'jmw', 'config.yaml');
  }

  try {
    // Try user config first
    if (fs.existsSync(configPath)) {
      const doc = yaml.load(fs.readFileSync(configPath, 'utf8'));
      return expandPaths(doc);
    }

    // Fall back to config.yaml in project directory
    const localConfig = path.join(process.cwd(), 'config.yaml');
    if (fs.existsSync(localConfig)) {
      const doc = yaml.load(fs.readFileSync(localConfig, 'utf8'));
      return expandPaths(doc);
    }

    throw new Error('Config file not found');
  } catch (error) {
    throw new Error('Failed to load config: ' + error.message);
  }
}

/**
 * Expand ~ paths to home directory
 */
function expandPaths(obj) {
  if (!obj || typeof obj !== 'object') return obj;

  if (Array.isArray(obj)) {
    return obj.map(expandPaths);
  }

  const expanded = {};
  for (const [key, value] of Object.entries(obj)) {
    if (typeof value === 'string' && value.startsWith('~')) {
      expanded[key] = value.replace('~', os.homedir());
    } else if (typeof value === 'object') {
      expanded[key] = expandPaths(value);
    } else {
      expanded[key] = value;
    }
  }
  return expanded;
}

/**
 * Get client configuration for a project
 */
function getClientConfig(project, clientName) {
  if (!clientName) return null;

  if (!project.clients || !project.clients[clientName]) {
    const available = project.clients ? Object.keys(project.clients).join(', ') : 'none';
    throw new Error('Client \'' + clientName + '\' not found. Available clients: ' + available);
  }

  return project.clients[clientName];
}

module.exports = {
  loadConfig,
  getClientConfig,
  expandPaths
};
