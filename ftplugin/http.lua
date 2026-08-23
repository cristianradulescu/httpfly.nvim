local config = require("httpfly.config")

vim.wo.winbar = "%{%v:lua.require('httpfly.env').status()%}"

if config.options.keymaps then
  vim.keymap.set("n", "<leader>hs", "<cmd>HttpSend<cr>", { buffer = true, desc = "Send request under cursor" })
  vim.keymap.set("n", "<leader>ha", "<cmd>HttpSendAll<cr>", { buffer = true, desc = "Send all requests in file" })
  vim.keymap.set("n", "<leader>he", "<cmd>HttpEnv<cr>", { buffer = true, desc = "Pick httpfly environment" })
  vim.keymap.set(
    "n",
    "<leader>hv",
    "<cmd>HttpEnvVars<cr>",
    { buffer = true, desc = "Show selected environment's variables" }
  )
  vim.keymap.set(
    "n",
    "<leader>hc",
    "<cmd>HttpSessionClear<cr>",
    { buffer = true, desc = "Clear httpfly session variables" }
  )
end
