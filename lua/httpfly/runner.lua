local config = require("httpfly.config")
local env = require("httpfly.env")
local format = require("httpfly.format")
local shared = require("httpfly.format.shared")
local preview = require("httpfly.preview")
local history = require("httpfly.history")
local session = require("httpfly.session")

local M = {}

local result_buf_name = "httpfly://result"
local hl_ns = vim.api.nvim_create_namespace("httpfly_result")

local function open_result_buf()
  local buf = vim.fn.bufnr(result_buf_name)
  local win
  for _, w in ipairs(vim.api.nvim_list_wins()) do
    if vim.api.nvim_win_get_buf(w) == buf then
      win = w
      break
    end
  end

  if buf == -1 then
    vim.cmd("vsplit")
    win = vim.api.nvim_get_current_win()
    buf = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_name(buf, result_buf_name)
    vim.bo[buf].buftype = "nofile"
    vim.bo[buf].swapfile = false
    vim.bo[buf].bufhidden = "hide"
    vim.api.nvim_win_set_buf(win, buf)
    vim.wo[win].number = false
    vim.wo[win].relativenumber = false
    vim.keymap.set("n", config.options.preview_keymap, function()
      preview.show(buf)
    end, { buffer = buf, desc = "httpfly: show full value under cursor" })
    vim.keymap.set(
      "n",
      "q",
      "<cmd>close<cr>",
      { buffer = buf, nowait = true, silent = true, desc = "httpfly: close result window" }
    )
  elseif not win then
    vim.cmd("vsplit")
    win = vim.api.nvim_get_current_win()
    vim.api.nvim_win_set_buf(win, buf)
    vim.wo[win].number = false
    vim.wo[win].relativenumber = false
  end

  return buf
end

-- quotes a command's args for copy-pasting into a POSIX shell
local function shell_quote(cmd)
  local parts = {}
  for _, arg in ipairs(cmd) do
    if arg:match("[^%w%-%.%_%/%:%@%%,]") then
      arg = "'" .. arg:gsub("'", "'\\''") .. "'"
    end
    table.insert(parts, arg)
  end
  return table.concat(parts, " ")
end

-- writes rendered `lines` (or a raw-output fallback) into the result
-- buffer and saves history; shared by run() (one invocation) and
-- run_many() (several invocations stitched into one combined view, for
-- "@download" -- see below)
local function present(buf, cwd, cmd_str, lines, truncations, highlights, raw_fallback)
  if not vim.api.nvim_buf_is_valid(buf) then
    return
  end

  local filetype = config.options.output_style == "unicode" and "text" or "markdown"
  local history_ext = config.options.output_style == "unicode" and "txt" or "md"

  if lines then
    history.save(lines, history_ext, cwd)
  else
    -- fall back to raw output (e.g. httpfly crashed before emitting JSON)
    filetype = "httpresult"
    lines = raw_fallback
  end

  preview.set(buf, truncations or {})

  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false
  vim.bo[buf].filetype = filetype

  vim.api.nvim_buf_clear_namespace(buf, hl_ns, 0, -1)
  for _, h in ipairs(highlights or {}) do
    vim.api.nvim_buf_add_highlight(buf, hl_ns, h[4], h[1] - 1, h[2], h[3])
  end
end

local function raw_fallback_lines(cmd_str, stdout, stderr, code)
  local lines = { "Command: " .. cmd_str, "" }
  if stdout and #stdout > 0 then
    vim.list_extend(lines, vim.split(stdout, "\n"))
  end
  if code and code ~= 0 then
    table.insert(lines, "")
    table.insert(lines, "--- exit code: " .. code .. " ---")
    if stderr and #stderr > 0 then
      vim.list_extend(lines, vim.split(stderr, "\n"))
    end
  end
  return lines
end

local function run(cmd, cwd)
  local buf = open_result_buf()
  local cmd_str = shell_quote(cmd)

  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "Running: " .. cmd_str, "" })
  vim.bo[buf].modifiable = false

  vim.system(cmd, { cwd = cwd, text = true }, function(res)
    vim.schedule(function()
      local lines, truncations, highlights
      if res.stdout then
        lines, truncations, highlights = format.render(res.stdout, cmd_str)
      end
      present(buf, cwd, cmd_str, lines, truncations, highlights, raw_fallback_lines(cmd_str, res.stdout, res.stderr, res.code))
    end)
  end)
end

