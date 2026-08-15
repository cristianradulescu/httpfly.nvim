local M = {}

-- Re-indents a JSON string with 2-space indentation. Operates as a bracket
-- scanner rather than a full parse+re-encode, so object key order and
-- numeric/string formatting are preserved exactly (vim.json.decode would
-- lose key order since Lua tables are unordered).
--
-- `tokens`, if given, is filled with one entry per meaningful token (object
-- keys, string values, numbers, booleans, null) as it's emitted:
-- { line = <0-based line within the returned text>, col_start = <byte>,
--   col_end = <byte>, type = "key"|"string"|"number"|"boolean"|"null" }.
-- Structural characters (brackets, commas, colons, indentation) never
-- contain a real newline within a single token, so line/col bookkeeping
-- only needs to special-case the indentation chunks this function itself
-- inserts.
function M.pretty(str, tokens)
  local out = {}
  local indent = 0
  local i = 1
  local n = #str
  local line, col = 0, 0

  local function emit(chunk, token_type)
    if token_type and tokens then
      tokens[#tokens + 1] = { line = line, col_start = col, col_end = col + #chunk, type = token_type }
    end
    table.insert(out, chunk)

    local nl_count, last_nl_end = 0, nil
    local pos = 1
    while true do
      local s, e = chunk:find("\n", pos, true)
      if not s then
        break
      end
      nl_count = nl_count + 1
      last_nl_end = e
      pos = e + 1
    end
    if nl_count > 0 then
      line = line + nl_count
      col = #chunk - last_nl_end
    else
      col = col + #chunk
    end
  end

  local function skip_ws()
    while i <= n do
      local c = str:sub(i, i)
      if c == " " or c == "\t" or c == "\n" or c == "\r" then
        i = i + 1
      else
        break
      end
    end
  end

  while i <= n do
    skip_ws()
    if i > n then
      break
    end
    local c = str:sub(i, i)

    if c == '"' then
      local start = i
      i = i + 1
      while i <= n do
        local ch = str:sub(i, i)
        if ch == "\\" then
          i = i + 2
        elseif ch == '"' then
          i = i + 1
          break
        else
          i = i + 1
        end
      end
      local raw = str:sub(start, i - 1)
      skip_ws()
      emit(raw, (str:sub(i, i) == ":") and "key" or "string")
    elseif c == "{" or c == "[" then
      local closing = (c == "{") and "}" or "]"
      i = i + 1
      skip_ws()
      if str:sub(i, i) == closing then
        emit(c .. closing, nil)
        i = i + 1
      else
        indent = indent + 1
        emit(c .. "\n" .. string.rep("  ", indent), nil)
      end
    elseif c == "}" or c == "]" then
      indent = indent - 1
      emit("\n" .. string.rep("  ", indent) .. c, nil)
      i = i + 1
    elseif c == "," then
      emit(",\n" .. string.rep("  ", indent), nil)
      i = i + 1
    elseif c == ":" then
      emit(": ", nil)
      i = i + 1
    else
      local start = i
      while i <= n and not str:sub(i, i):match('[%s,%]}:"]') do
        i = i + 1
      end
      local t = str:sub(start, i - 1)
      local token_type = "number"
      if t == "true" or t == "false" then
        token_type = "boolean"
      elseif t == "null" then
        token_type = "null"
      end
      emit(t, token_type)
    end
  end

  return table.concat(out)
end

return M
