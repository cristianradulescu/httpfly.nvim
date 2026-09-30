local backend = require("httpfly.backend")

local M = {}

local health = vim.health

local function check_backend()
  local path, err = backend.resolve()
  if not path then
    health.error(err, {
      "Run `make build` in the httpfly.nvim plugin directory to compile it (requires Go).",
      'lazy.nvim users: add build = "make build" to the plugin spec to build it automatically.',
    })
    return
  end

  local res = vim.system({ path, "version" }, { text = true }):wait()
  if res.code ~= 0 then
    health.error("httpfly at " .. path .. " did not run successfully", { vim.trim(res.stderr or "") })
    return
  end
  health.ok(vim.trim(res.stdout or "") .. " (" .. path .. ")")
end

local function check_go()
  if vim.fn.executable("go") == 1 then
    local res = vim.system({ "go", "version" }, { text = true }):wait()
    health.ok(vim.trim(res.stdout or "go"))
  else
    health.warn(
      "`go` not found on $PATH",
      { "Go is needed to (re)build the bundled httpfly binary with `make build`." }
    )
  end
end

function M.check()
  health.start("httpfly backend")
  check_backend()
  check_go()
end

return M
