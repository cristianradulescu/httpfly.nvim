local M = {}

-- the binary `make build` produces, relative to the plugin's own root
M.BIN = "bin/httpfly"

-- resolves the bundled bin/httpfly on the runtimepath (i.e. in this
-- plugin's own directory, wherever the plugin manager installed it).
-- Looked up on every call rather than cached, so a `make build` run
-- mid-session is picked up without restarting.
function M.get_path()
  local found = vim.api.nvim_get_runtime_file(M.BIN, false)[1]
  -- absolute, since httpfly is launched with a different cwd (the env
  -- file's directory) and a relative runtimepath entry would break that
  return found and vim.fn.fnamemodify(found, ":p")
end

function M.missing_message(path)
  local hint = "Run `make build` in the httpfly.nvim plugin directory (requires Go), "
    .. 'or add build = "make build" to your plugin manager spec.'
  if path then
    return "httpfly binary not executable at: " .. path .. ". " .. hint
  end
  return "httpfly binary not found. " .. hint
end

-- returns the path if it's runnable, otherwise nil plus a user-facing error
function M.resolve()
  local path = M.get_path()
  if not path or vim.fn.executable(path) == 0 then
    return nil, M.missing_message(path)
  end
  return path
end

return M
