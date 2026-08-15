if vim.g.loaded_httpfly then
  return
end
vim.g.loaded_httpfly = true

local env = require("httpfly.env")
local runner = require("httpfly.runner")

vim.api.nvim_create_user_command("HttpEnv", function(opts)
  if opts.args ~= "" then
    env.set(opts.args, 0)
  else
    env.pick(0)
  end
end, {
  nargs = "?",
  desc = "Select or set the httpyac environment for the current .http file",
})

vim.api.nvim_create_user_command("HttpSend", function()
  runner.send_current()
end, { desc = "Send the http request under the cursor via httpyac" })

vim.api.nvim_create_user_command("HttpSendAll", function()
  runner.send_all()
end, { desc = "Send all http requests in the current file via httpyac" })

vim.api.nvim_create_user_command("HttpEnvVars", function()
  env.show_vars(0)
end, { desc = "Show variables of the currently selected httpyac environment" })

vim.api.nvim_create_user_command("HttpSessionClear", function()
  runner.session_clear()
end, { desc = "Clear the persisted client.global variables shared between separate sends" })
