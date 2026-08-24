local json = require("httpfly.json")
local shared = require("httpfly.format.shared")

local M = {}

-- appends a markdown table of headers to `out`, recording any truncated
-- cell's full value into `truncations` keyed by the resulting line number
local function append_headers(out, truncations, headers)
  if not headers or next(headers) == nil then
    table.insert(out, "_none_")
    return
  end

  local keys = {}
  for k in pairs(headers) do
    table.insert(keys, k)
  end
  table.sort(keys)

  table.insert(out, "| Header | Value |")
  table.insert(out, "|---|---|")
  for _, k in ipairs(keys) do
    local display, full = shared.truncate(shared.header_value(headers[k]))
    display = display:gsub("|", "\\|")
    table.insert(out, string.format("| `%s` | %s |", k, display))
    if full then
      truncations[#out] = full
    end
  end
end

local function body_block(body, headers)
  if not body or body == "" then
    return { "_(empty body)_" }
  end
  if shared.is_binary(body) then
    return { string.format("_(binary content, %d bytes — not shown)_", #body) }
  end

  local lang = shared.body_lang(headers)
  local text = body
  if lang == "json" then
    local ok, pretty = pcall(json.pretty, body)
    if ok and pretty and pretty ~= "" then
      text = pretty
    end
  end

  local lines = { "```" .. lang }
  vim.list_extend(lines, vim.split(text, "\n"))
  table.insert(lines, "```")
  return lines
end

function M.render(decoded, cmd_str)
  local out = {}
  local truncations = {}

  if cmd_str then
    table.insert(out, "**Command**")
    table.insert(out, "")
    table.insert(out, "```sh")
    -- cmd_str can be several newline-joined commands (run_many() stitching
    -- several "httpfly run -name X" invocations into one view, for
    -- "@download") -- vim.split is a no-op for an already single-line
    -- cmd_str, so this is safe either way
    vim.list_extend(out, vim.split(cmd_str, "\n"))
    table.insert(out, "```")
    table.insert(out, "")
  end

  local total, failed, script_errors = #decoded, 0, 0
  for _, req in ipairs(decoded) do
    if req.error then
      failed = failed + 1
    end
    if req.script_error then
      script_errors = script_errors + 1
    end
  end
  table.insert(
    out,
    string.format(
      "# httpfly results — %d/%d sent, %d failed to send, %d script error(s)",
      total - failed,
      total,
      failed,
      script_errors
    )
  )
  table.insert(out, "")

  for _, req in ipairs(decoded) do
    local rreq = req.request or {}
    local resp = req.response
    table.insert(out, "## " .. req.name)
    table.insert(out, "")

    if req.error then
      table.insert(out, "### Error")
      table.insert(out, "")
      table.insert(out, "`" .. tostring(req.error) .. "`")
      table.insert(out, "")
    end

    if resp then
      table.insert(
        out,
        string.format(
          "%s **%s** `%s` → **%s** (%dms)",
          shared.status_badge(resp.status_code),
          rreq.method or "?",
          rreq.url or "?",
          tostring(resp.status_code or "?"),
          req.duration_ms or 0
        )
      )
      table.insert(out, "")

      table.insert(out, "### Request")
      table.insert(out, "")
      table.insert(out, "**Headers**")
      table.insert(out, "")
      append_headers(out, truncations, rreq.headers)
      table.insert(out, "")
      if rreq.body and rreq.body ~= "" then
        table.insert(out, "**Body**")
        table.insert(out, "")
        vim.list_extend(out, body_block(rreq.body, rreq.headers))
        table.insert(out, "")
      end

      table.insert(out, "### Response")
      table.insert(out, "")
      table.insert(out, "**Headers**")
      table.insert(out, "")
      append_headers(out, truncations, resp.headers)
      table.insert(out, "")
      if resp.download_path then
        table.insert(out, "**Downloaded to**")
        table.insert(out, "")
        table.insert(out, "`" .. resp.download_path .. "`")
        table.insert(out, "")
      else
        table.insert(out, "**Body**")
        table.insert(out, "")
        vim.list_extend(out, body_block(resp.body, resp.headers))
        table.insert(out, "")
      end

      if req.script_error then
        table.insert(out, string.format("⚠️ **post-request script error:** %s", tostring(req.script_error)))
        table.insert(out, "")
      end
    end

    table.insert(out, "---")
    table.insert(out, "")
  end

  return out, truncations
end

return M
