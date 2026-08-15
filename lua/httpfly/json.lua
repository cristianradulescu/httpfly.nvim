local M = {}

-- Re-indents a JSON string with 2-space indentation. Operates as a bracket
-- scanner rather than a full parse+re-encode, so object key order and
-- numeric/string formatting are preserved exactly (vim.json.decode would
-- lose key order since Lua tables are unordered).
function M.pretty(str)
  local out = {}
  local indent = 0
  local i = 1
  local n = #str

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
      table.insert(out, str:sub(start, i - 1))
    elseif c == "{" or c == "[" then
      local closing = (c == "{") and "}" or "]"
      i = i + 1
      skip_ws()
      if str:sub(i, i) == closing then
        table.insert(out, c .. closing)
        i = i + 1
      else
        indent = indent + 1
        table.insert(out, c .. "\n" .. string.rep("  ", indent))
      end
    elseif c == "}" or c == "]" then
      indent = indent - 1
      table.insert(out, "\n" .. string.rep("  ", indent) .. c)
      i = i + 1
    elseif c == "," then
      table.insert(out, ",\n" .. string.rep("  ", indent))
      i = i + 1
    elseif c == ":" then
      table.insert(out, ": ")
      i = i + 1
    else
      local start = i
      while i <= n and not str:sub(i, i):match('[%s,%]}:"]') do
        i = i + 1
      end
      table.insert(out, str:sub(start, i - 1))
    end
  end

  return table.concat(out)
end

return M
