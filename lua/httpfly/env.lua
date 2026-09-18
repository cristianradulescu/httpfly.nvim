local config = require("httpfly.config")
local session = require("httpfly.session")

local M = {}

-- env file path -> selected environment name, scoped per project
local selected = {}

-- applied as defaults to every environment in both the public and private
-- files, dollar-prefixed so it can't collide with a real environment
-- someone names "shared" (matches httpfly's internal/env.go)
local SHARED_KEY = "$shared"

local function read_json(path)
  local ok_read, content = pcall(vim.fn.readfile, path)
  if not ok_read then
    return {}
  end
  local ok_decode, decoded = pcall(vim.json.decode, table.concat(content, "\n"))
  if not ok_decode or type(decoded) ~= "table" then
    return {}
  end
  return decoded
end

local function dir_for_buf(bufnr)
  return vim.fn.fnamemodify(vim.api.nvim_buf_get_name(bufnr), ":h")
end

-- httpfly resolves http-client.env.json (and .httpfly/state.json) via its
-- own process cwd only, no upward search of its own -- so this plugin
-- instead does the upward search itself (from the .http file's own
-- directory to the filesystem root) to find where the env file actually
-- lives, and then always launches httpfly with that directory as cwd
-- (M.resolve_cwd() below). This lets a project keep one
-- http-client.env.json at its root while .http files live in
-- subdirectories (e.g. v1/request.http, v2/request.http) -- without this,
-- a real `httpfly run` from a subdir wouldn't see the root env file at
-- all. When no env file is found anywhere upward, cwd falls back to the
-- .http file's own directory, matching the plugin's previous behavior.
-- Only the public file (config.options.env_file) is searched for here --
-- the optional private overlay (config.options.private_env_file) is
-- always looked for alongside whichever directory this search finds,
-- never on its own (matching httpfly's own env.Load, which requires the
-- public file to exist and treats the private one as an optional sibling
-- in that same directory).
local function find_env_dir(dir)
  local found = vim.fs.find(config.options.env_file, { path = dir, upward = true })[1]
  if not found then
    return nil
  end
  return vim.fn.fnamemodify(found, ":h")
end

-- every top-level key across the public and private files, except
-- "$shared", which isn't itself a selectable environment (same as
-- httpfly's own CLI -- "-env $shared" is an error there). A name defined
-- only in the private file (e.g. a personal "local" environment) is a
-- valid choice too, same as httpfly's own resolution.
local function read_env_names(public_decoded, private_decoded)
  local names = {}
  local function collect(decoded)
    if type(decoded) ~= "table" then
      return
    end
    for k in pairs(decoded) do
      if k ~= SHARED_KEY then
        names[k] = true
      end
    end
  end
  collect(public_decoded)
  collect(private_decoded)

  local list = {}
  for k in pairs(names) do
    table.insert(list, k)
  end
  table.sort(list)
  return list
end

-- layers variables into `vars` in httpfly's own precedence order, lowest
-- to highest: public "$shared" < public <env> < private "$shared" <
-- private <env> -- so a private-file value always wins over a
-- public-file one, and each file's own environment entry still wins over
-- that same file's "$shared" defaults (matches httpfly's internal/env.go
-- Load()).
local function merge_env(vars, public, private, name)
  local function apply(decoded, key)
    if type(decoded) ~= "table" then
      return
    end
    local layer = decoded[key]
    if type(layer) == "table" then
      for k, v in pairs(layer) do
        vars[k] = v
      end
    end
  end
  apply(public, SHARED_KEY)
  apply(public, name)
  apply(private, SHARED_KEY)
  apply(private, name)
end

function M.env_file_for_buf(bufnr)
  bufnr = bufnr or 0
  local dir = dir_for_buf(bufnr)
  local env_dir = find_env_dir(dir)
  if not env_dir then
    return nil
  end
  return env_dir .. "/" .. config.options.env_file
end

-- the optional private overlay's path, alongside the public env file found
-- by env_file_for_buf() -- nil under the same conditions env_file_for_buf()
-- returns nil (no public env file found upward), since httpfly itself
-- never looks for the private file without the public one existing first.
function M.private_env_file_for_buf(bufnr)
  bufnr = bufnr or 0
  local dir = dir_for_buf(bufnr)
  local env_dir = find_env_dir(dir)
  if not env_dir then
    return nil
  end
  return env_dir .. "/" .. config.options.private_env_file
end

