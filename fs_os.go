//go:build !fs_embed

package main

import "os"

var dirFS = os.DirFS(".")
