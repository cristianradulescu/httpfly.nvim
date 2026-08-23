local M = {}

M.state_filename = ".httpfly/state.json"

function M.file_for_dir(dir)
  return dir .. "/" .. M.state_filename
end

-- flat key -> value map of variables currently persisted for env_name
-- (pass "" for no -env) in the given directory's .httpfly/state.json --
-- httpfly itself writes this file (client.global:set in a script writes
-- through immediately), keyed by environment name so "-env dev"/"-env
-- prod" never share values; used by env.lua to show persisted overrides
-- alongside env-file variables
function M.load(dir, env_name)
  local path = M.file_for_dir(dir)
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

  local bucket = decoded[env_name or ""]
  if type(bucket) ~= "table" then
    return {}
  end
  return bucket
end

function M.clear(dir)
  local path = M.file_for_dir(dir)
  if vim.fn.filereadable(path) == 1 then
    vim.fn.delete(path)
    return true
  end
  return false
end

return M
