const fs = require('fs');
const path = require('path');
const chalk = require('chalk');

/**
 * Deploy artifact to WildFly
 */
async function deployArtifact(artifactPath, detection, clientName, clientConfig) {
  const { project, projectConfig, module: moduleInfo } = detection;

  console.log(chalk.blue('=== Deployment Plan ==='));
  console.log('Project: ' + project);
  console.log('Artifact: ' + artifactPath);
  console.log('Module: ' + moduleInfo.artifactId);
  console.log('Type: ' + (moduleInfo.isGlobalModule ? 'Global Module' : 'Normal Deployment'));
  console.log('');

  // Get WildFly configuration (local deployment)
  const wildflyConfig = getWildflyConfig(projectConfig, null);

  console.log(chalk.yellow('WildFly Root:'), wildflyConfig.root);
  console.log(chalk.yellow('Mode:'), wildflyConfig.mode);
  if (wildflyConfig.mode === 'domain') {
    console.log(chalk.yellow('Server Group:'), wildflyConfig.serverGroup);
  }
  console.log('');

  // Confirm deployment
  const confirmed = await confirm('Proceed with deployment?');
  if (!confirmed) {
    console.log(chalk.red('Deployment cancelled'));
    return;
  }

  // Execute deployment
  try {
    if (moduleInfo.isGlobalModule) {
      await deployGlobalModule(artifactPath, wildflyConfig, moduleInfo);
    } else {
      await deployNormal(artifactPath, wildflyConfig, moduleInfo);
    }

    console.log(chalk.green('Deployment completed'));

    // Show restart guidance
    showRestartGuidance(wildflyConfig);

    // Show remote deployment guide if configured (use default client)
    const defaultClientName = projectConfig.default_client;
    if (defaultClientName && projectConfig.clients && projectConfig.clients[defaultClientName]) {
      const defaultClient = projectConfig.clients[defaultClientName];
      console.log('');
      console.log(chalk.blue('=== Remote Deployment Instructions (Default Client: ' + defaultClientName + ') ==='));
      console.log('');
      showRemoteDeploymentGuide(artifactPath, wildflyConfig, defaultClient);
    }

  } catch (error) {
    console.error(chalk.red('Deployment failed:'), error.message);
    throw error;
  }
}

/**
 * Deploy global module to WildFly modules directory
 */
function deployGlobalModule(artifactPath, wildflyConfig, moduleInfo) {
  const modulesDir = path.join(wildflyConfig.root, 'modules');
  const modulePath = path.join(modulesDir, moduleInfo.deploymentPath, 'main');

  console.log(chalk.blue('=== Global Module Deployment ==='));
  console.log('Source: ' + artifactPath);
  console.log('Target: ' + modulePath);
  console.log('');

  // Copy artifact
  fs.mkdirSync(modulePath, { recursive: true });
  const destPath = path.join(modulePath, path.basename(artifactPath));
  fs.copyFileSync(artifactPath, destPath);

  console.log(chalk.green('Module deployed to: ' + destPath));
}

/**
 * Deploy to normal WildFly deployments
 */
function deployNormal(artifactPath, wildflyConfig, moduleInfo) {
  console.log(chalk.blue('=== Normal Deployment ==='));

  if (wildflyConfig.mode === 'standalone') {
    deployStandalone(artifactPath, wildflyConfig, moduleInfo);
  } else {
    deployDomain(artifactPath, wildflyConfig, moduleInfo);
  }
}

/**
 * Deploy to standalone mode
 */
function deployStandalone(artifactPath, wildflyConfig, moduleInfo) {
  const deploymentsDir = path.join(wildflyConfig.root, 'standalone', 'deployments');
  const destPath = path.join(deploymentsDir, path.basename(artifactPath));
  const markerPath = path.join(deploymentsDir, path.basename(artifactPath) + '.dodeploy');

  console.log('Target: ' + destPath);
  console.log('');

  // Copy artifact
  fs.mkdirSync(deploymentsDir, { recursive: true });
  fs.copyFileSync(artifactPath, destPath);

  // Create marker file
  fs.writeFileSync(markerPath, '');

  console.log(chalk.green('Deployed to: ' + destPath));
  console.log(chalk.green('Marker created: ' + markerPath));
}

/**
 * Deploy to domain mode
 */
function deployDomain(artifactPath, wildflyConfig, moduleInfo) {
  const artifactName = path.basename(artifactPath);
  const deploymentsDir = path.join(wildflyConfig.root, 'domain', 'deployments');

  console.log('Server Group: ' + wildflyConfig.serverGroup);
  console.log('Artifact: ' + artifactName);
  console.log('');
  console.log(chalk.yellow('Use jboss-cli.sh to deploy:'));
  console.log('  deploy ' + artifactPath + ' --server-groups=' + wildflyConfig.serverGroup);
  console.log('');

  // Copy to deployments dir (for reference)
  fs.mkdirSync(deploymentsDir, { recursive: true });
  const destPath = path.join(deploymentsDir, artifactName);
  fs.copyFileSync(artifactPath, destPath);
  console.log(chalk.green('Copied to: ' + destPath));
}

/**
 * Get WildFly configuration (local deployment)
 */
function getWildflyConfig(projectConfig, clientConfig) {
  const config = {
    root: projectConfig.wildfly_root,
    mode: projectConfig.wildfly_mode || 'standalone',
    serverGroup: projectConfig.server_group
  };

  return config;
}

/**
 * Show restart guidance
 */
function showRestartGuidance(wildflyConfig) {
  console.log(chalk.blue('=== Restart Guidance ==='));

  const isGlobalModule = wildflyConfig.globalModule;

  if (isGlobalModule) {
    console.log(chalk.red('Restart required: YES'));
    console.log('Global modules require WildFly restart.');
  } else {
    console.log(chalk.yellow('Restart required: Check deployment'));
    console.log('Normal deployments may not require restart.');
  }

  console.log('');
  console.log(chalk.yellow('Restart command:'));

  if (wildflyConfig.mode === 'standalone') {
    console.log('  ' + wildflyConfig.root + '/bin/shutdown.sh --restart');
  } else {
    console.log('  ' + wildflyConfig.root + '/bin/domain.sh --restart');
  }

  console.log('');
}

/**
 * Show remote deployment guide
 */
function showRemoteDeploymentGuide(artifactPath, wildflyConfig, clientConfig) {
  const remote = clientConfig.remote;
  const artifactName = path.basename(artifactPath);

  console.log(chalk.yellow('SCP command:'));
  console.log('  scp ' + artifactPath + ' ' + remote.user + '@' + remote.host + ':~/');
  console.log('');
  console.log(chalk.yellow('SSH commands:'));
  console.log('  # Copy to WildFly');
  console.log('  ssh ' + remote.user + '@' + remote.host + ' "sudo cp ~/' + artifactName + ' ' + remote.wildfly_path + '/' + wildflyConfig.mode + '/deployments/"');
  console.log('');
  console.log('  # Restart WildFly');
  console.log('  ssh ' + remote.user + '@' + remote.host + ' "' + remote.restart_cmd + '"');
  console.log('');
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
  deployArtifact,
  getWildflyConfig,
  deployGlobalModule,
  deployNormal,
  deployStandalone,
  deployDomain,
  showRestartGuidance,
  showRemoteDeploymentGuide,
  confirm
};
