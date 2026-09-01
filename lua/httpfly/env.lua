local config = require("httpfly.config")
local session = require("httpfly.session")

local M = {}

-- env file path -> selected environment name, scoped per project
local selected = {}

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

-- httpfly resolves httpfly.env.json (and .httpfly/state.json) via its own
-- process cwd only, no upward search of its own -- so this plugin instead
-- does the upward search itself (from the .http file's own directory to
-- the filesystem root) to find where the env file actually lives, and then
-- always launches httpfly with that directory as cwd (M.resolve_cwd()
-- below). This lets a project keep one httpfly.env.json at its root while
-- .http files live in subdirectories (e.g. v1/request.http,
-- v2/request.http) -- without this, a real `httpfly run` from a subdir
-- wouldn't see the root env file at all. When no env file is found
-- anywhere upward, cwd falls back to the .http file's own directory,
-- matching the plugin's previous behavior.
local function find_env_dir(dir)
  local found = vim.fs.find(config.options.env_file, { path = dir, upward = true })[1]
  if not found then
    return nil
  end
  return vim.fn.fnamemodify(found, ":h")
end

-- environment names declared under the file's "environments" key; "shared"
-- isn't itself a selectable environment, same as httpfly's own CLI ("-env
-- shared" is an error there)
local function read_env_names(env_file)
  local decoded = read_json(env_file)
  local environments = decoded.environments
  if type(environments) ~= "table" then
    return {}
  end
  local names = {}
  for k in pairs(environments) do
    table.insert(names, k)
  end
  table.sort(names)
  return names
end

local function merge_env(vars, decoded, name)
  if type(decoded) ~= "table" then
    return
  end
  local shared = decoded.shared
  if type(shared) == "table" then
    for k, v in pairs(shared) do
      vars[k] = v
    end
  end
  local environments = decoded.environments
  local env = type(environments) == "table" and environments[name]
  if type(env) == "table" then
    for k, v in pairs(env) do
      vars[k] = v
    end
  end
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

-- the directory httpfly itself must be launched with as cwd for this
-- buffer: the directory containing the env file found via the upward
-- search above, or the .http file's own directory if none was found.
-- runner.lua uses this same directory for -- and only for -- vim.system's
-- cwd, so httpfly.env.json and .httpfly/state.json (both cwd-relative on
-- httpfly's side) are read from/written to the same place this plugin
-- just looked in.
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

-- merged variables ("shared" + selected environment, then
-- session-persisted vars overriding those) for the environment currently
-- selected for this buffer. `session_keys` is the set of keys present in
-- the persisted state; `env_keys` is the set of keys that already had a
-- value from the env file alone, before persisted state was applied -- the
-- two together let callers tell "session added a new var" apart from
-- "session overrode an existing env var"
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
  merge_env(vars, read_json(env_file), name)

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

-- markdown text plus a { line, col_start, col_end } list marking where the
-- session-provenance marker (if any) sits on that heading line, so the
-- caller can highlight it once the buffer is populated
local function render_vars_markdown(name, vars, session_keys, env_keys)
  local keys = {}
  for k in pairs(vars) do
    table.insert(keys, k)
  end
  table.sort(keys)

  local lines = { "# Environment: " .. name, "" }
  local highlights = {}

  for _, k in ipairs(keys) do
    local heading = "## `" .. k .. "`"
    local marker = ""
    if session_keys[k] then
      marker = env_keys[k] and " _(overridden by session)_" or " _(session)_"
    end
    table.insert(lines, heading .. marker)
    if marker ~= "" then
      table.insert(highlights, { #lines - 1, #heading, #heading + #marker })
    end
    table.insert(lines, "")
    -- value on its own line/paragraph (not a code fence, not a table cell)
    -- so long unbroken strings like JWTs wrap naturally in the window
    -- instead of overflowing or breaking table rendering
    table.insert(lines, tostring(vars[k]))
    table.insert(lines, "")
  end

  if #keys == 0 then
    table.insert(lines, "_(no variables)_")
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

  local lines, highlights = render_vars_markdown(name, vars, session_keys, env_keys)

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
  vim.bo[buf].filetype = "markdown"

  local ns = vim.api.nvim_create_namespace("httpfly_env_vars")
  for _, h in ipairs(highlights) do
    vim.api.nvim_buf_add_highlight(buf, ns, "WarningMsg", h[1], h[2], h[3])
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

  local names = read_env_names(env_file)
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
