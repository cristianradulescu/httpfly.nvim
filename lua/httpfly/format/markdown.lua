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
    local display, full = shared.truncate(headers[k])
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
    table.insert(out, cmd_str)
    table.insert(out, "```")
    table.insert(out, "")
  end

  local s = decoded.summary or {}
  table.insert(
    out,
    string.format(
      "# httpyac results — %d/%d succeeded, %d failed, %d errored",
      s.successRequests or 0,
      s.totalRequests or 0,
      s.failedRequests or 0,
      s.erroredRequests or 0
    )
  )
  table.insert(out, "")

  for _, req in ipairs(decoded.requests) do
    local resp = req.response
    local download_path, test_results = shared.extract_download(req.testResults)
    local title = (req.name and req.name ~= "") and req.name or req.fileName
    table.insert(out, "## " .. title)
    table.insert(out, "")

    if resp then
      local rreq = resp.request or {}
      table.insert(
        out,
        string.format(
          "%s **%s** `%s` → **%s %s** (%dms)",
          shared.status_badge(resp.statusCode),
          rreq.method or "?",
          rreq.url or "?",
          tostring(resp.statusCode or "?"),
          resp.statusMessage or "",
          math.floor((req.duration or 0) + 0.5)
        )
      )
      table.insert(out, "")

      table.insert(out, "### Request")
      table.insert(out, "")
      table.insert(out, "**Headers**")
      table.insert(out, "")
      append_headers(out, truncations, rreq.headers)
      table.insert(out, "")
      if rreq.body then
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
      table.insert(out, "**Body**")
      table.insert(out, "")
      vim.list_extend(out, body_block(resp.body, resp.headers))
      table.insert(out, "")

      if download_path then
        table.insert(out, "### Download")
        table.insert(out, "")
        table.insert(out, "`" .. download_path .. "`")
        table.insert(out, "")
      end
    end

    if test_results and #test_results > 0 then
      table.insert(out, "### Test Results")
      table.insert(out, "")
      for _, t in ipairs(test_results) do
        table.insert(out, string.format("- %s %s", shared.test_mark(t.status), t.message or t.status or ""))
      end
      table.insert(out, "")
    end

    table.insert(out, "---")
    table.insert(out, "")
  end

  return out, truncations
end

return M
