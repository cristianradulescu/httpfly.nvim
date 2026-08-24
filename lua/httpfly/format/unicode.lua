local json = require("httpfly.json")
local shared = require("httpfly.format.shared")

local M = {}

local RULE_HEAVY = "━"
local RULE_LIGHT = "─"

local JSON_TOKEN_GROUP = {
  key = "@lsp.type.property",
  string = "String",
  number = "Number",
  boolean = "Boolean",
  ["null"] = "Constant",
}

-- unicode's own symbols, deliberately not shared.status_badge's full-color
-- emoji: "✅"/"❌" are emoji glyphs whose color usually comes from the
-- terminal's emoji font, not from our DiagnosticOk/Error highlight, so
-- they can end up low-contrast or barely visible depending on
-- terminal/theme. These are plain Unicode dingbats instead, which render
-- as regular glyphs tinted by our own highlight.
local STATUS_BADGE = { ok = "✔", warn = "⚠", error = "✘" }
local OUTCOME_GROUP = { ok = "DiagnosticOk", warn = "DiagnosticWarn", error = "DiagnosticError" }

local function status_badge(code)
  return STATUS_BADGE[shared.status_category(code)]
end

-- highlight group for a status code's outcome; used for both the badge
-- and the status text so they read as one colored unit
local function outcome_group(code)
  return OUTCOME_GROUP[shared.status_category(code)]
end

-- reformats a single-line "a && b && c" shell command into a readable
-- multi-line script using "&&" line continuations, the way it'd typically
-- be hand-written -- the generated command line is otherwise long enough
-- to force horizontal scrolling in the result split
local function format_command(cmd)
  local parts = vim.split(cmd, " && ", { plain = true })
  if #parts <= 1 then
    return cmd
  end
  return table.concat(parts, " && \\\n")
end

local function pad(text, width)
  return text .. string.rep(" ", width - vim.fn.strdisplaywidth(text))
end

