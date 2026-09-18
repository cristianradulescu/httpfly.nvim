local unicode = require("httpfly.format.unicode")
local shared = require("httpfly.format.shared")

local M = {}

-- renders an already-decoded array of jsonResult objects -- used by
-- runner.lua when it has to stitch together several separate `httpfly run`
-- invocations (one per request, for "@download") into one combined view
function M.render_decoded(decoded, cmd_str)
  if type(decoded) ~= "table" then
    return nil
  end
  return unicode.render(decoded, cmd_str)
end

function M.render(raw_stdout, cmd_str)
  local ok, decoded = pcall(vim.json.decode, shared.extract_json(raw_stdout))
  if not ok then
    return nil
  end
  return M.render_decoded(decoded, cmd_str)
end

return M
