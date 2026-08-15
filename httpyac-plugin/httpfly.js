// httpyac plugin loaded via the HTTPYAC_PLUGIN env var on every send.
// Two independent features live here (kept as separate registration
// functions below, both called from configureHooks) because httpyac only
// supports one HTTPYAC_PLUGIN path at a time -- there's nowhere else to
// put a second plugin file without also registering a project-local
// .httpyac.js, which would be one more thing for users to manage.
const fs = require('fs');
const path = require('path');

// ---------------------------------------------------------------------
// Session persistence: makes client.global.set(...) / {{var}} survive
// across separate `httpyac send` process invocations, without touching
// http-client.env.json.
//
// httpyac already keeps global variables in an in-memory sessionStore
// (userSessionStore) for the lifetime of one process -- that's what lets
// `--all` share state across requests in a single run. This mirrors that
// store to a JSON file on disk: load it back in at startup, and persist
// it again on every change. httpyac's own {{var}} resolution and
// client.global API do the rest, completely unmodified.
// ---------------------------------------------------------------------

const SESSION_FILENAME = '.httpfly/session.json';

// httpyac's own process cwd is set to the env file's directory (needed so
// httpyac can find http-client.env.json when the .http file being sent
// lives in a subdirectory below it) -- that's not necessarily where the
// user wants session state to live. HTTPFLY_SESSION_FILE, when set,
// overrides the default of "next to wherever httpyac's cwd happens to be".
function sessionFile() {
  return process.env.HTTPFLY_SESSION_FILE || path.join(process.cwd(), SESSION_FILENAME);
}

// only global-variable-cache sessions are persisted; httpyac's sessionStore
// also holds transient per-connection sessions (streaming, last response)
// that aren't JSON-serializable (they hold live sockets) and aren't
// relevant to variable persistence anyway
function isGlobalSession(session) {
  return typeof session.type === 'string' && session.type.endsWith('global_cache');
}

function loadSession(sessionStore) {
  let sessions;
  try {
    sessions = JSON.parse(fs.readFileSync(sessionFile(), 'utf8'));
  } catch (e) {
    return;
  }
  for (const session of sessions) {
    sessionStore.setUserSession(session);
  }
}

function saveSession(sessionStore) {
  const globalSessions = sessionStore.userSessions.filter(isGlobalSession);
  const file = sessionFile();
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify(globalSessions, null, 2));
}

function registerSessionPersistence(api) {
  loadSession(api.sessionStore);
  api.sessionStore.onSessionChanged(() => saveSession(api.sessionStore));
}

// ---------------------------------------------------------------------
// Downloads: "# @download" on a request saves the raw response body to
// disk, byte-perfect -- including genuinely binary content, which a
// "> {% ... %}" script's response.body can't do reliably (httpyac decodes
// it to a JS string somewhere in its own pipeline before any script ever
// sees it, which is lossy for arbitrary bytes; see doc/examples/9_binary_
// download.http for how that was confirmed). response.rawBody (a real,
// undecoded Buffer) does exist -- just not by the time an inline script
// runs. It's still there one step earlier, in the onResponse hook, which
// is the only reason this needs to be a plugin rather than something
// expressible inline in a .http file.
//
// "# @download" (bare) saves under a filename picked automatically, in
// the same priority order browsers use: Content-Disposition's filename,
// then the URL's last path segment if it looks like a real filename, then
// a content-type-guessed extension. "# @download name.ext" overrides that
// with an explicit filename.
// ---------------------------------------------------------------------

const DOWNLOAD_DIRNAME = '.httpfly/downloads';

// httpfly.nvim's Lua renderers (lua/httpfly/format/shared.lua's
// DOWNLOAD_MARKER, kept in sync with this string) look for a testResult
// whose message starts with this prefix and pull it into its own
// "Download" section instead of "Test Results". This is the only way for
// an onResponse hook to get information into httpyac's --json output at
// all: setting arbitrary extra fields on `response` doesn't survive --json
// serialization (confirmed empirically -- httpyac serializes a fixed
// known shape, not the object as given), but httpRegion.testResults is a
// plain array that real test results also flow through, and that array's
// contents do survive.
const DOWNLOAD_MARKER = 'httpfly:download:';

// same HTTPYAC_PLUGIN-process-cwd-vs-user's-actual-project-cwd mismatch as
// the session file above, same fix: an env var override, set by
// runner.lua to match wherever .httpfly/history and .httpfly/session.json
// already live.
function downloadDir() {
  return process.env.HTTPFLY_DOWNLOAD_DIR || path.join(process.cwd(), DOWNLOAD_DIRNAME);
}

function pickFilename(response) {
  const contentDisposition = response.headers && response.headers['content-disposition'];
  if (contentDisposition) {
    const match = /filename\*?=(?:UTF-8'')?"?([^";]+)"?/i.exec(contentDisposition);
    if (match) {
      return decodeURIComponent(match[1]);
    }
  }

  const url = response.request && response.request.url;
  if (url) {
    const lastSegment = url.split('?')[0].split('/').pop();
    if (lastSegment && lastSegment.includes('.')) {
      return lastSegment;
    }
  }

  const contentType = (response.headers && response.headers['content-type']) || '';
  const extension = contentType.split('/')[1] ? contentType.split('/')[1].split(';')[0].trim() : 'bin';
  return 'download.' + extension;
}

function registerDownloads(api) {
  api.hooks.onResponse.addHook('httpflyDownload', (response, context) => {
    const download = context.httpRegion && context.httpRegion.metaData && context.httpRegion.metaData.download;
    if (download === undefined) {
      return;
    }
    const requested = typeof download === 'string' ? download : pickFilename(response);
    // `requested` may come straight from the HTTP response
    // (Content-Disposition or the URL, via pickFilename) -- treat it as
    // untrusted regardless of how trusted the .http file itself is, since
    // it reflects whatever a malicious/compromised server chooses to send.
    // path.basename() strips directory components, but doesn't neutralize
    // a bare ".." (path.join(dir, "..") still escapes to the parent) --
    // the resolved-path prefix check below catches that and any other
    // remaining traversal.
    const dir = downloadDir();
    const filename = path.basename(requested);
    const dest = path.join(dir, filename);
    const resolvedDirPrefix = path.resolve(dir) + path.sep;
    if (!path.resolve(dest).startsWith(resolvedDirPrefix)) {
      throw new Error(`httpfly: refusing to save download outside ${dir}: ${JSON.stringify(requested)}`);
    }
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(dest, response.rawBody);

    if (!context.httpRegion.testResults) {
      context.httpRegion.testResults = [];
    }
    context.httpRegion.testResults.push({
      message: DOWNLOAD_MARKER + dest,
      status: 'SUCCESS',
    });
  });
}

module.exports.configureHooks = function (api) {
  registerSessionPersistence(api);
  registerDownloads(api);
};
