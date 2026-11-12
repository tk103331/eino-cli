'use strict';

// Best-effort binary download during `npm install`. Failure is non-fatal:
// the launcher retries the download on first run.
try {
  require('../lib/install').ensureBinarySync();
} catch (err) {
  console.warn(`[eino-cli] binary download skipped: ${err.message}`);
  console.warn('[eino-cli] it will be retried on first run');
}
