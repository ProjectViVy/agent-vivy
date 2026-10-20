#!/usr/bin/env node
import { doctor } from './verify-kernel.mjs';
try { doctor(); }
catch (error) {
  console.error(`[FAIL] ${error.message}`);
  console.log('doctor: 0 ok, 0 warnings, 1 failures');
  process.exitCode = 1;
}
