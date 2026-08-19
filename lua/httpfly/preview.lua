local M = {}

-- result buffer id -> { [line] = full untruncated value }
local truncations = {}

-- result buffer id -> { win, preview_buf, line, close_autocmd_id }
-- tracks the currently open preview popup (if any) for that result buffer,
-- so a second `K` on the same line can focus into it instead of reopening it
local active = {}

function M.set(buf, map)
  truncations[buf] = map
end

local function close_active(buf)
  local st = active[buf]
  if not st then
    return
  end
  active[buf] = nil
  if st.close_autocmd_id then
    pcall(vim.api.nvim_del_autocmd, st.close_autocmd_id)
  end
  if vim.api.nvim_win_is_valid(st.win) then
    vim.api.nvim_win_close(st.win, true)
  end
end

function M.show(buf)
  local map = truncations[buf]
  if not map then
    return
  end

  local line = vim.api.nvim_win_get_cursor(0)[1]

  local st = active[buf]
  if st and st.line == line and vim.api.nvim_win_is_valid(st.win) then
    if vim.api.nvim_get_current_win() == st.win then
      -- already focused in the popup: press K again to close it
      close_active(buf)
      return
    end

    -- second K on the same line: jump focus into the already-open popup
    if st.close_autocmd_id then
      pcall(vim.api.nvim_del_autocmd, st.close_autocmd_id)
      st.close_autocmd_id = nil
    end
    vim.api.nvim_set_current_win(st.win)

    local close_on_leave = vim.api.nvim_create_autocmd({ "BufLeave", "WinLeave" }, {
      buffer = st.preview_buf,
      once = true,
      callback = function()
        close_active(buf)
      end,
    })
    st.close_autocmd_id = close_on_leave
    return
  end

  local text = map[line]
  if not text then
    vim.notify("httpfly: nothing truncated on this line", vim.log.levels.INFO)
    return
  end

  close_active(buf)

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

  vim.keymap.set("n", "q", function()
    close_active(buf)
  end, { buffer = preview_buf, nowait = true, silent = true })

  local close_on_move = vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI", "BufLeave", "InsertEnter" }, {
    buffer = buf,
    once = true,
    callback = function()
      close_active(buf)
    end,
  })

  active[buf] = {
    win = win,
    preview_buf = preview_buf,
    line = line,
    close_autocmd_id = close_on_move,
  }
end

return M
