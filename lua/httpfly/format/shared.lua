local config = require("httpfly.config")

local M = {}

-- httpfly's "-json" output is a top-level array, so the payload starts at
-- "[", not "{" -- defensive against any stray non-JSON text before it
-- (e.g. if `cmd` is ever invoked through something like npx that prints
-- notices to stdout). httpfly itself keeps its stdout clean in -json
-- mode: since its post-v0.3.1 change a script's print(...) and flag-parsing errors both
-- go to stderr, which runner.lua shows separately.
function M.extract_json(text)
  local start = text:find("[", 1, true)
  if not start then
    return text
  end
  return text:sub(start)
end

-- truncates long single-line values (a JWT in an Authorization header
-- would otherwise stretch the headers table far past the window width);
-- returns display text plus the full original text if truncation
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

-- httpfly's headers are always "name -> array of values" (a header can
-- legitimately repeat, e.g. Set-Cookie), never a bare string -- joins them
-- for display/matching purposes
function M.header_value(v)
  if type(v) == "table" then
    return table.concat(v, ", ")
  end
  return tostring(v)
end

function M.body_lang(headers)
  local ct = headers and (headers["content-type"] or headers["Content-Type"])
  if not ct then
    return "text"
  end
  ct = M.header_value(ct)
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
