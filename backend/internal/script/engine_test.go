package script

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianradulescu/httpfly/internal/client"
)

func TestRunPreScriptSetsAndGetsGlobal(t *testing.T) {
	dir := t.TempDir()
	global := NewGlobalState(dir, "", nil)

	err := RunPreScript(`client.global:set("token", "abc123")`, global)
	if err != nil {
		t.Fatalf("RunPreScript: %v", err)
	}
	if want := "abc123"; global.Vars()["token"] != want {
		t.Errorf("token = %q, want %q", global.Vars()["token"], want)
	}

	// A second script should see the value the first one persisted.
	err = RunPreScript(`
		local t = client.global:get("token")
		if t ~= "abc123" then error("expected abc123, got " .. t) end
	`, global)
	if err != nil {
		t.Fatalf("RunPreScript (read-back): %v", err)
	}
}

func TestGlobalSetPersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	global := NewGlobalState(dir, "myenv", nil)

	if err := RunPreScript(`client.global:set("x", "1")`, global); err != nil {
		t.Fatalf("RunPreScript: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".httpfly", "state.json")); err != nil {
		t.Errorf("state file not written: %v", err)
	}
}

func TestRunPostScriptReadsResponse(t *testing.T) {
	dir := t.TempDir()
	global := NewGlobalState(dir, "", nil)

	result := client.Result{
		StatusCode: 200,
		Body:       []byte(`{"uuid": "req-42"}`),
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
	}

	err := RunPostScript(`
		if response.status ~= 200 then error("bad status") end
		local json = require("json")
		local data = json.decode(response.body)
		client.global:set("request_id", data.uuid)
	`, result, global)
	if err != nil {
		t.Fatalf("RunPostScript: %v", err)
	}
	if want := "req-42"; global.Vars()["request_id"] != want {
		t.Errorf("request_id = %q, want %q", global.Vars()["request_id"], want)
	}
}

func TestRunPostScriptReadsResponseHeaders(t *testing.T) {
	dir := t.TempDir()
	global := NewGlobalState(dir, "", nil)

	result := client.Result{
		StatusCode: 200,
		Headers:    http.Header{"X-Request-Id": []string{"abc"}},
	}

	err := RunPostScript(`
		local id = response.headers["X-Request-Id"][1]
		client.global:set("seen_id", id)
	`, result, global)
	if err != nil {
		t.Fatalf("RunPostScript: %v", err)
	}
	if want := "abc"; global.Vars()["seen_id"] != want {
		t.Errorf("seen_id = %q, want %q", global.Vars()["seen_id"], want)
	}
}

func TestScriptCanReadAFile(t *testing.T) {
	dir := t.TempDir()
	fileToRead := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(fileToRead, []byte("s3cr3t"), 0o600); err != nil {
		t.Fatal(err)
	}
	global := NewGlobalState(dir, "", nil)

	err := RunPreScript(`
		local ioutil = require("ioutil")
		local content = ioutil.read_file("`+fileToRead+`")
		client.global:set("secret", content)
	`, global)
	if err != nil {
		t.Fatalf("RunPreScript: %v", err)
	}
	if want := "s3cr3t"; global.Vars()["secret"] != want {
		t.Errorf("secret = %q, want %q", global.Vars()["secret"], want)
	}
}

func TestScriptCanExecAndCaptureOutput(t *testing.T) {
	dir := t.TempDir()
	global := NewGlobalState(dir, "", nil)

	err := RunPreScript(`
		local cmd = require("cmd")
		local result = cmd.exec("echo -n hello-from-exec")
		client.global:set("out", result.stdout)
	`, global)
	if err != nil {
		t.Fatalf("RunPreScript: %v", err)
	}
	if want := "hello-from-exec"; global.Vars()["out"] != want {
		t.Errorf("out = %q, want %q", global.Vars()["out"], want)
	}
}

func TestRunPreScriptSyntaxErrorReturnsError(t *testing.T) {
	global := NewGlobalState(t.TempDir(), "", nil)
	if err := RunPreScript(`this is not valid lua (((`, global); err == nil {
		t.Fatal("expected an error for invalid lua")
	}
}

func TestRunPreScriptRuntimeErrorReturnsError(t *testing.T) {
	global := NewGlobalState(t.TempDir(), "", nil)
	if err := RunPreScript(`error("boom")`, global); err == nil {
		t.Fatal("expected an error from a script that calls error()")
	}
}
