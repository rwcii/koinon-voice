package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rwcii/koinon-voice/internal/keychain"
	"github.com/rwcii/koinon-voice/internal/signed"
)

// The key's store and clock; tests replace them.
var (
	keyStore signed.Store = keychain.Keychain{}
	keyNow                = time.Now
)

// runKey creates, shows or enrolls this install's maintainer key (ADR 004; rwcii/koinon#693).
// Only public material is printed.
func runKey(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return badOptions(stderr)
	}
	ctx := context.Background()
	switch args[0] {
	case "create":
		fs := flags("key create")
		user := fs.String("user", "", "the user name")
		if fs.Parse(args[1:]) != nil || fs.NArg() != 0 || *user == "" {
			return badOptions(stderr)
		}
		k, err := signed.Create(ctx, keyStore, *user)
		if errors.Is(err, keychain.ErrExists) {
			return commandError(stderr, "this install already has a maintainer key; koinon-voice key show prints it")
		}
		if err != nil {
			return commandError(stderr, err.Error())
		}
		printKey(stdout, k)
		return 0
	case "show":
		fs := flags("key show")
		if fs.Parse(args[1:]) != nil || fs.NArg() != 0 {
			return badOptions(stderr)
		}
		k, err := signed.Load(ctx, keyStore)
		if err != nil {
			return commandError(stderr, err.Error())
		}
		printKey(stdout, k)
		return 0
	case "enroll":
		fs := flags("key enroll")
		target := fs.String("target", "", "the target's pin, from koinon link status on the target")
		client := fs.String("client", "koinon-voice", "the client label")
		out := fs.String("out", "", "write the statement to a new file")
		if fs.Parse(args[1:]) != nil || fs.NArg() != 0 || *target == "" {
			return badOptions(stderr)
		}
		k, err := signed.Load(ctx, keyStore)
		if err != nil {
			return commandError(stderr, err.Error())
		}
		e, err := signed.Enrollment(k.Public(), *target, k.User, *client, keyNow().Unix())
		if err != nil {
			return commandError(stderr, err.Error())
		}
		e, err = signed.Sign(k.Private, e)
		if err != nil {
			return commandError(stderr, err.Error())
		}
		b, _ := json.Marshal(e)
		b = append(b, '\n')
		if *out == "" {
			stdout.Write(b)
			return 0
		}
		f, err := os.OpenFile(*out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return commandError(stderr, fmt.Sprintf("the statement was not written: %s exists or cannot be created", *out))
		}
		if _, err := f.Write(b); err != nil {
			f.Close()
			return commandError(stderr, "the statement was not written: "+err.Error())
		}
		if err := f.Close(); err != nil {
			return commandError(stderr, "the statement was not written: "+err.Error())
		}
		fmt.Fprintf(stdout, "Wrote %s. Within 10 minutes, on the target: koinon maintainer key add --statement %s\n", *out, *out)
		return 0
	}
	return badOptions(stderr)
}

// printKey prints the key's public parts as JSON.
func printKey(w io.Writer, k signed.Key) {
	printJSON(w, struct {
		KeyID     string `json:"key_id"`
		PublicKey string `json:"public_key"`
		User      string `json:"user"`
	}{k.ID(), base64.StdEncoding.EncodeToString(k.Public()), k.User}, "  ")
}
