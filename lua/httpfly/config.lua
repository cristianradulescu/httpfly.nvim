local M = {}

M.defaults = {
  cmd = "httpfly",
  env_file = "httpfly.env.json",
  keymaps = true,
  max_header_value_len = 100,
  preview_keymap = "K",
  output_style = "markdown", -- "markdown" or "unicode"
}

M.options = vim.deepcopy(M.defaults)

function M.setup(opts)
  M.options = vim.tbl_deep_extend("force", M.defaults, opts or {})
end

return M
