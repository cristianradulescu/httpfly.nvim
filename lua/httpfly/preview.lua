local M = {}

-- result buffer id -> { [line] = full untruncated value }
local truncations = {}

function M.set(buf, map)
  truncations[buf] = map
end

function M.show(buf)
  local map = truncations[buf]
  if not map then
    return
  end

  local line = vim.api.nvim_win_get_cursor(0)[1]
  local text = map[line]
  if not text then
    vim.notify("httpfly: nothing truncated on this line", vim.log.levels.INFO)
    return
  end

  local lines = vim.split(text, "\n")
  local width = 0
  for _, l in ipairs(lines) do
    width = math.max(width, vim.fn.strdisplaywidth(l))
  end
  width = math.min(width + 2, math.floor(vim.o.columns * 0.8))
  width = math.max(width, 20)
  local height = math.min(#lines, math.floor(vim.o.lines * 0.6))
  height = math.max(height, 1)

  local preview_buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(preview_buf, 0, -1, false, lines)
  vim.bo[preview_buf].modifiable = false
  vim.bo[preview_buf].bufhidden = "wipe"

  local win = vim.api.nvim_open_win(preview_buf, false, {
    relative = "cursor",
    row = 1,
    col = 0,
    width = width,
    height = height,
    style = "minimal",
    border = "rounded",
  })
  vim.wo[win].wrap = true

  vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI", "BufLeave", "InsertEnter" }, {
    buffer = buf,
    once = true,
    callback = function()
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_win_close(win, true)
      end
    end,
  })
end

return M
