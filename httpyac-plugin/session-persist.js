// httpyac plugin (loaded via the HTTPYAC_PLUGIN env var) that makes
// client.global.set(...) / {{var}} survive across separate `httpyac send`
// process invocations, without touching http-client.env.json.
//
// httpyac already keeps global variables in an in-memory sessionStore
// (userSessionStore) for the lifetime of one process -- that's what lets
// `--all` share state across requests in a single run. This plugin just
// mirrors that store to a JSON file on disk: load it back in at startup,
// and persist it again on every change. httpyac's own {{var}} resolution
// and client.global API do the rest, completely unmodified.
const fs = require('fs');
const path = require('path');

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

function load(sessionStore) {
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

function save(sessionStore) {
  const globalSessions = sessionStore.userSessions.filter(isGlobalSession);
  const file = sessionFile();
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify(globalSessions, null, 2));
}

module.exports.configureHooks = function (api) {
  load(api.sessionStore);
  api.sessionStore.onSessionChanged(() => save(api.sessionStore));
};
