local config = require("httpfly.config")
local env = require("httpfly.env")
local format = require("httpfly.format")
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

local function run(cmd, cwd)
  local buf = open_result_buf()
  local cmd_str = shell_quote(cmd)

  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "Running: " .. cmd_str, "" })
  vim.bo[buf].modifiable = false

  vim.system(cmd, { cwd = cwd, text = true }, function(res)
    vim.schedule(function()
      if not vim.api.nvim_buf_is_valid(buf) then
        return
      end

      local lines, truncations, highlights
      if res.stdout then
        lines, truncations, highlights = format.render(res.stdout, cmd_str)
      end
      local filetype = config.options.output_style == "unicode" and "text" or "markdown"
      local history_ext = config.options.output_style == "unicode" and "txt" or "md"

      if lines then
        history.save(lines, history_ext, cwd)
      end

      if not lines then
        -- fall back to raw output (e.g. httpfly crashed before emitting JSON)
        filetype = "httpresult"
        lines = { "Command: " .. cmd_str, "" }
        if res.stdout and #res.stdout > 0 then
          vim.list_extend(lines, vim.split(res.stdout, "\n"))
        end
        if res.code ~= 0 then
          table.insert(lines, "")
          table.insert(lines, "--- exit code: " .. res.code .. " ---")
          if res.stderr and #res.stderr > 0 then
            vim.list_extend(lines, vim.split(res.stderr, "\n"))
          end
        end
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
    end)
  end)
end

local function build_cmd(file, name_filter)
  local cmd = { config.options.cmd, "run", "-json" }

  local e = env.get(0)
  if e then
    vim.list_extend(cmd, { "-env", e })
  end
  if name_filter then
    vim.list_extend(cmd, { "-name", name_filter })
  end
  table.insert(cmd, file)

  return cmd
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

-- httpfly resolves httpfly.env.json and .httpfly/state.json relative to
-- its own process cwd, with no upward search -- so the cwd passed to
-- vim.system has to be exactly the directory a real `httpfly run` from
-- there would use. The .http file's own directory is the only sensible
-- choice, since that's also where env.lua checks for httpfly.env.json.
local function resolve_cwd(file)
  return vim.fn.fnamemodify(file, ":h")
end

-- httpfly's "-name" flag replaces httpyac's line-based "--line N": find
-- the @name of the request block enclosing the cursor by mirroring
-- httpfly's own parser, which splits the file on "###" lines (the segment
-- before the first "###" is the prelude and never carries a @name).
local function find_enclosing_name(bufnr, cursor_line)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, false)

  local boundaries = { 0 }
  for i, line in ipairs(lines) do
    if line:match("^%s*###") then
      table.insert(boundaries, i)
    end
  end
  table.insert(boundaries, #lines + 1)

  local block_start, block_end
  for idx = 1, #boundaries - 1 do
    local start_line = boundaries[idx] + 1
    local end_line = boundaries[idx + 1] - 1
    if cursor_line >= start_line and cursor_line <= end_line then
      block_start, block_end = start_line, end_line
      break
    end
  end
  if not block_start then
    return nil
  end

  for i = block_start, block_end do
    local name = lines[i] and lines[i]:match("^%s*#%s*@name%s+(.-)%s*$")
    if name and name ~= "" then
      return name
    end
  end
  return nil
end

function M.send_current()
  local file = require_file()
  if not file then
    return
  end
  local line = vim.api.nvim_win_get_cursor(0)[1]
  local name = find_enclosing_name(0, line)
  if not name then
    vim.notify("httpfly: no request (@name) found under cursor", vim.log.levels.WARN)
    return
  end
  local cwd = resolve_cwd(file)
  run(build_cmd(file, name), cwd)
end

function M.send_all()
  local file = require_file()
  if not file then
    return
  end
  local cwd = resolve_cwd(file)
  run(build_cmd(file), cwd)
end

function M.session_clear()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("httpfly: buffer has no file", vim.log.levels.WARN)
    return
  end
  if session.clear(resolve_cwd(file)) then
    vim.notify("httpfly: session cleared", vim.log.levels.INFO)
  else
    vim.notify("httpfly: no session file to clear", vim.log.levels.INFO)
  end
end

return M