-- the directory httpfly itself must be launched with as cwd for this
-- buffer: the directory containing the env file found via the upward
-- search above, or the .http file's own directory if none was found.
-- runner.lua uses this same directory for -- and only for -- vim.system's
-- cwd, so http-client.env.json (plus its optional private overlay) and
-- .httpfly/state.json (all cwd-relative on httpfly's side) are read
-- from/written to the same place this plugin just looked in.
function M.resolve_cwd(bufnr)
  bufnr = bufnr or 0
  local dir = dir_for_buf(bufnr)
  return find_env_dir(dir) or dir
end

function M.get(bufnr)
  local env_file = M.env_file_for_buf(bufnr)
  if not env_file then
    return nil
  end
  return selected[env_file]
end

function M.set(name, bufnr)
  local env_file = M.env_file_for_buf(bufnr)
  if not env_file then
    vim.notify("httpfly: no " .. config.options.env_file .. " found for this file", vim.log.levels.WARN)
    return
  end
  selected[env_file] = name
  vim.notify("httpfly: environment set to '" .. name .. "'", vim.log.levels.INFO)
end

-- winbar text for the currently selected environment; meant to be called
-- from a winbar expression (e.g. via v:lua), so it defaults to buffer 0
function M.status(bufnr)
  local env_file = M.env_file_for_buf(bufnr or 0)
  if not env_file then
    return ""
  end
  local name = selected[env_file]
  return "env: " .. (name or "(none, :HttpEnv)")
end

-- merged variables (public "$shared"/env, then private "$shared"/env, then
-- session-persisted vars overriding those -- see merge_env() above for the
-- exact precedence) for the environment currently selected for this
-- buffer. `session_keys` is the set of keys present in the persisted
-- state; `env_keys` is the set of keys that already had a value from the
-- env files alone, before persisted state was applied -- the two together
-- let callers tell "session added a new var" apart from "session overrode
-- an existing env var"
function M.vars(bufnr)
  bufnr = bufnr or 0
  local env_file = M.env_file_for_buf(bufnr)
  if not env_file then
    return nil
  end
  local name = selected[env_file]
  if not name then
    return nil
  end

  local vars = {}
  merge_env(vars, read_json(env_file), read_json(M.private_env_file_for_buf(bufnr)), name)

  local env_keys = {}
  for k in pairs(vars) do
    env_keys[k] = true
  end

  local dir = M.resolve_cwd(bufnr)
  local session_vars = session.load(dir, name)
  local session_keys = {}
  for k, v in pairs(session_vars) do
    vars[k] = v
    session_keys[k] = true
  end

  return vars, name, session_keys, env_keys
end

-- plain-text lines in the same box-drawn style as the result pane (a
-- heavy-ruled title, then one entry per variable: its name, a
-- provenance marker if it came from persisted session state, and its
-- value on the indented line below), plus a { line0, col_start, col_end,
-- group } list of highlight spans for the caller to apply once the buffer
-- is populated. Byte offsets, as nvim_buf_add_highlight expects.
local function render_vars(name, vars, session_keys, env_keys)
  local keys = {}
  for k in pairs(vars) do
    table.insert(keys, k)
  end
  table.sort(keys)

  local title = " Environment: " .. name
  local rule = string.rep("━", math.max(vim.fn.strdisplaywidth(title) + 1, 20))
  local lines = { rule, title, rule, "" }
  local highlights = {
    { 0, 0, -1, "Comment" },
    { 1, 0, -1, "Title" },
    { 2, 0, -1, "Comment" },
  }

  for _, k in ipairs(keys) do
    local marker = ""
    if session_keys[k] then
      marker = env_keys[k] and "  (overridden by session)" or "  (session)"
    end
    table.insert(lines, k .. marker)
    table.insert(highlights, { #lines - 1, 0, #k, "Identifier" })
    if marker ~= "" then
      table.insert(highlights, { #lines - 1, #k, #k + #marker, "WarningMsg" })
    end
    -- value on its own indented line so long unbroken strings like JWTs
    -- wrap naturally in the window instead of pushing the name off-screen
    table.insert(lines, "  " .. tostring(vars[k]))
    table.insert(highlights, { #lines - 1, 0, -1, "String" })
    table.insert(lines, "")
  end

  if #keys == 0 then
    table.insert(lines, "(no variables)")
    table.insert(highlights, { #lines - 1, 0, -1, "Comment" })
  end

  return lines, highlights
end

function M.show_vars(bufnr)
  bufnr = bufnr or 0
  local vars, name, session_keys, env_keys = M.vars(bufnr)
  if not vars then
    vim.notify("httpfly: no environment selected (use :HttpEnv)", vim.log.levels.WARN)
    return
  end

  local lines, highlights = render_vars(name, vars, session_keys, env_keys)

  local width = math.min(90, math.floor(vim.o.columns * 0.85))
  local rows = 0
  for _, line in ipairs(lines) do
    rows = rows + math.max(1, math.ceil(math.max(vim.fn.strdisplaywidth(line), 1) / width))
  end
  local height = math.min(rows, math.floor(vim.o.lines * 0.7))

  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false
  vim.bo[buf].bufhidden = "wipe"
  vim.bo[buf].filetype = "text"

  local ns = vim.api.nvim_create_namespace("httpfly_env_vars")
  for _, h in ipairs(highlights) do
    vim.api.nvim_buf_add_highlight(buf, ns, h[4], h[1], h[2], h[3])
  end

  local win = vim.api.nvim_open_win(buf, true, {
    relative = "cursor",
    row = 1,
    col = 0,
    width = width,
    height = height,
    style = "minimal",
    border = "rounded",
    title = " env vars ",
    title_pos = "center",
  })
  vim.wo[win].wrap = true
  vim.wo[win].linebreak = false

  vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = buf, nowait = true, silent = true })
  vim.keymap.set("n", "<esc>", "<cmd>close<cr>", { buffer = buf, nowait = true, silent = true })
  vim.api.nvim_create_autocmd("BufLeave", {
    buffer = buf,
    once = true,
    callback = function()
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_win_close(win, true)
      end
    end,
  })
end

function M.pick(bufnr)
  bufnr = bufnr or 0
  local env_file = M.env_file_for_buf(bufnr)
  if not env_file then
    vim.notify("httpfly: no " .. config.options.env_file .. " found for this file", vim.log.levels.WARN)
    return
  end

  local names = read_env_names(read_json(env_file), read_json(M.private_env_file_for_buf(bufnr)))
  if #names == 0 then
    vim.notify("httpfly: no environments found in " .. env_file, vim.log.levels.WARN)
    return
  end

  vim.ui.select(names, { prompt = "Select httpfly environment:" }, function(choice)
    if not choice then
      return
    end
    selected[env_file] = choice
    vim.notify("httpfly: environment set to '" .. choice .. "'", vim.log.levels.INFO)
  end)
end

return M
