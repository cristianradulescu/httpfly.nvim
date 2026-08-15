local config = require("httpfly.config")
local env = require("httpfly.env")
local format = require("httpfly.format")
local preview = require("httpfly.preview")
local history = require("httpfly.history")
local session = require("httpfly.session")

local M = {}

local result_buf_name = "httpfly://result"

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
  local plugin_env = { HTTPYAC_PLUGIN = session.plugin_path() }
  local cmd_str = string.format(
    "cd %s && HTTPYAC_PLUGIN=%s %s",
    shell_quote({ cwd }),
    shell_quote({ plugin_env.HTTPYAC_PLUGIN }),
    shell_quote(cmd)
  )

  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "Running: " .. cmd_str, "" })
  vim.bo[buf].modifiable = false

  vim.system(cmd, { cwd = cwd, env = plugin_env, text = true }, function(res)
    vim.schedule(function()
      if not vim.api.nvim_buf_is_valid(buf) then
        return
      end

      local lines, truncations
      if res.stdout then
        lines, truncations = format.render(res.stdout, cmd_str)
      end
      local filetype = "markdown"

      if lines then
        history.save(lines)
      end

      if not lines then
        -- fall back to raw output (e.g. httpyac crashed before emitting JSON)
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
    end)
  end)
end

local function build_cmd(extra)
  local cmd = { config.options.cmd, "send", "--json", "--no-color" }
  vim.list_extend(cmd, extra)

  local e = env.get(0)
  if e then
    vim.list_extend(cmd, { "--env", e })
  end

  return cmd
end

local function require_file()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("httpfly: buffer has no file", vim.log.levels.WARN)
    return nil
  end
  return file
end

-- httpyac resolves http-client.env.json (and its .private. counterpart)
-- relative to its own process cwd, not relative to the .http file being
-- sent. So when the request lives in a subdirectory below the env file
-- (e.g. v2/request.http with http-client.env.json at the project root),
-- running httpyac with cwd = the request's own directory makes it unable
-- to find the env file at all. Use the env file's directory instead, when
-- one was found for this buffer; otherwise fall back to the file's own
-- directory since there's nothing else to prefer.
local function resolve_cwd(file)
  local env_file = env.env_file_for_buf(0)
  if env_file then
    return vim.fn.fnamemodify(env_file, ":h")
  end
  return vim.fn.fnamemodify(file, ":h")
end

function M.send_current()
  local file = require_file()
  if not file then
    return
  end
  local line = vim.api.nvim_win_get_cursor(0)[1]
  local cwd = resolve_cwd(file)
  run(build_cmd({ file, "--line", tostring(line) }), cwd)
end

function M.send_all()
  local file = require_file()
  if not file then
    return
  end
  local cwd = resolve_cwd(file)
  run(build_cmd({ file, "--all" }), cwd)
end

function M.session_clear()
  local file = require_file()
  if not file then
    return
  end
  if session.clear(resolve_cwd(file)) then
    vim.notify("httpfly: session cleared", vim.log.levels.INFO)
  else
    vim.notify("httpfly: no session file to clear", vim.log.levels.INFO)
  end
end

return M
