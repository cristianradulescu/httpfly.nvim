local M = {}

M.defaults = {
  cmd = "httpfly",
  env_file = "http-client.env.json",
  private_env_file = "http-client.private.env.json",
  keymaps = true,
  max_header_value_len = 100,
  preview_keymap = "K",
  timeout = nil, -- per-request timeout passed as httpfly's "-timeout", e.g. "2m"; nil = httpfly's default (30s)
}

M.options = vim.deepcopy(M.defaults)

function M.setup(opts)
  M.options = vim.tbl_deep_extend("force", M.defaults, opts or {})
end

return M
