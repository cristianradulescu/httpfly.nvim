local M = {}

M.filename = ".httpfly/session.json"

-- absolute path to the bundled httpyac plugin (httpyac-plugin/session-persist.js)
-- that mirrors httpyac's own global-variable sessionStore to disk; computed
-- from this file's own location so it works regardless of where the user's
-- plugin manager installed httpfly.nvim
function M.plugin_path()
  local source = debug.getinfo(1, "S").source:sub(2)
  local plugin_root = vim.fn.fnamemodify(source, ":h:h:h")
  return plugin_root .. "/httpyac-plugin/session-persist.js"
end

function M.file_for_cwd(cwd)
  return cwd .. "/" .. M.filename
end

-- flat key -> value map of variables currently persisted in the session
-- file (mirrors what httpyac-plugin/session-persist.js writes out); used
-- by env.lua to show session overrides alongside env-file variables
function M.load(cwd)
  local path = M.file_for_cwd(cwd)
  if vim.fn.filereadable(path) == 0 then
    return {}
  end

  local ok_read, content = pcall(vim.fn.readfile, path)
  if not ok_read then
    return {}
  end
  local ok_decode, decoded = pcall(vim.json.decode, table.concat(content, "\n"))
  if not ok_decode or type(decoded) ~= "table" then
    return {}
  end

  local vars = {}
  for _, entry in ipairs(decoded) do
    if type(entry) == "table" and type(entry.type) == "string" and entry.type:match("global_cache$") then
      local details = entry.details
      if type(details) == "table" then
        -- some httpyac global-cache shapes nest under "$global", others are flat
        local flat = type(details["$global"]) == "table" and details["$global"] or details
        for k, v in pairs(flat) do
          vars[k] = v
        end
      end
    end
  end
  return vars
end

function M.clear(cwd)
  local path = M.file_for_cwd(cwd)
  if vim.fn.filereadable(path) == 1 then
    vim.fn.delete(path)
    return true
  end
  return false
end

return M
