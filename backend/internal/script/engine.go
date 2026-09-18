// Package script runs a request's pre-/post-request Lua scripts, giving
// them a small object model (client.global, response) plus file-read and
// command-exec capabilities via github.com/vadv/gopher-lua-libs.
package script

import (
	"fmt"
	"io"
	"os"
	"strings"

	lua "github.com/yuin/gopher-lua"

	luacmd "github.com/vadv/gopher-lua-libs/cmd"
	luafilepath "github.com/vadv/gopher-lua-libs/filepath"
	luaioutil "github.com/vadv/gopher-lua-libs/ioutil"
	luajson "github.com/vadv/gopher-lua-libs/json"

	"github.com/cristianradulescu/httpfly/internal/client"
	"github.com/cristianradulescu/httpfly/internal/state"
)

// GlobalState is one directory+environment's persisted "client.global"
// variables (see internal/state). Set writes through to disk immediately,
// so even a single ad-hoc request durably updates it for later runs.
type GlobalState struct {
	dir     string
	envName string
	vars    map[string]string
}

// NewGlobalState wraps an already-loaded variable set (internal/state.Load)
// for use by scripts run against the same directory and environment.
func NewGlobalState(dir, envName string, vars map[string]string) *GlobalState {
	if vars == nil {
		vars = make(map[string]string)
	}
	return &GlobalState{dir: dir, envName: envName, vars: vars}
}

// Vars returns the current variables, e.g. to merge into interpolation for
// requests after any scripts that ran before them.
func (g *GlobalState) Vars() map[string]string {
	return g.vars
}

// get returns the persisted value for name and whether one exists -- a
// script sees nil for an unset variable (Lua's own convention for
// "absent"), not "", so it can tell "never set" apart from "set to empty".
func (g *GlobalState) get(name string) (string, bool) {
	v, ok := g.vars[name]
	return v, ok
}

func (g *GlobalState) set(name, value string) error {
	g.vars[name] = value
	return state.Save(g.dir, g.envName, g.vars)
}

// RunPreScript executes a request's "< {% ... %}" script before it's sent.
// It can read/write persisted globals via client.global, and use the
// preloaded cmd/ioutil/filepath/json modules for file and command work.
func RunPreScript(source string, global *GlobalState) error {
	L := newState()
	defer L.Close()
	registerClient(L, global)
	if err := L.DoString(source); err != nil {
		return fmt.Errorf("pre-request script: %w", err)
	}
	return nil
}

// RunPostScript executes a request's "> {% ... %}" script after its
// response is received. It additionally has access to response.
func RunPostScript(source string, result client.Result, global *GlobalState) error {
	L := newState()
	defer L.Close()
	registerClient(L, global)
	registerResponse(L, result)
	if err := L.DoString(source); err != nil {
		return fmt.Errorf("post-request script: %w", err)
	}
	return nil
}

// PrintOutput is where a script's print(...) calls go. It defaults to
// stderr rather than Lua's usual stdout so that debugging output from a
// script can never end up mixed into "run -json"'s stdout, which must stay
// a single clean JSON document for tools (e.g. the neovim plugin) reading it.
var PrintOutput io.Writer = os.Stderr

// newState returns a fresh Lua VM with the standard library plus
// cmd/ioutil/filepath/json preloaded under their own names (so a script
// calls e.g. ioutil.readfile(...), cmd.execute(...), json.decode(...)
// directly, per those packages' own documented API), and print redirected
// to PrintOutput.
func newState() *lua.LState {
	L := lua.NewState()
	luacmd.Preload(L)
	luaioutil.Preload(L)
	luafilepath.Preload(L)
	luajson.Preload(L)
	L.SetGlobal("print", L.NewFunction(luaPrint))
	return L
}

// luaPrint mirrors Lua's own print (tab-separated tostring of every
// argument, then a newline) but writes to PrintOutput instead of stdout.
func luaPrint(L *lua.LState) int {
	n := L.GetTop()
	parts := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		parts = append(parts, L.ToStringMeta(L.Get(i)).String())
	}
	fmt.Fprintln(PrintOutput, strings.Join(parts, "\t"))
	return 0
}

// registerClient exposes "client.global:get(name)" (nil if unset) /
// "client.global:set(name, value)".
func registerClient(L *lua.LState, global *GlobalState) {
	globalTbl := L.NewTable()
	globalTbl.RawSetString("get", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(2)
		if v, ok := global.get(name); ok {
			L.Push(lua.LString(v))
		} else {
			L.Push(lua.LNil)
		}
		return 1
	}))
	globalTbl.RawSetString("set", L.NewFunction(func(L *lua.LState) int {
		name := L.CheckString(2)
		value := L.CheckString(3)
		if err := global.set(name, value); err != nil {
			L.RaiseError("client.global:set: %v", err)
			return 0
		}
		return 0
	}))

	client := L.NewTable()
	client.RawSetString("global", globalTbl)
	L.SetGlobal("client", client)
}

// registerResponse exposes "response.status", "response.headers" (a table
// of name -> array of values), and "response.body" (raw string -- parse it
// with the preloaded json.decode(response.body) if it's JSON).
func registerResponse(L *lua.LState, result client.Result) {
	tbl := L.NewTable()
	tbl.RawSetString("status", lua.LNumber(result.StatusCode))
	tbl.RawSetString("body", lua.LString(result.Body))

	headers := L.NewTable()
	for name, values := range result.Headers {
		arr := L.NewTable()
		for _, v := range values {
			arr.Append(lua.LString(v))
		}
		headers.RawSetString(name, arr)
	}
	tbl.RawSetString("headers", headers)

	L.SetGlobal("response", tbl)
}
