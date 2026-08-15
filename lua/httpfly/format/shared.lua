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

-- a NUL byte is a rock-solid signal that content isn't real text (valid
-- JSON/XML/HTML/plain text never contains one) -- used to avoid dumping
-- genuinely binary response bodies into the result buffer. Needed because
-- a Lua string containing a NUL byte, once it reaches something like
-- vim.fn.writefile()'s line-list argument, gets silently promoted to a
-- Blob by Neovim's own Lua<->VimL bridge (VimL strings are NUL-terminated,
-- unlike Lua's byte-counted strings) -- which that function then rejects
-- outright ("Expected a Number or a String, Blob found"), crashing
-- history.save() for any request whose body happens to contain one.
function M.is_binary(text)
  return text:find("\0", 1, true) ~= nil
end

-- kept in sync with httpyac-plugin/httpfly.js's DOWNLOAD_MARKER -- see
-- that file for why a testResult message prefix is the channel used
M.DOWNLOAD_MARKER = "httpfly:download:"

-- pulls the saved-file path (if any) for an "@download" request out of
-- its testResults, returning the path plus a copy of testResults with
-- that synthetic entry removed (so it doesn't also render as a fake
-- test). Safe to call even when test_results is nil/empty.
function M.extract_download(test_results)
  if not test_results then
    return nil, test_results
  end

  local download_path
  local filtered = {}
  for _, entry in ipairs(test_results) do
    local message = type(entry) == "table" and entry.message
    if not download_path and type(message) == "string" and message:sub(1, #M.DOWNLOAD_MARKER) == M.DOWNLOAD_MARKER then
      download_path = message:sub(#M.DOWNLOAD_MARKER + 1)
    else
      table.insert(filtered, entry)
    end
  end
  return download_path, filtered
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
