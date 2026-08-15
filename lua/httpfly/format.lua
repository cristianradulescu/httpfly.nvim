local config = require("httpfly.config")
local shared = require("httpfly.format.shared")

local M = {}

local renderers = {
  markdown = "httpfly.format.markdown",
  unicode = "httpfly.format.unicode",
}

function M.render(raw_stdout, cmd_str)
  local ok, decoded = pcall(vim.json.decode, shared.extract_json(raw_stdout))
  if not ok or type(decoded) ~= "table" or not decoded.requests then
    return nil
  end

  local module_path = renderers[config.options.output_style] or renderers.markdown
  local renderer = require(module_path)
  return renderer.render(decoded, cmd_str)
end

return M