-- runs several "httpfly run -name X ..." invocations, one per request, in
-- file order, and stitches their JSON results into one combined array
-- before rendering -- needed because httpfly's own "-download" flag only
-- ever applies to a single selected request (see build_cmds()/"@download"
-- below), so a file mixing download and non-download requests can't be
-- sent as one "httpfly run <file>" call the way M.send_all() normally
-- does. Sequential separate invocations still see each other's persisted
-- client.global state correctly (httpfly writes it through to disk
-- immediately, the same mechanism that already makes chaining work across
-- separate :HttpSend calls), so this is functionally equivalent to one
-- process for the whole file, just slower.
local function run_many(cmds, cwd)
  local buf = open_result_buf()
  local cmd_strs = {}
  for _, cmd in ipairs(cmds) do
    table.insert(cmd_strs, shell_quote(cmd))
  end
  local combined_cmd_str = table.concat(cmd_strs, "\n")

  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "Running " .. #cmds .. " request(s)...", "" })
  vim.bo[buf].modifiable = false

  local combined = {}
  local raw_chunks = {}
  local last_stderr, last_code

  local step
  step = function(i)
    if i > #cmds then
      local lines, truncations, highlights
      if #combined > 0 then
        lines, truncations, highlights = format.render_decoded(combined, combined_cmd_str)
      end
      local fallback = { "Command:", "" }
      vim.list_extend(fallback, vim.split(combined_cmd_str, "\n"))
      table.insert(fallback, "")
      vim.list_extend(fallback, raw_chunks)
      if last_code and last_code ~= 0 then
        table.insert(fallback, "")
        table.insert(fallback, "--- exit code: " .. last_code .. " ---")
        if last_stderr and #last_stderr > 0 then
          vim.list_extend(fallback, vim.split(last_stderr, "\n"))
        end
      end
      present(buf, cwd, combined_cmd_str, lines, truncations, highlights, fallback)
      return
    end

    vim.system(cmds[i], { cwd = cwd, text = true }, function(res)
      vim.schedule(function()
        local stdout = res.stdout or ""
        vim.list_extend(raw_chunks, vim.split(stdout, "\n"))
        last_stderr, last_code = res.stderr, res.code
        local ok, decoded = pcall(vim.json.decode, shared.extract_json(stdout))
        if ok and type(decoded) == "table" then
          for _, item in ipairs(decoded) do
            table.insert(combined, item)
          end
        end
        step(i + 1)
      end)
    end)
  end

  step(1)
end

local function build_cmd(file, name_filter, download_path)
  local cmd = { config.options.cmd, "run", "-json" }

  local e = env.get(0)
  if e then
    vim.list_extend(cmd, { "-env", e })
  end
  if download_path then
    vim.list_extend(cmd, { "-download", download_path })
  end
  if name_filter then
    vim.list_extend(cmd, { "-name", name_filter })
  end
  table.insert(cmd, file)

  return cmd
end

-- checked on every send rather than once at startup, so a `cmd` changed via
-- a live setup() call, or a binary installed mid-session, is picked up
-- immediately; vim.fn.executable() resolves both bare names (via $PATH) and
-- absolute/relative paths, matching how vim.system() itself would look it up
local function ensure_binary()
  if vim.fn.executable(config.options.cmd) == 0 then
    vim.notify(
      "httpfly: '"
        .. config.options.cmd
        .. "' not found. Install it (see https://github.com/cristianradulescu/httpfly) "
        .. "or set `cmd` in httpfly.setup({ cmd = ... }) to its full path.",
      vim.log.levels.ERROR
    )
    return false
  end
  return true
end

local function require_file()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("httpfly: buffer has no file", vim.log.levels.WARN)
    return nil
  end
  -- httpfly reads the file from disk, not the buffer, so an unsaved edit
  -- would otherwise silently send the stale on-disk version.
  if vim.bo.modified then
    vim.cmd("write")
  end
  return file
end

