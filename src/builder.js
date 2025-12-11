const path = require('path');
const execa = require('execa');
const chalk = require('chalk');

/**
 * Build a Maven module
 */
async function buildModule(detection, profile, options = {}) {
  const { project, projectConfig, module: moduleInfo } = detection;
  const skipTests = options.skipTests || projectConfig.skip_tests || false;

  console.log(chalk.blue('=== Build Plan ==='));
  console.log('Project: ' + project);
  console.log('Module: ' + moduleInfo.artifactId);
  console.log('Packaging: ' + moduleInfo.packaging);
  console.log('Path: ' + moduleInfo.path);
  console.log('');

  // Show profile
  const effectiveProfile = profile || projectConfig.default_profile || 'none';
  console.log('Profile: ' + effectiveProfile);
  console.log('');

  // Build Maven command
  const cmdArgs = buildMavenCommand(moduleInfo, effectiveProfile, skipTests, projectConfig);

  console.log(chalk.yellow('Command:'), 'mvn', cmdArgs.join(' '));
  console.log('');

  // Confirm build
  const confirmed = await confirm('Proceed with build?');
  if (!confirmed) {
    console.log(chalk.red('Build cancelled'));
    return;
  }

  // Execute build
  try {
    const { stdout, stderr } = await execa('mvn', cmdArgs, {
      cwd: moduleInfo.isMultiModule ? projectConfig.base_path : moduleInfo.path,
      stdio: 'inherit'
    });

    console.log('');
    console.log(chalk.green('Build completed successfully'));

    // Show artifacts
    showArtifacts(moduleInfo);

  } catch (error) {
    console.error(chalk.red('Build failed:'), error.message);
    throw error;
  }
}

/**
 * Build Maven command arguments
 */
function buildMavenCommand(moduleInfo, profile, skipTests, projectConfig) {
  const args = [];

  // Phase
  args.push('install');

  // Profiles
  const profiles = getProfiles(profile, projectConfig);
  if (profiles.length > 0) {
    args.push('-P', profiles.join(','));
  }

  // Skip tests
  if (skipTests) {
    args.push('-DskipTests=true');
  }

  // Multi-module specific
  if (moduleInfo.isMultiModule) {
    args.push('-pl', moduleInfo.artifactId);
    args.push('-am'); // Also make
  }

  return args;
}

/**
 * Get Maven profiles for a project
 */
function getProfiles(profile, projectConfig) {
  if (!profile || profile === 'none') {
    return [];
  }

  // Check if profile is in available_profiles
  const available = projectConfig.available_profiles || [];
  if (available.length > 0 && !available.includes(profile)) {
    const msg = 'Profile \'' + profile + '\' not available. Available: ' + available.join(', ');
    throw new Error(msg);
  }

  // Check profile_overrides
  const overrides = projectConfig.profile_overrides;
  if (overrides && overrides[profile]) {
    return overrides[profile];
  }

  return [profile];
}

/**
 * Show built artifacts
 */
function showArtifacts(moduleInfo) {
  console.log(chalk.blue('=== Artifacts ==='));

  const targetPath = path.join(moduleInfo.path, 'target');
  const artifacts = findArtifacts(targetPath, moduleInfo.packaging);

  if (artifacts.length === 0) {
    console.log('No artifacts found');
    return;
  }

  artifacts.forEach(artifact => {
    console.log('  ' + chalk.green(artifact));
  });

  console.log('');
  console.log(chalk.blue('=== Restart Guidance ==='));
  console.log('Check if restart is required based on your deployment configuration.');
  console.log('');
}

/**
 * Find artifacts in target directory
 */
function findArtifacts(targetPath, packaging) {
  try {
    const fs = require('fs');
    if (!fs.existsSync(targetPath)) {
      return [];
    }

    return fs.readdirSync(targetPath)
      .filter(file => file.endsWith('.' + packaging))
      .map(file => path.join(targetPath, file));
  } catch (error) {
    return [];
  }
}

/**
 * Simple confirmation prompt
 */
function confirm(message) {
  return new Promise(resolve => {
    const readline = require('readline');
    const rl = readline.createInterface({
      input: process.stdin,
      output: process.stdout
    });

    rl.question(message + ' (y/N) ', answer => {
      rl.close();
      resolve(answer.toLowerCase() === 'y' || answer.toLowerCase() === 'yes');
    });
  });
}

module.exports = {
  buildModule,
  buildMavenCommand,
  getProfiles,
  showArtifacts,
  findArtifacts,
  confirm
};
