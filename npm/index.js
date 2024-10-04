#!/usr/bin/env node

const { execFile } = require('child_process');
const path = require('path');
const os = require('os');

// Gen binary file depends on platform
function getBinaryPath() {
  const platform = os.platform();
  let binaryPath;

  if (platform === 'linux') {
    binaryPath = path.join(__dirname, 'bin', 'generate-types-linux');
  } else if (platform === 'darwin') {
    binaryPath = path.join(__dirname, 'bin', 'generate-types-macos');
  } else if (platform === 'win32') {
    binaryPath = path.join(__dirname, 'bin', 'generate-types-windows.exe');
  } else {
    throw new Error(`Unsupported platform: ${platform}`);
  }

  return binaryPath;
}

const binary = getBinaryPath();

// Add params into binary file
const args = process.argv.slice(2);

// Run binary
execFile(binary, args, (error, stdout, stderr) => {
  if (error) {
    console.error(`Error: ${error.message}`);
    return;
  }
  if (stderr) {
    console.error(`stderr: ${stderr}`);
    return;
  }
  console.log(stdout);
});
