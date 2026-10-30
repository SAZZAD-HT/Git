// Package config loads, defaults and validates the server configuration (anvil.toml).
//
// Order of precedence, lowest to highest: built-in defaults (Default), the TOML file,
// then ANVIL_<SECTION>_<KEY> environment variables (ADR-010, NFR-OPS-02). Every key is
// documented in docs/reference/11-configuration-and-operations.md §3; a test fails if a
// field is added here without a matching entry there.
//
// The package only reads files and environment variables. It never creates directories
// or opens the database; `anvil init` and `anvil serve` do that with the result.
package config
