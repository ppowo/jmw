const { Command } = require('commander');
const chalk = require('chalk');

const { loadConfig, getClientConfig } = require('./config');
const { detectProject } = require('./detector');
const { buildModule } = require('./builder');
const { deployArtifact, getWildflyConfig, showRemoteDeploymentGuide } = require('./deployer');

const program = new Command();

/**
 * Main entry point
 */
program
  .name('jmw')
  .description('Java Maven WildFly - Interactive deployment helper')
  .version('2.0.0');

/**
 * Build command
 */
program
  .command('build')
  .description('Build a Maven module')
  .argument('[profile]', 'Maven profile (e.g., TEST, PROD)')
  .option('--client <name>', 'Target client (shows remote deployment commands after build)')
  .option('--skip-tests', 'Skip tests during build')
  .action(async (profile, options) => {
    try {
      console.log(chalk.blue.bold('\n=== JMW Build ===\n'));

      // Load config
      const config = loadConfig();

      // Detect project
      const detection = detectProject(config);

      // Get client config if specified
      const clientConfig = options.client
        ? getClientConfig(detection.projectConfig, options.client)
        : (detection.projectConfig.default_client
            ? getClientConfig(detection.projectConfig, detection.projectConfig.default_client)
            : null);

      if (options.client && !clientConfig) {
        console.error(chalk.red('Client \'' + options.client + '\' not found'));
        process.exit(1);
      }

      console.log(chalk.green('Detected project: ' + detection.project));
      console.log(chalk.green('Module: ' + detection.module.artifactId));

      if (options.client) {
        console.log(chalk.green('Client: ' + options.client));
      } else if (detection.projectConfig.default_client) {
        console.log(chalk.green('Client: ' + detection.projectConfig.default_client + ' (default)'));
      }

      console.log('');

      // Build
      await buildModule(detection, profile, { skipTests: options.skipTests });

      // Show remote deployment guide if client configured
      if (clientConfig) {
        const artifactPath = 'path/to/artifact'; // This would be the actual built artifact
        console.log('');
        console.log(chalk.blue('=== Remote Deployment Commands ==='));
        console.log('');
        const wildflyConfig = getWildflyConfig(detection.projectConfig, clientConfig);
        showRemoteDeploymentGuide(artifactPath, wildflyConfig, clientConfig);
      }

      console.log(chalk.blue.bold('\n=== Build Complete ===\n'));

    } catch (error) {
      console.error(chalk.red('\nError: ' + error.message + '\n'));
      process.exit(1);
    }
  });

/**
 * Deploy command
 */
program
  .command('deploy')
  .description('Deploy artifact to WildFly')
  .argument('<artifact>', 'Path to artifact JAR/WAR file')
  .action(async (artifact, options) => {
    try {
      console.log(chalk.blue.bold('\n=== JMW Deploy ===\n'));

      // Load config
      const config = loadConfig();

      // Detect project
      const detection = detectProject(config);

      // Validate artifact path
      const fs = require('fs');
      if (!fs.existsSync(artifact)) {
        throw new Error('Artifact not found: ' + artifact);
      }

      console.log(chalk.green('Detected project: ' + detection.project));
      console.log(chalk.green('Module: ' + detection.module.artifactId));
      console.log(chalk.green('Artifact: ' + artifact));
      console.log('');

      // Deploy
      await deployArtifact(artifact, detection, null, null);

      console.log(chalk.blue.bold('\n=== Deploy Complete ===\n'));

    } catch (error) {
      console.error(chalk.red('\nError: ' + error.message + '\n'));
      process.exit(1);
    }
  });

/**
 * Show clients command
 */
program
  .command('clients')
  .description('Show available clients for current project')
  .action(() => {
    try {
      console.log(chalk.blue.bold('\n=== Available Clients ===\n'));

      const config = loadConfig();
      const detection = detectProject(config);

      const clients = detection.projectConfig.clients;

      if (!clients || Object.keys(clients).length === 0) {
        console.log(chalk.yellow('No clients configured for this project'));
        console.log('');
        return;
      }

      Object.entries(clients).forEach(([name, client]) => {
        const label = chalk.white.bold(name);
        const remote = client.remote ? client.remote.user + '@' + client.remote.host : 'No remote config';
        console.log('  ' + label + ': ' + remote);
      });

      if (detection.projectConfig.default_client) {
        console.log('');
        console.log('Default client: ' + chalk.green(detection.projectConfig.default_client));
      }

      console.log('');

    } catch (error) {
      console.error(chalk.red('\nError: ' + error.message + '\n'));
      process.exit(1);
    }
  });

/**
 * Show help on error
 */
const helpText = `
Examples:

  $ jmw build
  $ jmw build TEST
  $ jmw build TEST --client metrocargo
  $ jmw deploy ./target/myapp.jar
  $ jmw clients

For more information: https://github.com/ppowo/jmw
`;

program.addHelpText('after', helpText);

program.parse();
