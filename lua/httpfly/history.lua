local M = {}

function M.save(lines, ext)
  local dir = vim.fn.getcwd() .. "/.httpfly/history"
  vim.fn.mkdir(dir, "p")

  local sec, usec = vim.uv.gettimeofday()
  local filename = os.date("%Y%m%d-%H%M%S", sec) .. string.format("-%06d", usec) .. "." .. (ext or "md")
  local path = dir .. "/" .. filename

  vim.fn.writefile(lines, path)
  return path
end

return M
