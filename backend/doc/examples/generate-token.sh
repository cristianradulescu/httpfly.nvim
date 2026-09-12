#!/bin/sh
# Stand-in for a complex auth step that's awkward to reimplement inline in
# a script block -- a company-internal CLI that talks to a vault/hardware
# token, a multi-request OAuth device-code exchange, a script that
# decrypts a credential file, etc. All that matters to the .http file
# calling this is: it prints a token to stdout.
echo "shell-token-$(date +%s)"
