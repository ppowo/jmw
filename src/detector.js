const fs = require('fs');
const path = require('path');
const { XMLParser } = require('fast-xml-parser');

const parser = new XMLParser({
  ignoreAttributes: false,
  attributeNamePrefix: ''
});

/**
 * Detect project from current directory
 * Walks up the tree to find pom.xml and matches against configured projects
 */
function detectProject(config, cwd) {
  if (!cwd) {
    cwd = process.cwd();
  }

  const currentPath = path.resolve(cwd);

  // Find which project this path belongs to
  let matchedProject = null;
  for (const [projectName, projectConfig] of Object.entries(config.projects)) {
    if (currentPath.startsWith(projectConfig.base_path)) {
      matchedProject = { name: projectName, config: projectConfig };
      break;
    }
  }

  if (!matchedProject) {
    throw new Error('Current directory is not within any configured project');
  }

  // Walk up to find pom.xml
  const pomPath = findPomXml(currentPath);
  if (!pomPath) {
    throw new Error('No pom.xml found in current directory or parent directories');
  }

  // Parse POM
  const pom = parsePom(pomPath);

  // Detect module
  const moduleInfo = detectModule(pomPath, pom, matchedProject.config);

  return {
    project: matchedProject.name,
    projectConfig: matchedProject.config,
    pomPath,
    module: moduleInfo
  };
}

/**
 * Walk up directory tree to find pom.xml
 */
function findPomXml(startPath) {
  let currentDir = startPath;
  const rootDir = path.parse(currentDir).root;

  while (currentDir !== rootDir) {
    const pomPath = path.join(currentDir, 'pom.xml');
    if (fs.existsSync(pomPath)) {
      return pomPath;
    }
    currentDir = path.dirname(currentDir);
  }

  return null;
}

/**
 * Parse pom.xml file
 */
function parsePom(pomPath) {
  try {
    const content = fs.readFileSync(pomPath, 'utf8');
    return parser.parse(content);
  } catch (error) {
    throw new Error('Failed to parse pom.xml: ' + error.message);
  }
}

/**
 * Detect module information from POM
 */
function detectModule(pomPath, pom, projectConfig) {
  const artifactId = pom.project?.artifactId;
  const packaging = pom.project?.packaging || 'jar';

  if (!artifactId) {
    throw new Error('artifactId not found in pom.xml');
  }

  // Check if this is a global module
  const modulePath = path.dirname(pomPath);
  const relativePath = path.relative(projectConfig.base_path, modulePath);

  // Determine deployment type
  const moduleConfig = projectConfig.modules?.[artifactId];
  const isGlobalModule = moduleConfig && moduleConfig !== '';

  // Check if this is a multi-module project
  const modules = pom.project?.modules?.module || [];
  const isMultiModule = Array.isArray(modules) ? modules.length > 0 : !!modules;

  return {
    artifactId,
    packaging,
    path: modulePath,
    relativePath,
    isGlobalModule,
    deploymentPath: moduleConfig || '',
    isMultiModule,
    modules: isMultiModule ? (Array.isArray(modules) ? modules : [modules]) : []
  };
}

module.exports = {
  detectProject,
  parsePom,
  findPomXml,
  detectModule
};