-- records a highlight span for the line just appended to `out`; col
-- offsets are byte offsets (Lua's # on a string is a byte count, which is
-- exactly what nvim_buf_add_highlight expects, so this works correctly
-- even with the multi-byte UTF-8 box-drawing/emoji characters used here)
local function hl(highlights, out, col_start, col_end, group)
  highlights[#highlights + 1] = { #out, col_start, col_end, group }
end

local function hl_line(highlights, out, group)
  hl(highlights, out, 0, -1, group)
end

-- appends a box-drawing table of headers to `out`, recording any truncated
-- cell's full value into `truncations` keyed by the resulting line number
local function append_headers_table(out, truncations, highlights, headers)
  if not headers or next(headers) == nil then
    table.insert(out, "(none)")
    return
  end

  local keys = {}
  for k in pairs(headers) do
    table.insert(keys, k)
  end
  table.sort(keys)

  local col1_header, col2_header = "Header", "Value"
  local rows = {} -- { key, display, full_or_nil }
  local col1_w = vim.fn.strdisplaywidth(col1_header)
  local col2_w = vim.fn.strdisplaywidth(col2_header)
  for _, k in ipairs(keys) do
    local display, full = shared.truncate(shared.header_value(headers[k]))
    rows[#rows + 1] = { k, display, full }
    col1_w = math.max(col1_w, vim.fn.strdisplaywidth(k))
    col2_w = math.max(col2_w, vim.fn.strdisplaywidth(display))
  end

  local top = "┌" .. string.rep("─", col1_w + 2) .. "┬" .. string.rep("─", col2_w + 2) .. "┐"
  local mid = "├" .. string.rep("─", col1_w + 2) .. "┼" .. string.rep("─", col2_w + 2) .. "┤"
  local bot = "└" .. string.rep("─", col1_w + 2) .. "┴" .. string.rep("─", col2_w + 2) .. "┘"

  table.insert(out, top)
  hl_line(highlights, out, "Comment")
  table.insert(out, "│ " .. pad(col1_header, col1_w) .. " │ " .. pad(col2_header, col2_w) .. " │")
  hl_line(highlights, out, "Title")
  table.insert(out, mid)
  hl_line(highlights, out, "Comment")
  for _, row in ipairs(rows) do
    local key_prefix = "│ "
    local key_str = pad(row[1], col1_w)
    table.insert(out, key_prefix .. key_str .. " │ " .. pad(row[2], col2_w) .. " │")
    hl(highlights, out, #key_prefix, #key_prefix + #key_str, "Identifier")
    if row[3] then
      truncations[#out] = row[3]
    end
  end
  table.insert(out, bot)
  hl_line(highlights, out, "Comment")
end

-- a body "block" delimited by light horizontal rules, the unicode
-- equivalent of a markdown fenced code block
local function body_block(out, highlights, body, headers)
  if not body or body == "" then
    table.insert(out, "(empty body)")
    return
  end
  if shared.is_binary(body) then
    table.insert(out, string.format("(binary content, %d bytes -- not shown)", #body))
    return
  end

  local lang = shared.body_lang(headers)
  local text = body
  local json_tokens
  if lang == "json" then
    json_tokens = {}
    local ok, pretty = pcall(json.pretty, body, json_tokens)
    if ok and pretty and pretty ~= "" then
      text = pretty
    else
      json_tokens = nil
    end
  end

  local lines = vim.split(text, "\n")
  local width = 0
  for _, l in ipairs(lines) do
    width = math.max(width, vim.fn.strdisplaywidth(l))
  end
  local rule = string.rep(RULE_LIGHT, math.max(width, 1))

  table.insert(out, rule)
  hl_line(highlights, out, "Comment")
  local body_line_offset = #out
  vim.list_extend(out, lines)
  for _, t in ipairs(json_tokens or {}) do
    local group = JSON_TOKEN_GROUP[t.type]
    if group then
      highlights[#highlights + 1] = { body_line_offset + t.line + 1, t.col_start, t.col_end, group }
    end
  end
  table.insert(out, rule)
  hl_line(highlights, out, "Comment")
end

function M.render(decoded, cmd_str)
  local out = {}
  local truncations = {}
  local highlights = {}

  if cmd_str then
    table.insert(out, "Command")
    hl_line(highlights, out, "Statement")
    body_block(out, highlights, format_command(cmd_str), nil)
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
      "httpfly results — %d/%d sent, %d failed to send, %d script error(s)",
      total - failed,
      total,
      failed,
      script_errors
    )
  )
  local summary_group = "DiagnosticOk"
  if failed > 0 or script_errors > 0 then
    summary_group = failed > 0 and "DiagnosticError" or "DiagnosticWarn"
  end
  hl_line(highlights, out, summary_group)
  table.insert(out, "")

  for _, req in ipairs(decoded) do
    local rreq = req.request or {}
    local resp = req.response
    local title = req.name
    local heavy_rule = string.rep(RULE_HEAVY, math.max(vim.fn.strdisplaywidth(title) + 2, 20))

    table.insert(out, heavy_rule)
    hl_line(highlights, out, "Comment")
    table.insert(out, " " .. title)
    hl_line(highlights, out, "Title")
    table.insert(out, heavy_rule)
    hl_line(highlights, out, "Comment")
    table.insert(out, "")

    if req.error then
      table.insert(out, "▸ Error")
      hl_line(highlights, out, "Title")
      table.insert(out, "")
      table.insert(out, "  " .. tostring(req.error))
      hl_line(highlights, out, "DiagnosticError")
      table.insert(out, "")
    end

    if resp then
      local og = outcome_group(resp.status_code)
      local badge = status_badge(resp.status_code)
      local method = rreq.method or "?"
      local url = rreq.url or "?"
      local status_text = tostring(resp.status_code or "?")
      local prefix = string.format("%s %s %s → ", badge, method, url)
      table.insert(out, string.format("%s%s (%dms)", prefix, status_text, req.duration_ms or 0))
      hl(highlights, out, 0, #badge, og)
      hl(highlights, out, #badge + 1, #badge + 1 + #method, "Keyword")
      hl(highlights, out, #prefix, #prefix + #status_text, og)
      table.insert(out, "")

      table.insert(out, "▸ Request")
      hl_line(highlights, out, "Title")
      table.insert(out, "")
      table.insert(out, "Headers")
      hl_line(highlights, out, "Statement")
      append_headers_table(out, truncations, highlights, rreq.headers)
      table.insert(out, "")
      if rreq.body and rreq.body ~= "" then
        table.insert(out, "Body")
        hl_line(highlights, out, "Statement")
        body_block(out, highlights, rreq.body, rreq.headers)
        table.insert(out, "")
      end

      table.insert(out, "▸ Response")
      hl_line(highlights, out, "Title")
      table.insert(out, "")
      table.insert(out, "Headers")
      hl_line(highlights, out, "Statement")
      append_headers_table(out, truncations, highlights, resp.headers)
      table.insert(out, "")
      if resp.download_path then
        table.insert(out, "Downloaded to")
        hl_line(highlights, out, "Statement")
        table.insert(out, "  " .. resp.download_path)
        hl_line(highlights, out, "DiagnosticOk")
        table.insert(out, "")
      else
        table.insert(out, "Body")
        hl_line(highlights, out, "Statement")
        body_block(out, highlights, resp.body, resp.headers)
        table.insert(out, "")
      end

      if req.script_error then
        table.insert(out, "  ⚠ post-request script error: " .. tostring(req.script_error))
        hl_line(highlights, out, "DiagnosticWarn")
        table.insert(out, "")
      end
    end
  end

  return out, truncations, highlights
end

return M
