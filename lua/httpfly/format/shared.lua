local config = require("httpfly.config")

local M = {}

function M.extract_json(text)
  local start = text:find("{")
  if not start then
    return text
  end
  return text:sub(start)
end

-- truncates long single-line values, which otherwise break markdown table
-- rendering (e.g. in render-markdown.nvim) or just make plain output hard
-- to scan; returns display text plus the full original text if truncation
-- happened, nil otherwise
function M.truncate(value)
  value = tostring(value)
  local max_len = config.options.max_header_value_len
  if #value <= max_len then
    return value, nil
  end
  return value:sub(1, max_len) .. "…", value
end

-- classifies an HTTP status code the same way across renderers; each
-- renderer maps the category to its own symbol/highlight group
function M.status_category(code)
  if not code then
    return "warn"
  elseif code >= 200 and code < 300 then
    return "ok"
  elseif code >= 400 then
    return "error"
  else
    return "warn"
  end
end

-- pass/fail marker for a testResults entry's status, identical across
-- renderers
function M.test_mark(status)
  return status == "SUCCESS" and "✅" or "❌"
end

local STATUS_BADGE = { ok = "✅", warn = "⚠️", error = "❌" }

-- symbol for a status code's outcome, identical across renderers
function M.status_badge(code)
  return STATUS_BADGE[M.status_category(code)]
end

function M.body_lang(headers)
  local ct = headers and (headers["content-type"] or headers["Content-Type"])
  if not ct then
    return "text"
  end
  if ct:find("json") then
    return "json"
  elseif ct:find("xml") then
    return "xml"
  elseif ct:find("html") then
    return "html"
  end
  return "text"
end

return M