-- parses every request block in the buffer, mirroring httpfly's own
-- parser: the file is split on lines starting with "###" -- literally, no
-- leading whitespace allowed, matching httpfly's strings.HasPrefix -- with
-- the segment before the first one being the prelude, which never carries
-- a @name. Since httpfly v0.3.0, trailing text on that "###" line itself
-- is shorthand for the block's @name ("### GetUsers" is equivalent to a
-- bare "###" followed by "# @name GetUsers"), so that's checked first;
-- an explicit "# @name" line later in the same block (still mandatory if
-- the separator line carries no name) overrides it, matching httpfly's own
-- rule that the two must agree when both are present. For each block, this
-- also pulls out its request line's URL (for a download filename guess),
-- and an "# @download" / "# @download some-name.ext" annotation if
-- present -- this plugin's own way of marking a request's response for
-- saving, since httpfly itself has no per-request annotation for that (its
-- "-download" is a plain "run" flag, not something a request declares --
-- see resolve_download_path() below). Blocks are returned in file order.
local function parse_blocks(bufnr)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)

  local boundaries = { { line = 0 } }
  for i, line in ipairs(lines) do
    local trailing = line:match("^###(.*)$")
    if trailing then
      trailing = vim.trim(trailing)
      table.insert(boundaries, { line = i, name = trailing ~= "" and trailing or nil })
    end
  end
  table.insert(boundaries, { line = #lines + 1 })

  local blocks = {}
  for idx = 1, #boundaries - 1 do
    local start_line = boundaries[idx].line + 1
    local end_line = boundaries[idx + 1].line - 1

    local name, url, download = boundaries[idx].name, nil, nil
    for i = start_line, end_line do
      local l = lines[i]
      if l then
        local n = l:match("^%s*#%s*@name%s+(.-)%s*$")
        if n and n ~= "" then
          name = n
        end
        if not url then
          local u = l:match("^%u+%s+(%S+)%s+HTTP/[%d%.]+%s*$") or l:match("^%u+%s+(%S+)%s*$")
          if u then
            url = u
          end
        end
        if download == nil then
          if l:match("^%s*#%s*@download%s*$") then
            download = true
          else
            local d = l:match("^%s*#%s*@download%s+(%S.-)%s*$")
            if d then
              download = d
            end
          end
        end
      end
    end

    table.insert(blocks, { start_line = start_line, end_line = end_line, name = name, url = url, download = download })
  end

  return blocks
end

local function find_enclosing_block(bufnr, cursor_line)
  for _, b in ipairs(parse_blocks(bufnr)) do
    if cursor_line >= b.start_line and cursor_line <= b.end_line then
      return b
    end
  end
  return nil
end

-- last path segment of a URL (query/fragment stripped), for suggesting a
-- download filename when "@download" was given bare. httpfly's own
-- "-download" flag needs the destination path upfront, before the request
-- is even sent, so unlike a browser (or the old httpyac-backed version of
-- this plugin) there's no way to name the file from the *response*
-- (Content-Disposition, actual Content-Type) -- only the URL is available
-- at this point.
local function guess_filename(url)
  if not url then
    return "download"
  end
  local path = url:match("^[^?#]+") or url
  local name = path:match("([^/]+)$")
  if not name or name == "" then
    return "download"
  end
  return name
end

-- resolves a block's "@download" annotation (true, a filename, or nil) to
-- a full destination path under ".httpfly/downloads/" next to the .http
-- file, creating that directory if needed -- httpfly's own "-download"
-- does not create its target's parent directory for you (same as
-- "curl -o"), it just fails if it's missing.
local function resolve_download_path(block, cwd)
  if not block.download then
    return nil
  end
  local filename = block.download == true and guess_filename(block.url) or block.download
  local dir = cwd .. "/.httpfly/downloads"
  vim.fn.mkdir(dir, "p")
  return dir .. "/" .. filename
end

function M.send_current()
  if not ensure_binary() then
    return
  end
  local file = require_file()
  if not file then
    return
  end
  local line = vim.api.nvim_win_get_cursor(0)[1]
  local block = find_enclosing_block(0, line)
  if not block or not block.name then
    vim.notify("httpfly: no request (@name) found under cursor", vim.log.levels.WARN)
    return
  end
  local cwd = env.resolve_cwd(0)
  local download_path = resolve_download_path(block, cwd)
  run(build_cmd(file, block.name, download_path), cwd)
end

function M.send_all()
  if not ensure_binary() then
    return
  end
  local file = require_file()
  if not file then
    return
  end
  local cwd = env.resolve_cwd(0)
  local blocks = parse_blocks(0)

  local has_download = false
  for _, b in ipairs(blocks) do
    if b.name and b.download then
      has_download = true
      break
    end
  end

  if not has_download then
    run(build_cmd(file), cwd)
    return
  end

  -- at least one request wants its response saved -- "-download" only
  -- ever applies to a single selected request, so the whole file has to
  -- be sent as one invocation per request instead of httpfly's own
  -- multi-request "run <file>" (see run_many()'s comment)
  local cmds = {}
  for _, b in ipairs(blocks) do
    if b.name then
      table.insert(cmds, build_cmd(file, b.name, resolve_download_path(b, cwd)))
    end
  end
  run_many(cmds, cwd)
end

function M.session_clear()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("httpfly: buffer has no file", vim.log.levels.WARN)
    return
  end
  if session.clear(env.resolve_cwd(0)) then
    vim.notify("httpfly: session cleared", vim.log.levels.INFO)
  else
    vim.notify("httpfly: no session file to clear", vim.log.levels.INFO)
  end
end

return M
